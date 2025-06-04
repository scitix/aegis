/*
Copyright 2020 The Kubernetes Authors.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AegisAlertOpsRule describe a alert event
type AegisAlertOpsRule struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// AegisAlertOpsRuleSpec defines the ops rule.
	// +optional
	Spec AegisAlertOpsRuleSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	// AegisAlertOpsRuleStatus defines the rule status.
	// +optional
	Status AegisAlertOpsRuleStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// AegisAlertOpsRuleSpec defines the alert ops rule.
type AegisAlertOpsRuleSpec struct {
	Selector        *metav1.LabelSelector   `json:"selector,omitempty" protobuf:"bytes,1,rep,name=selector"`
	AlertConditions []AegisAlertCondition   `json:"alertConditions,omitempty" protobuf:"bytes,4,opt,name=alertConditions"`
	OpsTemplate     *corev1.ObjectReference `json:"opsTemplate,omitempty" protobuf:"bytes,1,rep,name=opsTemplate"`
}

type AegisAlertCondition struct {
	Type   string `json:"type,omitempty" protobuf:"bytes,1,rep,name=type"`
	Status string `json:"status,omitempty" protobuf:"bytes,1,rep,name=status"`
}

// type AegisAlertOpsRuleStatus struct defines the rule status.
type AegisAlertOpsRuleStatus struct {
	// Status is the rule status.
	// +optional
	Status string `json:"status,omitempty" protobuf:"bytes,3,rep,name=status"`
	// TriggerStatus is the rule trigger ops statisis.
	// +optional
	TriggerStatus TriggerStatus `json:"triggerStatus,omitempty" protobuf:"bytes,3,rep,name=triggerStatus"`
}

type TriggerStatus struct {
	Succeeded int `json:"succeeded,omitempty" protobuf:"bytes,1,rep,name=succeeded"`
	Failed    int `json:"failed,omitempty" protobuf:"bytes,2,rep,name=failed"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AegisAlertOpsRuleList is a list of AegisAlertOpsRule items.
type AegisAlertOpsRuleList struct {
	metav1.TypeMeta `json:",inline"`

	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is a list of AegisAlertOpsRule objects.
	Items []AegisAlertOpsRule `json:"items" protobuf:"bytes,2,rep,name=items"`
}
