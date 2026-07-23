#!/bin/bash

# Copyright AppsCode Inc. and Contributors
#
# Licensed under the AppsCode Community License 1.0.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Community-1.0.0.md
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Collect image lists for individual OCI charts that carry curated CI values.
#
# For each chart it renders `helm template` with the values file under hack/ci/
# and writes the referenced images into <APPSCODE_CLOUD_TAG>/images/<name>.yaml.
# The chart version is resolved from the chart lists already collected by
# collect-from-orgs.sh (<APPSCODE_CLOUD_TAG>/charts/*.yaml), so it stays in sync
# with the release without a hardcoded version here.
#
# Must run AFTER collect-from-orgs.sh (needs charts/ for versions and images/ to exist).
#
# Required env var:
#   APPSCODE_CLOUD_TAG -> names the output dir (same as collect-from-orgs.sh)

set -eou pipefail

# chart name (== OCI repo basename == output basename) | values file under hack/ci/
CHARTS=(
    "kube-prometheus-stack|prometheus-stack-ci-values.yaml"
    "cert-manager|cert-manager-ci-values.yaml"
)

OCI_PREFIX="oci://ghcr.io/appscode-charts"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

ENV_FILE="${REPO_ROOT}/default-tags.env"
if [ -f "${ENV_FILE}" ]; then
    while IFS='=' read -r k v; do
        [[ "${k}" =~ ^[A-Z_]+$ ]] || continue
        [ -z "${!k:-}" ] && export "${k}=${v}"
    done < <(grep -E '^[A-Z_]+=' "${ENV_FILE}")
fi

: "${APPSCODE_CLOUD_TAG:?APPSCODE_CLOUD_TAG must be set (names the output dir)}"
OUT_DIR="${REPO_ROOT}/${APPSCODE_CLOUD_TAG}"
IMAGES_DIR="${OUT_DIR}/images"
CHARTS_DIR="${OUT_DIR}/charts"
VALUES_DIR="${REPO_ROOT}/hack/ci"

if [ ! -d "${CHARTS_DIR}" ]; then
    echo "ERROR: ${CHARTS_DIR} not found; run collect-from-orgs.sh first" >&2
    exit 1
fi
mkdir -p "${IMAGES_DIR}"

# Extract image refs from a rendered manifest: both `image:` fields and
# container args of the form --flag=<registry>/... (config-reloader, thanos,
# acme solver), restricted to the registries these charts pull from.
extract_images() {
    local text="$1"
    {
        grep -oE 'image:[[:space:]]*"?[^"[:space:]]+' <<<"${text}" | sed -E 's/image:[[:space:]]*"?//' || true
        grep -oE '\-\-[a-z0-9-]+=(quay\.io|registry\.k8s\.io|docker\.io|ghcr\.io)[^"[:space:]]+' <<<"${text}" | sed -E 's/^--[a-z0-9-]+=//' || true
    } | sort -u
}

for entry in "${CHARTS[@]}"; do
    name="${entry%%|*}"
    values="${entry##*|}"
    values_path="${VALUES_DIR}/${values}"

    if [ ! -f "${values_path}" ]; then
        echo "ERROR: values file ${values_path} not found for ${name}" >&2
        exit 1
    fi

    ver="$(grep -hoE "appscode-charts/${name}:[^\"[:space:]]+" "${CHARTS_DIR}"/*.yaml | head -1 | sed -E "s#.*/${name}:##")"
    if [ -z "${ver}" ]; then
        echo "ERROR: could not resolve version for ${name} from ${CHARTS_DIR}/*.yaml" >&2
        exit 1
    fi

    echo "==> ${name} @ ${ver}"
    render="$(helm template "${name}" "${OCI_PREFIX}/${name}" --version "${ver}" \
        -n "${name}" -f "${values_path}")"

    out="${IMAGES_DIR}/${name}.yaml"
    extract_images "${render}" | sed 's/^/- /' >"${out}"
    echo "--> wrote ${out} ($(grep -c '^- ' "${out}") images)"
done
