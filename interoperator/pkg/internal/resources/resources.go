package resources

import (
	"context"
	"fmt"
	"os"

	osbv1alpha1 "github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/apis/osb/v1alpha1"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/constants"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/errors"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/internal/dynamic"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/internal/properties"
	rendererFactory "github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/internal/renderer/factory"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/internal/services"

	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kubernetes "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("resources.internal")

// ResourceManager defines the interface implemented by resources
//go:generate mockgen -source resources.go -destination ./mock_resources/mock_resources.go
type ResourceManager interface {
	ComputeExpectedResources(client kubernetes.Client, instanceID, bindingID, serviceID, planID, action, namespace string) ([]*unstructured.Unstructured, error)
	SetOwnerReference(owner metav1.Object, ownerGVK schema.GroupVersionKind, resources []*unstructured.Unstructured) error
	ReconcileResources(sourceClient kubernetes.Client, targetClient kubernetes.Client, expectedResources []*unstructured.Unstructured, lastResources []osbv1alpha1.Source) ([]osbv1alpha1.Source, error)
	ComputeStatus(sourceClient kubernetes.Client, targetClient kubernetes.Client, instanceID, bindingID, serviceID, planID, action, namespace string) (*properties.Status, error)
	DeleteSubResources(client kubernetes.Client, subResources []osbv1alpha1.Source) ([]osbv1alpha1.Source, error)
}

type resourceManager struct {
}

// New creates a new ResourceManager object.
func New() ResourceManager {
	return resourceManager{}
}

func (r resourceManager) fetchResources(client kubernetes.Client, instanceID, bindingID, serviceID, planID, namespace string) (*osbv1alpha1.SFServiceInstance, *osbv1alpha1.SFServiceBinding, *osbv1alpha1.SFService, *osbv1alpha1.SFPlan, error) {
	var instance *osbv1alpha1.SFServiceInstance
	var binding *osbv1alpha1.SFServiceBinding
	var service *osbv1alpha1.SFService
	var plan *osbv1alpha1.SFPlan
	var err error

	if instanceID != "" {
		instance = &osbv1alpha1.SFServiceInstance{}
		err = client.Get(context.TODO(), types.NamespacedName{
			Name:      instanceID,
			Namespace: namespace,
		}, instance)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				return nil, nil, nil, nil, errors.NewSFServiceInstanceNotFound(instanceID, err)
			}
			log.Error(err, "failed to get service instance", "instanceID", instanceID)
			return nil, nil, nil, nil, err
		}
	}

	if serviceID != "" && planID != "" {
		serviceNamespace := os.Getenv(constants.NamespaceEnvKey)
		if serviceNamespace == "" {
			serviceNamespace = constants.DefaultServiceFabrikNamespace
		}
		service, plan, err = services.FindServiceInfo(client, serviceID, planID, serviceNamespace)
		if err != nil {
			log.Error(err, "failed finding service and plan info", "serviceID", serviceID, "planID", planID)
			return nil, nil, nil, nil, err
		}
	}

	if bindingID != "" {
		binding = &osbv1alpha1.SFServiceBinding{}
		err := client.Get(context.TODO(), types.NamespacedName{
			Name:      bindingID,
			Namespace: namespace,
		}, binding)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				return nil, nil, nil, nil, errors.NewSFServiceBindingNotFound(bindingID, err)
			}
			log.Error(err, "failed getting service binding", "bindingID", bindingID)
			return nil, nil, nil, nil, err
		}
	}

	return instance, binding, service, plan, nil
}

