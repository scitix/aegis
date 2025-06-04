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
	nodecheck "gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/nodecheck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AegisClusterHealthCheck describe a check
type AegisClusterHealthCheck struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// AegisClusterHealthCheckSpec defines the check spec.
	// +optional
	Spec AegisClusterHealthCheckSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	// AegisClusterHealthCheckStatus defines the template status.
	// +optional
	Status AegisClusterHealthCheckStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// AegisClusterHealthCheckSpec defines the check spec.
type AegisClusterHealthCheckSpec struct {
	// +optional
	Schedule string `json:"schedule,omitempty" protobuf:"bytes,1,rep,name=schedule"`

	// +optional
	Timeout *int32 `json:"timeout,omitempty" protobuf:"bytes,2,rep,name=timeout"`

	// +optional
	RuleConfigmapSelector metav1.LabelSelector `json:"ruleConfigmapSelector,omitempty" protobuf:"bytes,3,opt,name=ruleConfigmapSelector"`

	// +optional
	// NodeSelector metav1.LabelSelector `json:"nodeSelector,omitempty" protobuf:"bytes,5,opt,name=nodeSelector"`

	// +optional
	Template corev1.PodTemplateSpec `json:"template,omitempty" protobuf:"bytes,4,opt,name=template"`
}

type CheckType string

const (
	CheckTypeNode CheckType = "Node"
)

type CheckResult struct {
	// +optional
	Name string `json:"name" protobuf:"bytes,1,rep,name=name"`

	// +optional
	ResultInfos nodecheck.ResultInfos `json:"resultInfos" protobuf:"bytes,2,rep,name=resultInfos"`
}

type CheckResults struct {
	// +optional
	CheckType CheckType `json:"checkType" protobuf:"bytes,1,rep,name=checkType"`

	// +optional
	Results []CheckResult `json:"results" protobuf:"bytes,2,rep,name=results"`
}

// AegisClusterHealthCheckStatus defines the check status.
type AegisClusterHealthCheckStatus struct {
	// The latest available observations of an object's current state
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=type
	// +listType=atomic
	Conditions []CheckCondition `json:"conditions,omitempty" protobuf:"bytes,1,rep,name=conditions"`

	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty" protobuf:"bytes,2,rep,name=startTime"`

	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty" protobuf:"bytes,3,rep,name=completionTime"`

	// Phase is the check phase.
	// +optional
	Phase CheckPhase `json:"phase,omitempty" protobuf:"bytes,4,rep,name=phase"`

	// +optional
	CheckResults *CheckResults `json:"checkResults,omitempty" protobuf:"bytes,5,rep,name=checkResults"`

	// +optional
	Active    int32  `json:"active" protobuf:"bytes,6,rep,name=active"`
	Desired   *int32 `json:"desired" protobuf:"bytes,7,rep,name=desired"`
	Failed    int32  `json:"failed" protobuf:"bytes,8,rep,name=failed"`
	Succeeded int32  `json:"succeeded" protobuf:"bytes,9,rep,name=succeeded"`
}

type CheckConditionType string

const (
	CheckSucceededCreateNodeCheck CheckConditionType = "SucceededCreateNodeCheck"
	CheckFailedCreateNodeCheck    CheckConditionType = "FailedCreateNodeCheck"
	Checking                      CheckConditionType = "Checking"
	CheckCompleted                CheckConditionType = "Completed"
)

type CheckCondition struct {
	Type               CheckConditionType     `json:"type,omitempty" protobuf:"bytes,1,rep,name=type"`
	Status             corev1.ConditionStatus `json:"status,omitempty" protobuf:"bytes,2,rep,name=status"`
	LastProbeTime      metav1.Time            `json:"lastProbeTime,omitempty" protobuf:"bytes,3,rep,name=lastProbeTime"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty" protobuf:"bytes,4,rep,name=lastTransitionTime"`
	Reason             string                 `json:"reason,omitempty" protobuf:"bytes,5,rep,name=reason"`
	Message            string                 `json:"message,omitempty" protobuf:"bytes,6,rep,name=message"`
}

type CheckPhase string

const (
	CheckPhaseUnknown   CheckPhase = "Unknown"
	CheckPhasePending   CheckPhase = "Pending"
	CheckPhaseChecking  CheckPhase = "Checking"
	CheckPhaseFailed    CheckPhase = "Failed"
	CheckPhaseCompleted CheckPhase = "Completed"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AegisClusterHealthCheckList is a list of AegisClusterHealthCheck items.
type AegisClusterHealthCheckList struct {
	metav1.TypeMeta `json:",inline"`

	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is a list of AegisNodeHealthCheck objects.
	Items []AegisClusterHealthCheck `json:"items" protobuf:"bytes,2,rep,name=items"`
}
