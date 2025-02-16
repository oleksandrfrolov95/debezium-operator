/*
Copyright 2025.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DebeziumConnectorSpec defines the desired state of DebeziumConnector
type DebeziumConnectorSpec struct {
	// +kubebuilder:validation:Required
	DebeziumHost string `json:"debeziumHost"`
	// +kubebuilder:validation:Required
	Config map[string]string `json:"config"`
}

// DebeziumConnectorStatus defines the observed state of DebeziumConnector
type DebeziumConnectorStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:webhook:path=/validate-dbc,mutating=false,failurePolicy=fail,sideEffects=None,groups=api.debezium,resources=debeziumconnectors,verbs=create;update,versions=v1alpha1,name=vdebeziumconnector,admissionReviewVersions=v1

// DebeziumConnector is the Schema for the debeziumconnectors API
type DebeziumConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DebeziumConnectorSpec   `json:"spec,omitempty"`
	Status DebeziumConnectorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DebeziumConnectorList contains a list of DebeziumConnector
type DebeziumConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DebeziumConnector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DebeziumConnector{}, &DebeziumConnectorList{})
}