// ComputeExpectedResources computes expected resources
func (r resourceManager) ComputeExpectedResources(client kubernetes.Client, instanceID, bindingID, serviceID, planID, action, namespace string) ([]*unstructured.Unstructured, error) {
	instance, binding, service, plan, err := r.fetchResources(client, instanceID, bindingID, serviceID, planID, namespace)
	if err != nil {
		log.Error(err, "failed fetching resources to compute expected resources")
		return nil, err
	}

	if plan == nil {
		return nil, errors.NewSFPlanNotFound(planID, nil)
	}

	if service == nil {
		return nil, errors.NewSFServiceNotFound(serviceID, nil)
	}

	name := types.NamespacedName{
		Namespace: namespace,
		Name:      instance.GetName(),
	}

	switch action {
	case osbv1alpha1.BindAction:
		name.Name = binding.GetName()
	}

	template, err := plan.GetTemplate(action)
	if err != nil {
		log.Error(err, "plan does not have template", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action)
		return nil, err
	}

	renderer, err := rendererFactory.GetRenderer(template.Type, nil)
	if err != nil {
		log.Error(err, "failed to get renderer", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action, "type", template.Type)
		return nil, err
	}

	input, err := rendererFactory.GetRendererInput(template, service, plan, instance, binding, name)
	if err != nil {
		log.Error(err, "failed creating renderer input", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action, "type", template.Type)
		return nil, err
	}

	output, err := renderer.Render(input)
	if err != nil {
		log.Error(err, "failed rendering resource", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action)
		if errors.RendererError(err) {
			rendererError := err.(*errors.InteroperatorError)
			log.Error(rendererError.Err, "failed rendering resource")
		}
		return nil, err
	}

	files, err := output.ListFiles()
	if err != nil {
		log.Error(err, "failed listing rendered resource files", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action)
		return nil, err
	}

	resources := make([]*unstructured.Unstructured, 0, len(files))
	for _, file := range files {
		subResourcesString, err := output.FileContent(file)
		if err != nil {
			log.Error(err, "failed to get rendered file content", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action, "file", file)
			return nil, err
		}

		subresources, err := dynamic.StringToUnstructured(subResourcesString)
		if err != nil {
			log.Error(err, "failed converting file content to unstructured", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action, "file", file)
			return nil, err
		}

		for _, obj := range subresources {
			obj.SetNamespace(namespace)
			resources = append(resources, obj)
		}
	}
	return resources, nil
}

// SetOwnerReference updates the owner reference for all the resources
// For multi cluster support we cannot use k8s OwnerReference as
// owner object might be in a different cluster. So use annotations.
// Not using labels as it was restrictions of field values.
func (r resourceManager) SetOwnerReference(owner metav1.Object, ownerGVK schema.GroupVersionKind, resources []*unstructured.Unstructured) error {
	for _, obj := range resources {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[constants.OwnerNameKey] = owner.GetName()
		annotations[constants.OwnerNamespaceKey] = owner.GetNamespace()
		annotations[constants.OwnerKindKey] = ownerGVK.Kind
		annotations[constants.OwnerAPIVersionKey] = ownerGVK.GroupVersion().String()
		obj.SetAnnotations(annotations)
	}
	return nil
}

