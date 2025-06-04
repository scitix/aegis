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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AegisDiagnosis describe a diagnosis
type AegisDiagnosis struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// AegisDiagnosisSpec defines the diagnosis details.
	// +optional
	Spec AegisDiagnosisSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	// AegisDiagnosisStatus defines the diagnosis status.
	// +optional
	Status AegisDiagnosisStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// TTLStrategy is the strategy for the time to live depending on if succeeded or failed
type TTLStrategy struct {
	// SecondsAfterCompletion is the number of seconds to live after completion
	SecondsAfterCompletion *int32 `json:"secondsAfterCompletion,omitempty" protobuf:"bytes,1,opt,name=secondsAfterCompletion"`
	// SecondsAfterSuccess is the number of seconds to live after success
	SecondsAfterSuccess *int32 `json:"secondsAfterSuccess,omitempty" protobuf:"bytes,2,opt,name=secondsAfterSuccess"`
	// SecondsAfterFailure is the number of seconds to live after failure
	SecondsAfterFailure *int32 `json:"secondsAfterFailure,omitempty" protobuf:"bytes,3,opt,name=secondsAfterFailure"`
}

// AegisDiagnosisSpec defines the diagnosis content.
type AegisDiagnosisSpec struct {
	// +optional
	Object AegisDiagnosisObject `json:"object,omitempty" protobuf:"bytes,1,rep,name=object"`

	// +optional
	Timeout string `json:"timeout,omitempty" protobuf:"bytes,2,opt,name=timeout"`

	// TTLStrategy limits the lifetime of a alert
	TTLStrategy *TTLStrategy `json:"ttlStrategy,omitempty" protobuf:"bytes,8,opt,name=ttlStrategy"`
}

type DiagnosisObjectKind string

const (
	NodeKind DiagnosisObjectKind = "Node"
	PodKind  DiagnosisObjectKind = "Pod"
)

type AegisDiagnosisObject struct {
	Kind      DiagnosisObjectKind `json:"kind,omitempty" protobuf:"bytes,1,rep,name=kind"`
	Name      string              `json:"name,omitempty" protobuf:"bytes,2,rep,name=name"`
	Namespace string              `json:"namespace,omitempty" protobuf:"bytes,3,rep,name=namespace"`
	Node      string              `json:"node,omitempty" protobuf:"bytes,4,rep,name=node"`
}

// AegisDiagnosisStatus defines the diagnosis status.
type AegisDiagnosisStatus struct {
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty" protobuf:"bytes,1,rep,name=startTime"`

	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty" protobuf:"bytes,2,rep,name=completionTime"`

	// Phase is the diagnosis phase.
	// +optional
	Phase DiagnosisPhase `json:"phase,omitempty" protobuf:"bytes,3,rep,name=phase"`

	// +optional
	Result *DiagnosisResult `json:"result,omitempty" protobuf:"bytes,4,rep,name=result"`

	// +optional
	Explain *string `json:"explain,omitempty" protobuf:"bytes,5,rep,name=explain"`

	// +optional
	ErrorResult *string `json:"errorResult,omitempty" protobuf:"bytes,6,rep,name=errorResult"`
}

type DiagnosisPhase string

const (
	DiagnosisPhaseUnknown    DiagnosisPhase = "Unknown"
	DiagnosisPhasePending    DiagnosisPhase = "Pending"
	DiagnosisPhaseDiagnosing DiagnosisPhase = "Diagnosing"
	DiagnosisPhaseFailed     DiagnosisPhase = "Failed"
	DiagnosisPhaseCompleted  DiagnosisPhase = "Completed"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AegisDiagnosisList is a list of AegisDiagnosis items.
type AegisDiagnosisList struct {
	metav1.TypeMeta `json:",inline"`

	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is a list of AegisDiagnosis objects.
	Items []AegisDiagnosis `json:"items" protobuf:"bytes,2,rep,name=items"`
}

type Status string

const (
	Healthy   Status = "Healthy"
	UnHealthy Status = "UnHealthy"
	UnKnown   Status = "Unknown"
)

type DiagnosisResult struct {
	Status   Status   `json:"status,omitempty" protobuf:"bytes,1,rep,name=status"`
	Failures []string `json:"failures,omitempty" protobuf:"bytes,2,rep,name=failures"`
	Warnings []string `json:"warnings,omitempty" protobuf:"bytes,3,rep,name=warnings"`
	Infos    []string `json:"infos,omitempty" protobuf:"bytes,1,rep,name=infos"`
}