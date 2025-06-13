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
#   github.com/scitix/aegis/pkg/generated/alert \
#   github.com/scitix/aegis/pkg/apis \
#   github.com/scitix/aegis/pkg/apis \
#   "alert:v1alpha1" \
#   --go-header-file $(pwd)/boilerplate/boilerplate.generatego.txt

# bash ../vendor/k8s.io/code-generator/generate-internal-groups.sh \
#   "deepcopy,client,informer,lister" \
#   github.com/scitix/aegis/pkg/generated/rule \
#   github.com/scitix/aegis/pkg/apis \
#   github.com/scitix/aegis/pkg/apis \
#   "rule:v1alpha1" \
#   --go-header-file $(pwd)/boilerplate/boilerplate.generatego.txt

# bash ../vendor/k8s.io/code-generator/generate-internal-groups.sh \
#   "deepcopy,client,informer,lister" \
#   github.com/scitix/aegis/pkg/generated/template \
#   github.com/scitix/aegis/pkg/apis \
#   github.com/scitix/aegis/pkg/apis \
#   "template:v1alpha1" \
#   --go-header-file $(pwd)/boilerplate/boilerplate.generatego.txt

# bash ../vendor/k8s.io/code-generator/generate-internal-groups.sh \
#   "deepcopy,client,informer,lister" \
#   github.com/scitix/aegis/pkg/generated/check \
#   github.com/scitix/aegis/pkg/apis \
#   github.com/scitix/aegis/pkg/apis \
#   "check:v1alpha1" \
#   --go-header-file $(pwd)/boilerplate/boilerplate.generatego.txt

bash vendor/k8s.io/code-generator/generate-internal-groups.sh all \
  github.com/scitix/aegis/pkg/generated/diagnosis \
  github.com/scitix/aegis/pkg/apis \
  github.com/scitix/aegis/pkg/apis \
  "diagnosis:v1alpha1" \
  --go-header-file ./hack/boilerplate/boilerplate.generatego.txt 

# bash ../vendor/k8s.io/code-generator/generate-internal-groups.sh \
#   "deepcopy,client,informer,lister" \
#   github.com/scitix/aegis/pkg/generated/machinecheck \
#   github.com/scitix/aegis/pkg/apis \
#   github.com/scitix/aegis/pkg/apis \
#   "machinecheck:v1alpha1" \
#   --go-header-file $(pwd)/boilerplate/boilerplate.generatego.txt