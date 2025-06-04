#!/usr/bin/env bash

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

echo $SCRIPT_ROOT
echo $CODEGEN_PKG
# bash ../vendor/k8s.io/code-generator/generate-internal-groups.sh \
#   "deepcopy,client,informer,lister" \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/alert \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/apis \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/apis \
#   "alert:v1alpha1" \
#   --go-header-file $(pwd)/boilerplate/boilerplate.generatego.txt

# bash ../vendor/k8s.io/code-generator/generate-internal-groups.sh \
#   "deepcopy,client,informer,lister" \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/rule \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/apis \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/apis \
#   "rule:v1alpha1" \
#   --go-header-file $(pwd)/boilerplate/boilerplate.generatego.txt

# bash ../vendor/k8s.io/code-generator/generate-internal-groups.sh \
#   "deepcopy,client,informer,lister" \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/template \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/apis \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/apis \
#   "template:v1alpha1" \
#   --go-header-file $(pwd)/boilerplate/boilerplate.generatego.txt

# bash ../vendor/k8s.io/code-generator/generate-internal-groups.sh \
#   "deepcopy,client,informer,lister" \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/check \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/apis \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/apis \
#   "check:v1alpha1" \
#   --go-header-file $(pwd)/boilerplate/boilerplate.generatego.txt

bash vendor/k8s.io/code-generator/generate-internal-groups.sh all \
  gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/diagnosis \
  gitlab.scitix-inner.ai/k8s/aegis/pkg/apis \
  gitlab.scitix-inner.ai/k8s/aegis/pkg/apis \
  "diagnosis:v1alpha1" \
  --go-header-file ./hack/boilerplate/boilerplate.generatego.txt 

# bash ../vendor/k8s.io/code-generator/generate-internal-groups.sh \
#   "deepcopy,client,informer,lister" \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/machinecheck \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/apis \
#   gitlab.scitix-inner.ai/k8s/aegis/pkg/apis \
#   "machinecheck:v1alpha1" \
#   --go-header-file $(pwd)/boilerplate/boilerplate.generatego.txt