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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_ROOT="${SCRIPT_DIR}/.."
CODEGEN_PKG="${CODEGEN_PKG:-${SCRIPT_ROOT}/vendor/k8s.io/code-generator}"

source "${CODEGEN_PKG}/kube_codegen.sh"

BOILERPLATE="${SCRIPT_ROOT}/hack/boilerplate/boilerplate.generatego.txt"
APIS_ROOT="${SCRIPT_ROOT}/pkg/apis"
MODULE="github.com/scitix/aegis"

# kube::codegen::gen_helpers \
#   --boilerplate "${SCRIPT_ROOT}/hack/boilerplate/boilerplate.generatego.txt" \
#   "${SCRIPT_ROOT}"

# kube::codegen::gen_register \
#   --boilerplate "${SCRIPT_ROOT}/hack/boilerplate/boilerplate.generatego.txt" \
#   "${SCRIPT_ROOT}"

# === Alert ===
kube::codegen::gen_client \
  --with-watch \
  --output-dir "${SCRIPT_ROOT}/pkg/generated/alert" \
  --output-pkg "${MODULE}/pkg/generated/alert" \
  --boilerplate "${BOILERPLATE}" \
  --one-input-api "alert/v1alpha1" \
  "${APIS_ROOT}"

# === Rule ===
kube::codegen::gen_client \
  --with-watch \
  --output-dir "${SCRIPT_ROOT}/pkg/generated/rule" \
  --output-pkg "${MODULE}/pkg/generated/rule" \
  --boilerplate "${BOILERPLATE}" \
  --one-input-api "rule/v1alpha1" \
  "${APIS_ROOT}"

# === Template ===
kube::codegen::gen_client \
  --with-watch \
  --output-dir "${SCRIPT_ROOT}/pkg/generated/template" \
  --output-pkg "${MODULE}/pkg/generated/template" \
  --boilerplate "${BOILERPLATE}" \
  --one-input-api "template/v1alpha1" \
  "${APIS_ROOT}"

# === Check ===
kube::codegen::gen_client \
  --with-watch \
  --output-dir "${SCRIPT_ROOT}/pkg/generated/clustercheck" \
  --output-pkg "${MODULE}/pkg/generated/clustercheck" \
  --boilerplate "${BOILERPLATE}" \
  --one-input-api "clustercheck/v1alpha1" \
  "${APIS_ROOT}"

kube::codegen::gen_client \
  --with-watch \
  --output-dir "${SCRIPT_ROOT}/pkg/generated/nodecheck" \
  --output-pkg "${MODULE}/pkg/generated/nodecheck" \
  --boilerplate "${BOILERPLATE}" \
  --one-input-api "nodecheck/v1alpha1" \
  "${APIS_ROOT}"

# === Diagnosis ===
kube::codegen::gen_client \
  --with-watch \
  --output-dir "${SCRIPT_ROOT}/pkg/generated/diagnosis" \
  --output-pkg "${MODULE}/pkg/generated/diagnosis" \
  --boilerplate "${BOILERPLATE}" \
  --one-input-api "diagnosis/v1alpha1" \
  "${APIS_ROOT}"
