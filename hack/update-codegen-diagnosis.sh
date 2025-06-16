#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_ROOT="${SCRIPT_DIR}/.."
CODEGEN_PKG="${CODEGEN_PKG:-${SCRIPT_ROOT}/vendor/k8s.io/code-generator}"

source "${CODEGEN_PKG}/kube_codegen.sh"

# kube::codegen::gen_helpers \
#   --boilerplate "${SCRIPT_ROOT}/hack/boilerplate/boilerplate.generatego.txt" \
#   "${SCRIPT_ROOT}"

# kube::codegen::gen_register \
#   --boilerplate "${SCRIPT_ROOT}/hack/boilerplate/boilerplate.generatego.txt" \
#   "${SCRIPT_ROOT}"

kube::codegen::gen_client \
  --with-watch \
  --output-dir "${SCRIPT_ROOT}/pkg/generated/diagnosis" \
  --output-pkg "github.com/scitix/aegis/pkg/generated/diagnosis" \
  --boilerplate "${SCRIPT_ROOT}/hack/boilerplate/boilerplate.generatego.txt" \
  --one-input-api "diagnosis/v1alpha1" \
  "${SCRIPT_ROOT}/pkg/apis"
