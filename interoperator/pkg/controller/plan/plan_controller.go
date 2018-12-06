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

package plan

import (
	"context"
	"fmt"
	"log"

	osbv1alpha1 "github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/apis/osb/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kubernetes "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new Plan Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePlan{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("plan-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Plan
	err = c.Watch(&source.Kind{Type: &osbv1alpha1.Plan{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

var _ reconcile.Reconciler = &ReconcilePlan{}

// ReconcilePlan reconciles a Plan object
type ReconcilePlan struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Plan object and makes changes based on the state read
// and what is in the Plan.Spec
// +kubebuilder:rbac:groups=osb.servicefabrik.io,resources=plans,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcilePlan) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Plan instance
	instance := &osbv1alpha1.Plan{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	labels := instance.GetLabels()
	updateRequired := false
	if serviceID, ok := labels["serviceId"]; !ok || instance.Spec.ServiceID != serviceID {
		labels["serviceId"] = instance.Spec.ServiceID
		updateRequired = true
	}
	if planID, ok := labels["planId"]; !ok || instance.Spec.ID != planID {
		labels["planId"] = instance.Spec.ID
		updateRequired = true
	}

	serviceID := instance.Spec.ServiceID
	services := &osbv1alpha1.ServiceList{}
	searchLabels := make(map[string]string)
	searchLabels["serviceId"] = serviceID
	options := kubernetes.MatchingLabels(searchLabels)
	options.Namespace = request.Namespace

	err = r.List(context.TODO(), options, services)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("unable to find service with id %s", serviceID)
		}
		return reconcile.Result{}, err
	}
	var service *osbv1alpha1.Service
	for _, obj := range services.Items {
		if obj.Spec.ID == serviceID {
			service = &obj
		}
	}
	if service == nil {
		return reconcile.Result{}, fmt.Errorf("unable to find service with id %s", serviceID)
	}

	ownerRefs := instance.GetOwnerReferences()
	existingRefs := make([]metav1.OwnerReference, len(ownerRefs))
	for i := range ownerRefs {
		existingRefs[i] = *ownerRefs[i].DeepCopy()
	}

	err = controllerutil.SetControllerReference(service, instance, r.scheme)
	if err != nil {
		log.Printf("error setting owner reference for plan %s. %v\n", instance.Spec.ID, err)
		return reconcile.Result{}, err
	}

	if !updateRequired {
		ownerRefs = instance.GetOwnerReferences()
		if len(ownerRefs) != len(existingRefs) {
			updateRequired = true
		} else {
			for i := range ownerRefs {
				if !referSameObject(ownerRefs[i], existingRefs[i]) {
					updateRequired = true
					break
				}
			}
		}
	}

	if updateRequired {
		instance.SetLabels(labels)
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		log.Printf("Plan %s labels updated\n", instance.GetName())
	}
	return reconcile.Result{}, nil
}

// Returns true if a and b point to the same object
func referSameObject(a, b metav1.OwnerReference) bool {
	aGV, err := schema.ParseGroupVersion(a.APIVersion)
	if err != nil {
		return false
	}

	bGV, err := schema.ParseGroupVersion(b.APIVersion)
	if err != nil {
		return false
	}

	return aGV == bGV && a.Kind == b.Kind && a.Name == b.Name
}
