/*
Copyright 2023 The K8sGPT Authors.
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

package common

import (
	kcommon "github.com/k8sgpt-ai/k8sgpt/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type IAnalyzer interface {
	Analyze(analysis Analyzer) (*Result, error)
	Prompt(result *Result) string
}

// PodLogConfig controls how container logs are fetched and filtered for diagnosis.
// nil → legacy behaviour (fetch 60 lines, no keyword filtering).
type PodLogConfig struct {
	// FetchLines is the number of log lines to retrieve from the container. Default: 1000.
	FetchLines int
	// Keywords is a list of case-insensitive substrings used to filter log lines.
	// An empty list disables filtering (all fetched lines are candidates for output).
	Keywords []string
	// MaxOutputLines is the maximum number of lines sent to the LLM. Default: 60.
	MaxOutputLines int
}

type Analyzer struct {
	kcommon.Analyzer
	Name           string
	CollectorImage string
	EnableProm     bool
	EnablePodLog   *bool
	PodLogConfig   *PodLogConfig
	Owner          metav1.Object
}

type PreAnalysis struct {
	kcommon.PreAnalysis
}

type Result struct {
	kcommon.Result
	Warning  []Warning         `json:"warning"`
	Info     []Info            `json:"info"`
	Metadata map[string]string `json:"metadata"`
}

type Info struct {
	Text          string
	KubernetesDoc string
	Sensitive     []kcommon.Sensitive
}

type Warning struct {
	Text          string
	KubernetesDoc string
	Sensitive     []kcommon.Sensitive
}