// ReconcileResources setups all resources according to expectation
func (r resourceManager) ReconcileResources(sourceClient kubernetes.Client, targetClient kubernetes.Client, expectedResources []*unstructured.Unstructured, lastResources []osbv1alpha1.Source) ([]osbv1alpha1.Source, error) {
	foundResources := make([]*unstructured.Unstructured, 0, len(expectedResources))
	for _, expectedResource := range expectedResources {
		foundResource := &unstructured.Unstructured{}

		kind := expectedResource.GetKind()
		apiVersion := expectedResource.GetAPIVersion()
		foundResource.SetKind(kind)
		foundResource.SetAPIVersion(apiVersion)
		namespacedName := types.NamespacedName{
			Name:      expectedResource.GetName(),
			Namespace: expectedResource.GetNamespace(),
		}
		foundResource.SetName(namespacedName.Name)
		foundResource.SetNamespace(namespacedName.Namespace)

		err := targetClient.Get(context.TODO(), namespacedName, foundResource)
		if err != nil && apiErrors.IsNotFound(err) {
			log.Info("reconcile - creating resource", "kind", kind, "namespacedName", namespacedName)
			err = targetClient.Create(context.TODO(), expectedResource)
			if err != nil {
				log.Error(err, "reconcile - failed to create resource", "kind", kind, "namespacedName", namespacedName)
				return nil, err
			}
			foundResources = append(foundResources, foundResource)
			continue
		} else if err != nil {
			log.Error(err, "reconcile - failed fetching resource", "kind", kind, "namespacedName", namespacedName)
			return nil, err
		}

		toBeUpdated := false
		updatedResource, toBeUpdated := dynamic.DeepUpdate(foundResource.Object, expectedResource.Object)
		if toBeUpdated {
			log.Info("reconcile - updating resource", "kind", kind, "namespacedName", namespacedName)
			foundResource.Object = updatedResource.(map[string]interface{})
			err = targetClient.Update(context.TODO(), foundResource)
			if err != nil {
				log.Error(err, "reconcile- failed to update resource", "kind", kind, "namespacedName", namespacedName)
				return nil, err
			}
		} else {
			log.Info("reconcile - resource already up todate", "kind", kind, "namespacedName", namespacedName)
		}
		foundResources = append(foundResources, foundResource)
	}

	for _, lastResource := range lastResources {
		oldResource := &unstructured.Unstructured{}
		oldResource.SetKind(lastResource.Kind)
		oldResource.SetAPIVersion(lastResource.APIVersion)
		oldResource.SetName(lastResource.Name)
		oldResource.SetNamespace(lastResource.Namespace)
		if ok := r.findUnstructuredObject(foundResources, oldResource); !ok {
			err := targetClient.Delete(context.TODO(), oldResource)
			if err != nil {
				// Not failing here. Add the outdated resource to foundResource
				// Delete will be retried on next reconcile
				log.Error(err, "reconcile - failed to delete outdated resource", "resource", lastResource)
				foundResources = append(foundResources, oldResource)
				continue
			}
			log.Info("reconcile - delete triggered for outdated resource", "resource", lastResource)
		}
	}
	resourceRefs := []osbv1alpha1.Source{}
	for _, object := range foundResources {
		resourceRefs = append(resourceRefs, r.unstructuredToSource(object))
	}
	return resourceRefs, nil
}

func (r resourceManager) unstructuredToSource(object *unstructured.Unstructured) osbv1alpha1.Source {
	resourceRef := osbv1alpha1.Source{}
	resourceRef.Kind = object.GetKind()
	resourceRef.APIVersion = object.GetAPIVersion()
	resourceRef.Name = object.GetName()
	resourceRef.Namespace = object.GetNamespace()
	return resourceRef
}

func (r resourceManager) findUnstructuredObject(list []*unstructured.Unstructured, item *unstructured.Unstructured) bool {
	for _, object := range list {
		if object.GetKind() == item.GetKind() && object.GetAPIVersion() == item.GetAPIVersion() && object.GetName() == item.GetName() && object.GetNamespace() == item.GetNamespace() {
			return true
		}
	}
	return false
}

