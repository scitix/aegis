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
)

type IAnalyzer interface {
	Analyze(analysis Analyzer) (*Result, error)
	Prompt(result *Result) string
}

type Analyzer struct {
	kcommon.Analyzer
	Name string
}

type PreAnalysis struct {
	kcommon.PreAnalysis
}

type Result struct {
	kcommon.Result
	Warning []Warning `json:"warning"`
	Info    []Info    `json:"info"`
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
