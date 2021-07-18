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
	"reflect"
	"testing"
	"time"

	osbv1alpha1 "github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/api/osb/v1alpha1"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/internal/properties"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/internal/resources/mock_resources"
	mock_clusterRegistry "github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/cluster/registry/mock_registry"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/constants"
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/errors"
	"github.com/golang/mock/gomock"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrlrun "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

const timeout = time.Second * 5

var instance = &osbv1alpha1.SFServiceInstance{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "instance-id",
		Namespace: constants.InteroperatorNamespace,
		Labels: map[string]string{
			"state": "in_queue",
		},
	},
	Spec: osbv1alpha1.SFServiceInstanceSpec{
		ServiceID:        "service-id",
		PlanID:           "plan-id",
		RawContext:       nil,
		OrganizationGUID: "organization-guid",
		SpaceGUID:        "space-guid",
		RawParameters:    nil,
		PreviousValues:   nil,
		ClusterID:        "1",
	},
	Status: osbv1alpha1.SFServiceInstanceStatus{
		State: "in_queue",
	},
}

var instanceKey = types.NamespacedName{Name: "instance-id", Namespace: constants.InteroperatorNamespace}
var expectedRequest = reconcile.Request{NamespacedName: instanceKey}

func setupInteroperatorConfig(g *gomega.GomegaWithT) {
	data := make(map[string]string)
	data["instanceWorkerCount"] = "1"
	data["bindingWorkerCount"] = "1"
	watchList := `
- apiVersion: kubedb.com/v1alpha1
  kind: Postgres
- apiVersion: kubernetes.sapcloud.io/v1alpha1
  kind: Postgresql
- apiVersion: deployment.servicefabrik.io/v1alpha1
  kind: Director`
	data["instanceContollerWatchList"] = watchList
	data["bindingContollerWatchList"] = watchList
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.ConfigMapName,
			Namespace: constants.InteroperatorNamespace,
		},
		Data: data,
	}
	g.Expect(c.Create(context.TODO(), configMap)).NotTo(gomega.HaveOccurred())
}

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var expectedResources = []*unstructured.Unstructured{nil}
	var appliedResources = []osbv1alpha1.Source{
		{},
	}
	err1 := fmt.Errorf("Some error")

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c, err = client.New(cfg, client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	mockResourceManager := mock_resources.NewMockResourceManager(ctrl)
	mockClusterRegistry := mock_clusterRegistry.NewMockClusterRegistry(ctrl)

	setupInteroperatorConfig(g)

	controller := &ReconcileSFServiceInstance{
		Client:          c,
		Log:             ctrlrun.Log.WithName("provisioners").WithName("instance"),
		resourceManager: mockResourceManager,
		clusterRegistry: mockClusterRegistry,
	}

	mockResourceManager.EXPECT().ComputeExpectedResources(gomock.Any(), "instance-id", "", "service-id", "plan-id", osbv1alpha1.ProvisionAction, constants.InteroperatorNamespace).Return(expectedResources, nil).AnyTimes()
	mockResourceManager.EXPECT().SetOwnerReference(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockClusterRegistry.EXPECT().GetClient("1").Return(controller, nil).AnyTimes()
	mockResourceManager.EXPECT().ReconcileResources(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(appliedResources, err1).Times(1)
	mockResourceManager.EXPECT().ReconcileResources(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(appliedResources, nil).AnyTimes()
	mockResourceManager.EXPECT().ComputeStatus(gomock.Any(), "instance-id", "", "service-id", "plan-id", osbv1alpha1.ProvisionAction, constants.InteroperatorNamespace).Return(&properties.Status{
		Provision: properties.InstanceStatus{
			State: "succeeded",
		},
		Deprovision: properties.InstanceStatus{
			State: "succeeded",
		},
	}, nil).AnyTimes()
	mockResourceManager.EXPECT().DeleteSubResources(gomock.Any(), gomock.Any()).Return(appliedResources, nil).AnyTimes()

	g.Expect(controller.SetupWithManager(mgr)).NotTo(gomega.HaveOccurred())
	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	// Create the ServiceInstance object and expect the Reconcile and Deployment to be created
	err = c.Create(context.TODO(), instance)
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Get the serviceInstance
	serviceInstance := &osbv1alpha1.SFServiceInstance{}
	g.Eventually(func() error {
		err := c.Get(context.TODO(), instanceKey, serviceInstance)
		if err != nil {
			return err
		}
		state := serviceInstance.GetState()
		if state != "succeeded" {
			return fmt.Errorf("state not updated")
		}
		return nil
	}, timeout).Should(gomega.Succeed())
	g.Expect(serviceInstance.Status.State).Should(gomega.Equal("succeeded"))

	// Delete the service instance
	g.Expect(c.Delete(context.TODO(), instance)).NotTo(gomega.HaveOccurred())
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		serviceInstance.SetState("delete")
		err := c.Update(context.TODO(), serviceInstance)
		if err != nil {
			// The service instance is possibly outdated, fetch it again and
			// retry the update operation.
			_ = c.Get(context.TODO(), instanceKey, serviceInstance)
			return err
		}
		return nil
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Service should disappear from api server
	g.Eventually(func() error {
		err := c.Get(context.TODO(), instanceKey, serviceInstance)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		return fmt.Errorf("not deleted")
	}, timeout).Should(gomega.Succeed())
}

func TestReconcileSFServiceInstance_handleError(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	c, err = client.New(cfg, client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	mockResourceManager := mock_resources.NewMockResourceManager(ctrl)
	mockClusterRegistry := mock_clusterRegistry.NewMockClusterRegistry(ctrl)
	stopMgr, mgrStopped := StartTestManager(mgr, g)
	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	instance := &osbv1alpha1.SFServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance-id",
			Namespace: constants.InteroperatorNamespace,
			Labels: map[string]string{
				"state":                 "in_queue",
				constants.ErrorCountKey: "10",
			},
		},
		Spec: osbv1alpha1.SFServiceInstanceSpec{
			ServiceID:        "service-id",
			PlanID:           "plan-id",
			RawContext:       nil,
			OrganizationGUID: "organization-guid",
			SpaceGUID:        "space-guid",
			RawParameters:    nil,
			PreviousValues:   nil,
		},
		Status: osbv1alpha1.SFServiceInstanceStatus{
			State: "in_queue",
		},
	}
	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	serviceInstance := &osbv1alpha1.SFServiceInstance{}
	g.Eventually(func() error {
		return c.Get(context.TODO(), instanceKey, serviceInstance)
	}, timeout).Should(gomega.Succeed())

	r := &ReconcileSFServiceInstance{
		Client:          c,
		Log:             ctrlrun.Log.WithName("provisioners").WithName("instance"),
		scheme:          mgr.GetScheme(),
		clusterRegistry: mockClusterRegistry,
		resourceManager: mockResourceManager,
	}
	type args struct {
		object        *osbv1alpha1.SFServiceInstance
		result        reconcile.Result
		inputErr      error
		lastOperation string
		retryCount    int
	}
	tests := []struct {
		name    string
		setup   func()
		args    args
		want    reconcile.Result
		wantErr bool
	}{
		{
			name: "ignore error if retry count is reached",
			args: args{
				object:        instance,
				result:        reconcile.Result{},
				inputErr:      errors.NewMarshalError("", nil),
				lastOperation: "in_queue",
				retryCount:    0,
			},
			want:    reconcile.Result{},
			wantErr: false,
		},
		{
			name: "return error if instance not found",
			setup: func() {
				g.Expect(c.Delete(context.TODO(), serviceInstance)).NotTo(gomega.HaveOccurred())
				g.Eventually(func() error {
					err := c.Get(context.TODO(), instanceKey, serviceInstance)
					if err != nil {
						return nil
					}
					return errors.NewMarshalError("", nil)
				}, timeout).Should(gomega.Succeed())
			},
			args: args{
				object:        instance,
				result:        reconcile.Result{},
				inputErr:      errors.NewMarshalError("", nil),
				lastOperation: "in_queue",
				retryCount:    0,
			},
			want:    reconcile.Result{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			got, err := r.handleError(tt.args.object, tt.args.result, tt.args.inputErr, tt.args.lastOperation, tt.args.retryCount)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReconcileSFServiceInstance.handleError() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReconcileSFServiceInstance.handleError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReconcileSFServiceInstance_handleErrorResponse(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	c, err = client.New(cfg, client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	mockResourceManager := mock_resources.NewMockResourceManager(ctrl)
	mockClusterRegistry := mock_clusterRegistry.NewMockClusterRegistry(ctrl)
	stopMgr, mgrStopped := StartTestManager(mgr, g)
	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	instance := &osbv1alpha1.SFServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance-id",
			Namespace: constants.InteroperatorNamespace,
			Labels: map[string]string{
				"state":                 "in_queue",
				constants.ErrorCountKey: "10",
			},
		},
		Spec: osbv1alpha1.SFServiceInstanceSpec{
			ServiceID:        "service-id",
			PlanID:           "plan-id",
			RawContext:       nil,
			OrganizationGUID: "organization-guid",
			SpaceGUID:        "space-guid",
			RawParameters:    nil,
			PreviousValues:   nil,
		},
		Status: osbv1alpha1.SFServiceInstanceStatus{
			State: "in_queue",
		},
	}

	instance2 := &osbv1alpha1.SFServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance-id-2",
			Namespace: constants.InteroperatorNamespace,
			Labels: map[string]string{
				"state":                 "in_queue",
				constants.ErrorCountKey: "10",
			},
		},
		Spec: osbv1alpha1.SFServiceInstanceSpec{
			ServiceID:        "service-id",
			PlanID:           "plan-id",
			RawContext:       nil,
			OrganizationGUID: "organization-guid",
			SpaceGUID:        "space-guid",
			RawParameters:    nil,
			PreviousValues:   nil,
		},
		Status: osbv1alpha1.SFServiceInstanceStatus{
			State: "in_queue",
		},
	}
	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	err = c.Create(context.TODO(), instance2)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	serviceInstance := &osbv1alpha1.SFServiceInstance{}
	g.Eventually(func() error {
		return c.Get(context.TODO(), instanceKey, serviceInstance)
	}, timeout).Should(gomega.Succeed())

	r := &ReconcileSFServiceInstance{
		Client:          c,
		Log:             ctrlrun.Log.WithName("provisioners").WithName("instance"),
		scheme:          mgr.GetScheme(),
		clusterRegistry: mockClusterRegistry,
		resourceManager: mockResourceManager,
	}
	type args struct {
		object        *osbv1alpha1.SFServiceInstance
		result        reconcile.Result
		inputErr      error
		lastOperation string
		retryCount    int
		err           error
	}
	tests := []struct {
		name       string
		setup      func()
		args       args
		want       reconcile.Result
		wantErr    bool
		wantErrMsg string
	}{
		{
			/*Default test on "APIVersion Notfound" error is being used to test the InputErr not-null case.*/
			name: "return input error message if inputErr is not empty",
			args: args{
				object:        instance,
				result:        reconcile.Result{},
				err:           fmt.Errorf("Object 'apiVersion' is missing in 'unstructured object has no version'"),
				inputErr:      errors.NewMarshalError("Object 'apiVersion' is missing in 'unstructured object has no version'", err),
				lastOperation: "in_queue",
				retryCount:    0,
			},
			want:       reconcile.Result{},
			wantErr:    false,
			wantErrMsg: "Object 'apiVersion' is missing in 'unstructured object has no version'",
		},
		{
			name: "return default error message if inputErr is empty",
			setup: func() {
				g.Expect(c.Delete(context.TODO(), serviceInstance)).NotTo(gomega.HaveOccurred())
				g.Eventually(func() error {
					err := c.Get(context.TODO(), instanceKey, serviceInstance)
					if err != nil {
						return nil
					}
					return errors.NewMarshalError("", nil)
				}, timeout).Should(gomega.Succeed())
			},
			args: args{
				object:        instance2,
				result:        reconcile.Result{},
				inputErr:      errors.NewMarshalError("", nil),
				lastOperation: "in_queue",
				retryCount:    0,
			},
			want:       reconcile.Result{},
			wantErr:    false,
			wantErrMsg: "Service Broker Error, status code: ETIMEDOUT, error code: 10008",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			got, err := r.handleError(tt.args.object, tt.args.result, tt.args.inputErr, tt.args.lastOperation, tt.args.retryCount)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReconcileSFServiceInstance.handleError() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReconcileSFServiceInstance.handleError() = %v, want %v", got, tt.want)
			}
			if tt.args.object.Status.Description != tt.wantErrMsg {
				t.Errorf("ReconcileSFServiceInstance.handleError() error = %v, wantErrMessage = %v", tt.args.object.Status.Description, tt.wantErrMsg)
				return
			}
		})
	}
}

func TestReconcileSFServiceInstance_setInProgress(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var instance = &osbv1alpha1.SFServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance-id",
			Namespace: constants.InteroperatorNamespace,
		},
		Spec: osbv1alpha1.SFServiceInstanceSpec{
			ServiceID:        "service-id",
			PlanID:           "plan-id",
			RawContext:       nil,
			OrganizationGUID: "organization-guid",
			SpaceGUID:        "space-guid",
			RawParameters:    nil,
			PreviousValues:   nil,
		},
		Status: osbv1alpha1.SFServiceInstanceStatus{
			State: "in_queue",
		},
	}
	var instanceKey = types.NamespacedName{Name: "instance-id", Namespace: constants.InteroperatorNamespace}
	subResource := osbv1alpha1.Source{
		APIVersion: "foo",
		Kind:       "bar",
		Name:       "subresource",
		Namespace:  constants.InteroperatorNamespace,
	}

	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c, err = client.New(cfg, client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	mockResourceManager := mock_resources.NewMockResourceManager(ctrl)
	mockClusterRegistry := mock_clusterRegistry.NewMockClusterRegistry(ctrl)

	r := &ReconcileSFServiceInstance{
		Client:          c,
		Log:             ctrlrun.Log.WithName("provisioners").WithName("instance"),
		scheme:          mgr.GetScheme(),
		clusterRegistry: mockClusterRegistry,
		resourceManager: mockResourceManager,
	}

	type args struct {
		namespacedName types.NamespacedName
		state          string
		resources      []osbv1alpha1.Source
		retryCount     int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		setup   func()
		cleanup func()
	}{
		{
			name: "Set the state to in progress",
			args: args{
				namespacedName: instanceKey,
				state:          "in_queue",
				resources: []osbv1alpha1.Source{
					subResource,
				},
				retryCount: 0,
			},
			setup: func() {
				instance.SetResourceVersion("")
				instance.SetState("in_queue")
				instance.SetLabels(nil)
				g.Expect(c.Create(context.TODO(), instance)).NotTo(gomega.HaveOccurred())
				g.Eventually(func() error {
					return c.Get(context.TODO(), instanceKey, instance)
				}, timeout).Should(gomega.Succeed())
			},
			wantErr: false,
			cleanup: func() {
				g.Expect(c.Get(context.TODO(), instanceKey, instance)).NotTo(gomega.HaveOccurred())
				g.Expect(instance.GetState()).To(gomega.Equal("in progress"))
				g.Expect(instance.GetLabels()).To(gomega.HaveKeyWithValue(constants.LastOperationKey, "in_queue"))
				g.Expect(instance.Status.Resources).To(gomega.ContainElement(subResource))
				g.Expect(c.Delete(context.TODO(), instance)).NotTo(gomega.HaveOccurred())
				g.Eventually(func() error {
					err := c.Get(context.TODO(), instanceKey, instance)
					if err != nil {
						if apierrors.IsNotFound(err) {
							return nil
						}
						return err
					}
					return fmt.Errorf("not deleted")
				}, timeout).Should(gomega.Succeed())
			},
		},
		{
			name: "Not set state to in progress if current state is different",
			args: args{
				namespacedName: instanceKey,
				state:          "in_queue",
				resources: []osbv1alpha1.Source{
					subResource,
				},
				retryCount: 0,
			},
			setup: func() {
				instance.SetResourceVersion("")
				instance.SetState("update")
				instance.SetLabels(nil)
				g.Expect(c.Create(context.TODO(), instance)).NotTo(gomega.HaveOccurred())
				g.Eventually(func() error {
					return c.Get(context.TODO(), instanceKey, instance)
				}, timeout).Should(gomega.Succeed())
			},
			wantErr: false,
			cleanup: func() {
				g.Expect(c.Get(context.TODO(), instanceKey, instance)).NotTo(gomega.HaveOccurred())
				g.Expect(instance.GetState()).NotTo(gomega.Equal("in progress"))
				g.Expect(instance.GetLabels()).NotTo(gomega.HaveKeyWithValue(constants.LastOperationKey, "in_queue"))
				g.Expect(instance.Status.Resources).To(gomega.ContainElement(subResource))
				g.Expect(c.Delete(context.TODO(), instance)).NotTo(gomega.HaveOccurred())
				g.Eventually(func() error {
					err := c.Get(context.TODO(), instanceKey, instance)
					if err != nil {
						if apierrors.IsNotFound(err) {
							return nil
						}
						return err
					}
					return fmt.Errorf("not deleted")
				}, timeout).Should(gomega.Succeed())
			},
		},
		{
			name: "fail if instance is gone",
			args: args{
				namespacedName: instanceKey,
				state:          "in_queue",
				resources: []osbv1alpha1.Source{
					subResource,
				},
				retryCount: 0,
			},
			setup: func() {
				g.Eventually(func() error {
					err := c.Get(context.TODO(), instanceKey, instance)
					if err != nil {
						if apierrors.IsNotFound(err) {
							return nil
						}
						return err
					}
					return fmt.Errorf("not deleted")
				}, timeout).Should(gomega.Succeed())
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			if tt.cleanup != nil {
				defer tt.cleanup()
			}
			if err := r.setInProgress(tt.args.namespacedName, tt.args.state, tt.args.resources, tt.args.retryCount); (err != nil) != tt.wantErr {
				t.Errorf("ReconcileSFServiceInstance.setInProgress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
