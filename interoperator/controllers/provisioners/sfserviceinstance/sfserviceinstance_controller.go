/*
Copyright 2018 The Service Fabrik Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sfserviceinstance

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	osbv1alpha1 "github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/api/osb/v1alpha1"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/internal/config"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/internal/properties"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/internal/resources"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/cluster/registry"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/constants"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/errors"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/utils"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/watches"

	"github.com/go-logr/logr"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ReconcileSFServiceInstance reconciles a SFServiceInstance object
type ReconcileSFServiceInstance struct {
	client.Client
	uncachedClient  client.Client
	Log             logr.Logger
	scheme          *runtime.Scheme
	clusterRegistry registry.ClusterRegistry
	resourceManager resources.ResourceManager
	watchList       []osbv1alpha1.APIVersionKind
	cfgManager      config.Config
}

// Reconcile reads that state of the cluster for a SFServiceInstance object and makes changes based on the state read
// and what is in the SFServiceInstance.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=osb.servicefabrik.io,resources=*,verbs=*
// +kubebuilder:rbac:groups=deployment.servicefabrik.io,resources=*,verbs=*
// +kubebuilder:rbac:groups=kubernetes.sapcloud.io,resources=*,verbs=*
// +kubebuilder:rbac:groups=kubedb.com,resources=Postgres,verbs=*
// +kubebuilder:rbac:groups=,resources=configmap,verbs=*
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=*
// TODO dynamically setup rbac rules
func (r *ReconcileSFServiceInstance) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("sfserviceinstance", req.NamespacedName)

	defer r.restartOnWatchUpdate()

	// Fetch the ServiceInstance instance
	instance := &osbv1alpha1.SFServiceInstance{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			log.Info("instance deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return r.handleError(instance, ctrl.Result{}, err, "", 0)
	}

	serviceID := instance.Spec.ServiceID
	planID := instance.Spec.PlanID
	instanceID := instance.GetName()
	bindingID := ""
	state := instance.GetState()

	if state == "succeeded" || state == "failed" {
		return ctrl.Result{}, nil
	}

	// Fetch again using uncachedClient to read the state again
	err = r.uncachedClient.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			log.Info("instance deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return r.handleError(instance, ctrl.Result{}, err, state, 0)
	}
	state = instance.GetState()

	clusterID, err := instance.GetClusterID()
	if err != nil {
		if errors.SFServiceInstanceNotFound(err) || errors.ClusterIDNotSet(err) {
			log.Info("clusterID not set. Ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get clusterID")
		return r.handleError(instance, ctrl.Result{}, err, state, 0)
	}
	if clusterID != constants.OwnClusterID {
		return ctrl.Result{}, nil
	}

	if err := r.reconcileFinalizers(instance, 0); err != nil {
		return r.handleError(instance, ctrl.Result{Requeue: true}, nil, "", 0)
	}

	if state == "delete" && !instance.GetDeletionTimestamp().IsZero() {
		// The object is being deleted
		// so lets handle our external dependency
		remainingResource, err := r.resourceManager.DeleteSubResources(r, instance.Status.Resources)
		if err != nil {
			log.Error(err, "Delete sub resources failed")
			return r.handleError(instance, ctrl.Result{}, err, state, 0)
		}
		err = r.setInProgress(req.NamespacedName, state, remainingResource, 0)
		if err != nil {
			return r.handleError(instance, ctrl.Result{}, err, state, 0)
		}
	} else if state == "in_queue" || state == "update" {
		expectedResources, err := r.resourceManager.ComputeExpectedResources(r, instanceID, bindingID, serviceID, planID, osbv1alpha1.ProvisionAction, instance.GetNamespace())
		if err != nil {
			return r.handleError(instance, ctrl.Result{}, err, state, 0)
		}

		err = r.resourceManager.SetOwnerReference(instance, expectedResources, r.scheme)
		if err != nil {
			return r.handleError(instance, ctrl.Result{}, err, state, 0)
		}

		resourceRefs, err := r.resourceManager.ReconcileResources(r, expectedResources, instance.Status.Resources, false)
		if err != nil {
			log.Error(err, "ReconcileResources failed")
			return r.handleError(instance, ctrl.Result{}, err, state, 0)
		}
		err = r.setInProgress(req.NamespacedName, state, resourceRefs, 0)
		if err != nil {
			return r.handleError(instance, ctrl.Result{}, err, state, 0)
		}
	}

	err = r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		return r.handleError(instance, ctrl.Result{}, err, "", 0)
	}
	state = instance.GetState()
	labels := instance.GetLabels()
	lastOperation, ok := labels[constants.LastOperationKey]
	if !ok {
		lastOperation = "in_queue"
	}

	if state == "in progress" {
		if lastOperation == "delete" {
			if err := r.updateDeprovisionStatus(instance, 0); err != nil {
				return r.handleError(instance, ctrl.Result{}, err, lastOperation, 0)
			}
		} else if lastOperation == "in_queue" || lastOperation == "update" {
			err = r.updateStatus(instance, 0)
			if err != nil {
				return r.handleError(instance, ctrl.Result{}, err, lastOperation, 0)
			}
		}
	}
	return r.handleError(instance, ctrl.Result{}, nil, lastOperation, 0)
}

func (r *ReconcileSFServiceInstance) reconcileFinalizers(object *osbv1alpha1.SFServiceInstance, retryCount int) error {
	ctx := context.Background()

	objectID := object.GetName()
	namespace := object.GetNamespace()
	// Fetch object again before updating
	namespacedName := types.NamespacedName{
		Name:      objectID,
		Namespace: namespace,
	}
	log := r.Log.WithValues("instanceID", objectID)

	err := r.Get(ctx, namespacedName, object)
	if err != nil {
		if retryCount < constants.ErrorThreshold {
			log.Info("Retrying", "function", "reconcileFinalizers", "retryCount", retryCount+1)
			return r.reconcileFinalizers(object, retryCount+1)
		}
		log.Error(err, "failed to fetch object")
		return err
	}
	if object.GetDeletionTimestamp().IsZero() {
		if !utils.ContainsString(object.GetFinalizers(), constants.FinalizerName) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object.
			object.SetFinalizers(append(object.GetFinalizers(), constants.FinalizerName))
			if err := r.Update(ctx, object); err != nil {
				if retryCount < constants.ErrorThreshold {
					log.Info("Retrying", "function", "reconcileFinalizers", "retryCount", retryCount+1)
					return r.reconcileFinalizers(object, retryCount+1)
				}
				log.Error(err, "failed to add finalizer")
				return err
			}
			log.Info("added finalizer")
		}
	}
	return nil
}

func (r *ReconcileSFServiceInstance) setInProgress(namespacedName types.NamespacedName, state string, resources []osbv1alpha1.Source, retryCount int) error {
	ctx := context.Background()
	log := r.Log.WithValues("sfserviceinstance", namespacedName, "function", "setInProgress")

	if state == "in_queue" || state == "update" || state == "delete" {
		instance := &osbv1alpha1.SFServiceInstance{}
		err := r.Get(ctx, namespacedName, instance)
		if err != nil {
			if retryCount < constants.ErrorThreshold {
				log.Info("Retrying", "retryCount", retryCount+1, "state", state)
				return r.setInProgress(namespacedName, state, resources, retryCount+1)
			}
			log.Error(err, "Updating status to in progress failed")
			return err
		}

		labels := instance.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}

		lastOperation, ok := labels[constants.LastOperationKey]
		if !ok {
			lastOperation = "in_queue"
		}

		labels[constants.LastOperationKey] = state

		// Do not update state if another operation happend in between
		if state == instance.GetState() {
			instance.SetState("in progress")
			instance.SetLabels(labels)
		} else {
			log.Info("Error while trying to set in progress. state mismatch", "state", state,
				"currentState", instance.GetState(), "lastOperation", lastOperation)
		}
		instance.Status.Resources = resources

		newState := instance.GetState()

		err = r.Update(ctx, instance)
		if err != nil {
			if retryCount < constants.ErrorThreshold {
				log.Info("Retrying", "retryCount", retryCount+1, "state", state, "newState", newState, "lastOperation", lastOperation)
				return r.setInProgress(namespacedName, state, resources, retryCount+1)
			}
			log.Error(err, "Updating status to in progress failed", "state", state, "newState", newState, "lastOperation", lastOperation)
			return err
		}
		log.Info("Updated status to in progress", "state", state, "newState", newState, "lastOperation", lastOperation)
	}
	return nil
}

func (r *ReconcileSFServiceInstance) updateDeprovisionStatus(instance *osbv1alpha1.SFServiceInstance, retryCount int) error {
	ctx := context.Background()

	serviceID := instance.Spec.ServiceID
	planID := instance.Spec.PlanID
	instanceID := instance.GetName()
	bindingID := ""
	namespace := instance.GetNamespace()

	log := r.Log.WithValues("instanceId", instanceID)

	labels := instance.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	lastOperation, ok := labels[constants.LastOperationKey]
	if !ok {
		lastOperation = "in_queue"
	}
	state := instance.GetState()

	computedStatus, err := r.resourceManager.ComputeStatus(r, instanceID, bindingID, serviceID, planID, osbv1alpha1.ProvisionAction, namespace)
	if err != nil && !errors.NotFound(err) {
		log.Error(err, "ComputeStatus failed for deprovision", "state", state, "lastOperation", lastOperation)
		return err
	}

	if errors.NotFound(err) && computedStatus == nil {
		computedStatus = &properties.Status{}
		computedStatus.Deprovision.State = instance.GetState()
		computedStatus.Deprovision.Error = err.Error()
	}

	// Fetch object again before updating status
	namespacedName := types.NamespacedName{
		Name:      instanceID,
		Namespace: namespace,
	}
	err = r.Get(ctx, namespacedName, instance)
	if err != nil {
		log.Error(err, "Failed to get instance", "state", state, "lastOperation", lastOperation)
		return err
	}

	labels = instance.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	lastOperation, ok = labels[constants.LastOperationKey]
	if !ok {
		lastOperation = "in_queue"
	}
	state = instance.GetState()
	if state != "in progress" {
		err = errors.NewPreconditionError("updateDeprovisionStatus", "state not in progress", nil)
		log.Error(err, "state changed while processing instance", "state", state, "lastOperation", lastOperation)
		return err
	}

	updateRequired := false
	updatedStatus := instance.Status.DeepCopy()
	updatedStatus.State = computedStatus.Deprovision.State
	updatedStatus.Error = computedStatus.Deprovision.Error
	updatedStatus.Description = computedStatus.Deprovision.Response
	updatedStatus.InstanceUsable = computedStatus.Deprovision.InstanceUsable
	updatedStatus.UpdateRepeatable = computedStatus.Deprovision.UpdateRepeatable

	remainingResource := []osbv1alpha1.Source{}
	for _, subResource := range instance.Status.Resources {
		resource := &unstructured.Unstructured{}
		resource.SetKind(subResource.Kind)
		resource.SetAPIVersion(subResource.APIVersion)
		resource.SetName(subResource.Name)
		resource.SetNamespace(subResource.Namespace)
		namespacedName := types.NamespacedName{
			Name:      resource.GetName(),
			Namespace: resource.GetNamespace(),
		}
		err := r.Get(ctx, namespacedName, resource)
		if !apiErrors.IsNotFound(err) {
			remainingResource = append(remainingResource, subResource)
		}
	}
	updatedStatus.Resources = remainingResource
	if !reflect.DeepEqual(&instance.Status, updatedStatus) {
		updatedStatus.DeepCopyInto(&instance.Status)
		updateRequired = true
	}

	if instance.GetState() == "succeeded" || len(remainingResource) == 0 {
		// remove our finalizer from the list and update it.
		log.Info("Removing finalizer", "state", state, "lastOperation", lastOperation)
		instance.SetFinalizers(utils.RemoveString(instance.GetFinalizers(), constants.FinalizerName))
		instance.SetState("succeeded")
		updateRequired = true
	}

	if updateRequired {
		newState := instance.GetState()
		log.Info("Updating deprovision status from template", "state", state, "lastOperation", lastOperation, "newState", newState)
		if err := r.Update(ctx, instance); err != nil {
			if retryCount < constants.ErrorThreshold {
				log.Info("Retrying", "function", "updateDeprovisionStatus", "retryCount", retryCount+1)
				return r.updateDeprovisionStatus(instance, retryCount+1)
			}
			log.Error(err, "failed to update deprovision status", "state", state, "lastOperation", lastOperation, "newState", newState)
			return err
		}
	}
	return nil
}

func (r *ReconcileSFServiceInstance) updateStatus(instance *osbv1alpha1.SFServiceInstance, retryCount int) error {
	serviceID := instance.Spec.ServiceID
	planID := instance.Spec.PlanID
	instanceID := instance.GetName()
	bindingID := ""
	namespace := instance.GetNamespace()

	ctx := context.Background()
	log := r.Log.WithValues("instanceID", instanceID)

	computedStatus, err := r.resourceManager.ComputeStatus(r, instanceID, bindingID, serviceID, planID, osbv1alpha1.ProvisionAction, namespace)
	if err != nil {
		log.Error(err, "Compute status failed")
		return err
	}

	// Fetch object again before updating status
	namespacedName := types.NamespacedName{
		Name:      instanceID,
		Namespace: namespace,
	}
	err = r.Get(ctx, namespacedName, instance)
	if err != nil {
		log.Error(err, "failed to fetch instance")
		return err
	}
	labels := instance.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	lastOperation, ok := labels[constants.LastOperationKey]
	if !ok {
		lastOperation = "in_queue"
	}
	state := instance.GetState()

	if state != "in progress" {
		err = errors.NewPreconditionError("updateStatus", "state not in progress", nil)
		log.Error(err, "state changed while processing instance", "state", state, "lastOperation", lastOperation)
		return err
	}

	updatedStatus := instance.Status.DeepCopy()
	updatedStatus.State = computedStatus.Provision.State
	updatedStatus.Error = computedStatus.Provision.Error
	updatedStatus.Description = computedStatus.Provision.Response
	updatedStatus.DashboardURL = computedStatus.Provision.DashboardURL
	updatedStatus.InstanceUsable = computedStatus.Provision.InstanceUsable
	updatedStatus.UpdateRepeatable = computedStatus.Provision.UpdateRepeatable

	if !reflect.DeepEqual(&instance.Status, updatedStatus) {
		updatedStatus.DeepCopyInto(&instance.Status)
		newState := instance.GetState()
		log.Info("Updating provision status from template", "state", state, "lastOperation", lastOperation, "newState", newState)
		err = r.Update(ctx, instance)
		if err != nil {
			if retryCount < constants.ErrorThreshold {
				log.Info("Retrying", "function", "updateStatus", "retryCount", retryCount+1)
				return r.updateStatus(instance, retryCount+1)
			}
			log.Error(err, "failed to update status", "state", state, "lastOperation", lastOperation, "newState", newState)
			return err
		}
	}
	return nil
}

func (r *ReconcileSFServiceInstance) handleError(object *osbv1alpha1.SFServiceInstance, result ctrl.Result, inputErr error, lastOperation string, retryCount int) (ctrl.Result, error) {
	objectID := object.GetName()
	namespace := object.GetNamespace()
	// Fetch object again before updating
	namespacedName := types.NamespacedName{
		Name:      objectID,
		Namespace: namespace,
	}

	ctx := context.Background()
	log := r.Log.WithValues("objectID", objectID, "function", "handleError")

	err := r.Get(ctx, namespacedName, object)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return result, inputErr
		}
		if retryCount < constants.ErrorThreshold {
			log.Info("Retrying", "retryCount", retryCount+1, "lastOperation", lastOperation, "err", inputErr)
			return r.handleError(object, result, inputErr, lastOperation, retryCount+1)
		}
		log.Error(err, "failed to fetch object")
		return result, inputErr
	}

	labels := object.GetLabels()
	var count int64

	if labels == nil {
		labels = make(map[string]string)
	}

	countString, ok := labels[constants.ErrorCountKey]
	if !ok {
		count = 0
	} else {
		i, err := strconv.ParseInt(countString, 10, 64)
		if err != nil {
			count = 0
		} else {
			count = i
		}
	}

	if inputErr != nil {
		if statusError, ok := inputErr.(*apiErrors.StatusError); ok {
			if statusError.ErrStatus.Code == 422 {
				log.Error(inputErr, "Encountered StatusError")
				object.Status.State = "failed"
				object.Status.Error = fmt.Sprintf("StatusError encountered for %s.\n%s", objectID, inputErr.Error())
				causes := statusError.ErrStatus.Details.Causes
				if len(causes) > 0 {
					messages := make([]string, 0)
					for _, v := range causes {
						messages = append(messages, v.Message)
					}
					object.Status.Description = fmt.Sprintf("%s, Error code: 422", strings.Join(messages[:], ", "))
				} else if inputErr.Error() != "" {
					object.Status.Description = fmt.Sprintf("%s, Error code: 422", inputErr.Error())
				} else {
					object.Status.Description = "Unprocessable Entity - this is usually caused by invalid request parameters, Error code: 422"
				}
				if lastOperation != "" {
					labels[constants.LastOperationKey] = lastOperation
					object.SetLabels(labels)
				}
				err := r.Update(ctx, object)
				if err != nil {
					log.Error(err, "Failed to set state to failed", "objectID", objectID)
				}
				return result, nil
			}
		}
	}
	if inputErr == nil {
		if count == 0 {
			//No change for count
			return result, inputErr
		}
		count = 0
	} else {
		count++
	}

	if count > constants.ErrorThreshold {
		log.Error(inputErr, "Retry threshold reached. Ignoring error")
		object.Status.State = "failed"
		object.Status.Error = fmt.Sprintf("Retry threshold reached for %s.\n%s", objectID, inputErr.Error())
		if inputErr.Error() != "" {
			object.Status.Description = inputErr.Error()
		} else {
			object.Status.Description = "Service Broker Error, status code: ETIMEDOUT, error code: 10008"
		}
		if lastOperation != "" {
			labels[constants.LastOperationKey] = lastOperation
			object.SetLabels(labels)
		}
		err := r.Update(ctx, object)
		if err != nil {
			if retryCount < constants.ErrorThreshold {
				log.Info("Retrying", "retryCount", retryCount+1, "lastOperation", lastOperation, "err", inputErr)
				return r.handleError(object, result, inputErr, lastOperation, retryCount+1)
			}
			log.Error(err, "Failed to set state to failed", "objectID", objectID)
		}
		return result, nil
	}

	labels[constants.ErrorCountKey] = strconv.FormatInt(count, 10)
	object.SetLabels(labels)
	err = r.Update(ctx, object)
	if err != nil {
		if retryCount < constants.ErrorThreshold {
			log.Info("Retrying", "retryCount", retryCount+1, "lastOperation", lastOperation, "err", inputErr)
			return r.handleError(object, result, inputErr, lastOperation, retryCount+1)
		}
		log.Error(err, "Failed to update error count label", "objectID", objectID, "count", count)
	}
	log.Info("Updated error count", "retryCount", retryCount, "lastOperation", lastOperation, "err", inputErr, "count", count)
	return result, inputErr
}

// Will kill the process if watchlist has changed
func (r *ReconcileSFServiceInstance) restartOnWatchUpdate() {
	if !constants.K8SDeployment {
		return
	}
	interoperatorCfg := r.cfgManager.GetConfig()
	if !watches.CompareWatchLists(interoperatorCfg.InstanceContollerWatchList, r.watchList) {
		r.Log.Info("Instance watch list changed. Restarting interoperator")
		os.Exit(1)
	}
}

// SetupWithManager registers the SFServiceInstance Controller with manager
// and setups the watches.
func (r *ReconcileSFServiceInstance) SetupWithManager(mgr ctrl.Manager) error {
	r.scheme = mgr.GetScheme()

	if r.Log == nil {
		r.Log = ctrl.Log.WithName("provisioners").WithName("instance")
	}
	if r.clusterRegistry == nil {
		clusterRegistry, err := registry.New(mgr.GetConfig(), mgr.GetScheme(), mgr.GetRESTMapper())
		if err != nil {
			return err
		}
		r.clusterRegistry = clusterRegistry
	}

	if r.resourceManager == nil {
		r.resourceManager = resources.New()
	}

	if r.uncachedClient == nil {
		uncachedClient, err := client.New(mgr.GetConfig(), client.Options{
			Scheme: mgr.GetScheme(),
			Mapper: mgr.GetRESTMapper(),
		})
		if err != nil {
			return err
		}
		r.uncachedClient = uncachedClient
	}

	cfgManager, err := config.New(mgr.GetConfig(), mgr.GetScheme(), mgr.GetRESTMapper())
	if err != nil {
		return err
	}
	interoperatorCfg := cfgManager.GetConfig()
	r.cfgManager = cfgManager

	builder := ctrl.NewControllerManagedBy(mgr).
		Named("instance").
		WithOptions(controller.Options{
			MaxConcurrentReconciles: interoperatorCfg.InstanceWorkerCount,
		}).
		For(&osbv1alpha1.SFServiceInstance{})

	// TODO dynamically setup rbac rules and watches
	r.watchList = make([]osbv1alpha1.APIVersionKind, len(interoperatorCfg.InstanceContollerWatchList))
	copy(r.watchList, interoperatorCfg.InstanceContollerWatchList)
	subresources := make([]runtime.Object, len(r.watchList))
	for i, gvk := range r.watchList {
		object := &unstructured.Unstructured{}
		object.SetKind(gvk.GetKind())
		object.SetAPIVersion(gvk.GetAPIVersion())
		subresources[i] = object
	}

	for _, subresource := range subresources {
		builder = builder.Watches(&source.Kind{Type: subresource},
			&handler.EnqueueRequestForOwner{
				IsController: false,
				OwnerType:    &osbv1alpha1.SFServiceInstance{},
			})
	}
	builder = builder.WithEventFilter(watches.NamespaceLabelFilter())

	return builder.Complete(r)
}
