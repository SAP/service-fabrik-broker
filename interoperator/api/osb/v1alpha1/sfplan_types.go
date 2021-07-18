/*
Copyright 2019 The Service Fabrik Authors.

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

package v1alpha1

import (
	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// List of templates to be provided for a service plan
const (
	ProvisionAction            = "provision"
	StatusAction               = "status"
	BindAction                 = "bind"
	UnbindAction               = "unbind"
	SourcesAction              = "sources"
	ClusterLabelSelectorAction = "clusterSelector"
)

// TemplateSpec is the specifcation of a template
type TemplateSpec struct {
	// +kubebuilder:validation:Enum=provision;status;bind;unbind;sources;clusterSelector
	Action string `yaml:"action" json:"action"`

	// +kubebuilder:validation:Enum=gotemplate;helm
	Type           string `yaml:"type" json:"type"`
	URL            string `yaml:"url,omitempty" json:"url,omitempty"`
	Content        string `yaml:"content,omitempty" json:"content,omitempty"`
	ContentEncoded string `yaml:"contentEncoded,omitempty" json:"contentEncoded,omitempty"`
}

// Schema definition for the input parameters.
type Schema struct {
	// +kubebuilder:pruning:PreserveUnknownFields
	Parameters *runtime.RawExtension `json:"parameters"`
}

// ServiceInstanceSchema is the schema definitions for creating
// and updating a Service Instance.
type ServiceInstanceSchema struct {
	Create *Schema `json:"create,omitempty"`
	Update *Schema `json:"update,omitempty"`
}

// ServiceBindingSchema is the  schema definition for creating a
// Service Binding. Used only if the Service Plan is bindable.
type ServiceBindingSchema struct {
	Create *Schema `json:"create,omitempty"`
}

// ServiceSchemas is definitions for Service Instances and
// Service Bindings for the Service Plan.
type ServiceSchemas struct {
	Instance ServiceInstanceSchema `json:"service_instance,omitempty"`
	Binding  ServiceBindingSchema  `json:"service_binding,omitempty"`
}

// MaintenanceInfo captures any maintainance related information for give plan
type MaintenanceInfo struct {
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// SFPlanSpec defines the desired state of SFPlan
type SFPlanSpec struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Description string `json:"description"`

	// +kubebuilder:pruning:PreserveUnknownFields
	Metadata               *runtime.RawExtension `json:"metadata,omitempty"`
	MaintenanceInfo        *MaintenanceInfo      `json:"maintenance_info,omitempty"`
	MaximumPollingDuration int                   `json:"maximum_polling_duration,omitempty"`
	Free                   bool                  `json:"free"`
	Bindable               bool                  `json:"bindable"`
	PlanUpdatable          bool                  `json:"planUpdatable,omitempty"`
	AutoUpdateInstances    bool                  `json:"autoUpdateInstances,omitempty"`
	Schemas                *ServiceSchemas       `json:"schemas,omitempty"`
	Templates              []TemplateSpec        `json:"templates"`
	ServiceID              string                `json:"serviceId"`

	// +kubebuilder:pruning:PreserveUnknownFields
	RawContext *runtime.RawExtension `json:"context,omitempty"`

	// +kubebuilder:pruning:PreserveUnknownFields
	Manager *runtime.RawExtension `json:"manager,omitempty"`
	// Add supported_platform field
}

// SFPlanStatus defines the observed state of SFPlan
type SFPlanStatus struct {
	SpecHash string `json:"specHash,omitempty"`
}

// +kubebuilder:object:root=true
// +genclient
// +genclient:noStatus
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="display-name",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="age",type=date,JSONPath=`.metadata.creationTimestamp`

// SFPlan is the Schema for the sfplans API
type SFPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SFPlanSpec   `json:"spec,omitempty"`
	Status SFPlanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SFPlanList contains a list of SFPlan
type SFPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SFPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SFPlan{}, &SFPlanList{})
}

// GetTemplate fetches the Template spec with the given action
func (sfPlan *SFPlan) GetTemplate(action string) (*TemplateSpec, error) {
	for _, template := range sfPlan.Spec.Templates {
		if template.Action == action {
			return &template, nil
		}
	}
	return nil, errors.NewTemplateNotFound(action, sfPlan.Spec.ID, nil)
}
