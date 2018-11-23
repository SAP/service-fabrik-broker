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

package v1alpha1

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceInstanceOptions defines all the options passed from OSB.
// Director/Docker operators also expect this format.
type ServiceInstanceOptions struct {
	ServiceID        string          `json:"service_id"`
	PlanID           string          `json:"plan_id"`
	RawContext       json.RawMessage `json:"context,omitempty"`
	OrganizationGUID string          `json:"organization_guid"`
	SpaceGUID        string          `json:"space_guid"`
	RawParameters    json.RawMessage `json:"parameters,omitempty"`

	// ServiceID        string `json:"serviceId"`
	// PlanID           string `json:"planId"`
	// RawContext       string `json:"context,omitempty"`
	// OrganizationGUID string `json:"organizationGuid"`
	// SpaceGUID        string `json:"spaceGuid"`
	// RawParameters    string `json:"parameters,omitempty"`
}

// ServiceInstanceSpec defines the desired state of ServiceInstance
type ServiceInstanceSpec struct {
	OptionsString string `json:"options,omitempty"`
	Options       ServiceInstanceOptions
}

// ServiceInstanceStatus defines the observed state of ServiceInstance
type ServiceInstanceStatus struct {
	DashboardURL  string `yaml:"dashboardUrl,omitempty" json:"dashboardUrl,omitempty"`
	State         string `yaml:"state" json:"state"`
	Error         string `yaml:"error,omitempty" json:"error,omitempty"`
	LastOperation string `yaml:"lastOperation,omitempty" json:"lastOperation,omitempty"`
	Response      string `yaml:"response,omitempty" json:"response,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceInstance is the Schema for the serviceinstances API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type ServiceInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceInstanceSpec   `json:"spec,omitempty"`
	Status ServiceInstanceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceInstanceList contains a list of ServiceInstance
type ServiceInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceInstance{}, &ServiceInstanceList{})
}