// ComputeStatus computes status template
func (r resourceManager) ComputeStatus(sourceClient kubernetes.Client, targetClient kubernetes.Client, instanceID, bindingID, serviceID, planID, action, namespace string) (*properties.Status, error) {
	instance, binding, service, plan, err := r.fetchResources(sourceClient, instanceID, bindingID, serviceID, planID, namespace)
	if err != nil {
		log.Error(err, "failed fetching resources to compute status")
		return nil, err
	}

	if plan == nil {
		return nil, errors.NewSFPlanNotFound(planID, nil)
	}

	if service == nil {
		return nil, errors.NewSFServiceNotFound(serviceID, nil)
	}

	name := types.NamespacedName{
		Namespace: namespace,
		Name:      instance.GetName(),
	}

	switch action {
	case osbv1alpha1.BindAction:
		name.Name = binding.GetName()
	}

	template, err := plan.GetTemplate(osbv1alpha1.SourcesAction)
	if err != nil {
		log.Error(err, "plan does not have sources template", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action)
		return nil, err
	}

	renderer, err := rendererFactory.GetRenderer(template.Type, nil)
	if err != nil {
		log.Error(err, "failed to get sources renderer", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action, "type", template.Type)
		return nil, err
	}

	input, err := rendererFactory.GetRendererInput(template, service, plan, instance, binding, name)
	if err != nil {
		log.Error(err, "failed creating renderer input for sources", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action, "type", template.Type)
		return nil, err
	}

	output, err := renderer.Render(input)
	if err != nil {
		log.Error(err, "failed rendering sources", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action)
		if errors.RendererError(err) {
			rendererError := err.(*errors.InteroperatorError)
			log.Error(rendererError.Err, "failed rendering sources")
		}
		return nil, err
	}

	files, err := output.ListFiles()
	if err != nil {
		log.Error(err, "failed listing rendered sources files", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action)
		return nil, err
	}

	if len(files) == 0 {
		log.Error(err, "sources template did not genarate any file", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action)
		return nil, err
	}

	sourcesFileName := files[0]
	for _, file := range files {
		if file == "sources.yaml" {
			sourcesFileName = file
			break
		}
	}

	sourcesString, err := output.FileContent(sourcesFileName)
	if err != nil {
		log.Error(err, "failed to get sources file content", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action, "file", sourcesFileName)
		return nil, err
	}

	sources, err := properties.ParseSources(sourcesString)
	if err != nil {
		log.Error(err, "failed parsing file content of sources", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action, "file", sourcesFileName)
		return nil, err
	}

	sourceObjects := make(map[string]*unstructured.Unstructured)
	for key, val := range sources {
		if val.Name != "" {
			obj := &unstructured.Unstructured{}
			obj.SetKind(val.Kind)
			obj.SetAPIVersion(val.APIVersion)
			namespacedName := types.NamespacedName{
				Name:      val.Name,
				Namespace: name.Namespace,
			}
			err := targetClient.Get(context.TODO(), namespacedName, obj)
			if err != nil {
				// Not failing here as the resource might not exist
				log.Error(err, "failed to fetch resource listed in sources", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action, "resource", val)
				continue
			}
			sourceObjects[key] = obj
		}
	}

	template, err = plan.GetTemplate(osbv1alpha1.StatusAction)
	if err != nil {
		log.Error(err, "plan does not have status template", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action)
		return nil, err
	}

	renderer, err = rendererFactory.GetRenderer(template.Type, nil)
	if err != nil {
		log.Error(err, "failed to get status renderer", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action, "type", template.Type)
		return nil, err
	}

	input, err = rendererFactory.GetStatusRendererInput(template, name, sourceObjects)
	if err != nil {
		log.Error(err, "failed creating renderer input for status", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action, "type", template.Type)
		return nil, err
	}

	output, err = renderer.Render(input)
	if err != nil {
		log.Error(err, "failed rendering status", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action)
		if errors.RendererError(err) {
			rendererError := err.(*errors.InteroperatorError)
			log.Error(rendererError.Err, "failed rendering status")
		}
		return nil, err
	}

	files, err = output.ListFiles()
	if err != nil {
		log.Error(err, "failed listing rendered status files", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action)
		return nil, err
	}

	if len(files) == 0 {
		log.Error(err, "status template did not genarate any file", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action)
		return nil, err
	}

	statusFileName := files[0]
	for _, file := range files {
		if file == "status.yaml" {
			statusFileName = file
			break
		}
	}

	statusString, err := output.FileContent(statusFileName)
	if err != nil {
		log.Error(err, "failed to get status file content", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action, "file", sourcesFileName)
		return nil, err
	}

	status, err := properties.ParseStatus(statusString)
	if err != nil {
		log.Error(err, "failed parsing file content of status", "serviceID", serviceID, "planID", planID, "instanceID", instanceID, "bindingID", bindingID, "action", action, "file", sourcesFileName)
		return nil, err
	}

	return status, nil
}

// DeleteSubResources setups all resources according to expectation
func (r resourceManager) DeleteSubResources(client kubernetes.Client, subResources []osbv1alpha1.Source) ([]osbv1alpha1.Source, error) {
	//
	// delete the external dependency here
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple types for same object.

	var remainingResource []osbv1alpha1.Source
	var lastError error

	for _, subResource := range subResources {
		resource := &unstructured.Unstructured{}
		resource.SetKind(subResource.Kind)
		resource.SetAPIVersion(subResource.APIVersion)
		resource.SetName(subResource.Name)
		resource.SetNamespace(subResource.Namespace)
		err := r.deleteSubResource(client, resource)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				log.Info("deleted completed for subResource", "subResource", subResource)
				continue
			}
			log.Error(err, "failed to delete subResource", "subResource", subResource)
			remainingResource = append(remainingResource, subResource)
			lastError = err
			continue
		}
		log.Info("deleted triggered for subResource", "subResource", subResource)
		remainingResource = append(remainingResource, subResource)
	}
	return remainingResource, lastError
}

func (r resourceManager) deleteSubResource(client kubernetes.Client, resource *unstructured.Unstructured) error {
	// Special delete handling for sf operators for delete
	var specialDelete = [...]string{"deployment.servicefabrik.io/v1alpha1", "bind.servicefabrik.io/v1alpha1"}
	apiVersion := resource.GetAPIVersion()

	for _, val := range specialDelete {
		if apiVersion == val {
			namespacedName := types.NamespacedName{
				Name:      resource.GetName(),
				Namespace: resource.GetNamespace(),
			}
			err := client.Get(context.TODO(), namespacedName, resource)
			if err != nil {
				return err
			}
			content := resource.UnstructuredContent()
			statusInt, ok := content["status"]
			var status map[string]interface{}
			if ok {
				status, ok = statusInt.(map[string]interface{})
				if !ok {
					return fmt.Errorf("status field not map for resource %v", resource)
				}
			} else {
				status = make(map[string]interface{})
			}

			status["state"] = "delete"
			content["status"] = status
			resource.SetUnstructuredContent(content)
			err = client.Update(context.TODO(), resource)
			if err != nil {
				return err
			}
			return nil
		}
	}
	return client.Delete(context.TODO(), resource)
}

// MapReconcileByAnnotations maps resources to its owner using the Annotations
func MapReconcileByAnnotations(a handler.MapObject, ownerGvk schema.GroupVersionKind) []reconcile.Request {
	annotations := a.Meta.GetAnnotations()
	ownerKind, ok := annotations[constants.OwnerKindKey]
	if !ok {
		return nil
	}
	ownerAPIVersion, ok := annotations[constants.OwnerAPIVersionKey]
	if !ok {
		return nil
	}

	if ownerKind != ownerGvk.Kind || ownerAPIVersion != ownerGvk.GroupVersion().String() {
		return nil
	}

	ownerName, ok := annotations[constants.OwnerNameKey]
	if !ok {
		return nil
	}
	ownerNamespace, ok := annotations[constants.OwnerNamespaceKey]
	if !ok {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{
		Name:      ownerName,
		Namespace: ownerNamespace,
	}}}
}

// MapReconcileByControllerReference maps resources to its owner using
// owner reference. Finds only the owner which is set a controller
func MapReconcileByControllerReference(a handler.MapObject, ownerGvk schema.GroupVersionKind) []reconcile.Request {
	ownerRef := metav1.GetControllerOf(a.Meta)
	if ownerRef == nil {
		return nil
	}

	// Compare the OwnerReference Group and Kind against the OwnerType Group and Kind specified by the user.
	// If the two match, create a Request for the objected referred to by
	// the OwnerReference. Use the Name from the OwnerReference and the Namespace from the
	// object in the event.
	if ownerRef.Kind != ownerGvk.Kind || ownerRef.APIVersion != ownerGvk.GroupVersion().String() {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{
		Name:      ownerRef.Name,
		Namespace: a.Meta.GetNamespace(),
	}}}
}
