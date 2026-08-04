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

# Collect per-repo catalog image lists into <APPSCODE_CLOUD_TAG>/images/<org>.yaml.
#
# For every installer repo it clones the given tag, regenerates the catalog via
# that repo's own hack/scripts/update-catalog.sh (which drives the image-packer
# binary), and copies catalog/imagelist.yaml into the output directory.
#
# For appscode-cloud/installer only, it also copies the catalog chart lists
# (ace.yaml, editor-charts.yaml, feature-charts.yaml, reusable-ui-charts.yaml)
# into <APPSCODE_CLOUD_TAG>/charts/.
#
# Required env vars (each is the git ref to checkout for that repo):
#   APPSCODE_CLOUD_TAG   -> appscode-cloud/installer   (also names the output dir)
#   KUBEDB_TAG           -> kubedb/installer
#   KUBESTASH_TAG        -> kubestash/installer
#   KUBEOPS_TAG          -> kubeops/installer
#   KLUSTER_MANAGER_TAG  -> kluster-manager/installer
#   OPEN_VIZ_TAG         -> open-viz/installer
#   OPNPULSE_TAG         -> opnpulse/installer

set -eou pipefail

# repo org | tag env var | output file basename (== org)
REPOS=(
    "appscode-cloud|APPSCODE_CLOUD_TAG"
    "kubedb|KUBEDB_TAG"
    "kubestash|KUBESTASH_TAG"
    "kubeops|KUBEOPS_TAG"
    "kluster-manager|KLUSTER_MANAGER_TAG"
    "open-viz|OPEN_VIZ_TAG"
    "opnpulse|OPNPULSE_TAG"
)

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# Seed tags from default-tags.env when present; already-set env vars win.
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
SCRIPTS_DIR="${OUT_DIR}/scripts"
SCRIPTS_SRC="${REPO_ROOT}/bare-scripts"

# Chart lists copied from appscode-cloud/installer's catalog/ into CHARTS_DIR.
CHART_FILES=(ace.yaml editor-charts.yaml feature-charts.yaml reusable-ui-charts.yaml)

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

mkdir -p "${IMAGES_DIR}" "${CHARTS_DIR}" "${SCRIPTS_DIR}"

# Ship the bare helper scripts (import/export/copy) and notes into the branch as-is.
cp "${SCRIPTS_SRC}"/* "${SCRIPTS_DIR}/"

# image-packer drives each repo's hack/scripts/update-catalog.sh. Install the
# version pinned by appscode-cloud/installer's go.mod at APPSCODE_CLOUD_TAG so it
# matches the release being collected. image-packer's go.mod carries replace
# directives, so `go install pkg@version` is rejected; instead build from source
# at the pinned ref, where its go.mod is the main module and replaces are honored.
echo "==> resolving image-packer version from appscode-cloud/installer @ ${APPSCODE_CLOUD_TAG}"
gomod="$(curl -fsSL "https://raw.githubusercontent.com/appscode-cloud/installer/${APPSCODE_CLOUD_TAG}/go.mod")"
ipk_ver="$(echo "${gomod}" | awk '/kmodules.xyz\/image-packer/ {print $2; exit}')"
if [ -z "${ipk_ver}" ]; then
    echo "ERROR: could not resolve image-packer version from appscode-cloud/installer go.mod" >&2
    exit 1
fi
# A pseudo-version (vX-YYYYMMDDhhmmss-<12-hex-commit>) resolves to its commit;
# a plain tag is used as-is.
if [[ "${ipk_ver}" =~ -([0-9a-f]{12})$ ]]; then
    ipk_ref="${BASH_REMATCH[1]}"
else
    ipk_ref="${ipk_ver}"
fi
GOBIN="$(go env GOPATH)/bin"
echo "--> building kmodules.xyz/image-packer @ ${ipk_ref} (from ${ipk_ver})"
git clone --filter=blob:none https://github.com/kmodules/image-packer.git "${WORK_DIR}/image-packer"
git -C "${WORK_DIR}/image-packer" checkout --quiet "${ipk_ref}"
( cd "${WORK_DIR}/image-packer" && go build -o "${GOBIN}/image-packer" . )
export PATH="${GOBIN}:${PATH}"

for entry in "${REPOS[@]}"; do
    org="${entry%%|*}"
    tag_var="${entry##*|}"
    tag="${!tag_var:-}"

    if [ -z "${tag}" ]; then
        echo "ERROR: ${tag_var} is not set for ${org}/installer" >&2
        exit 1
    fi

    echo "==> ${org}/installer @ ${tag}"
    src="${WORK_DIR}/${org}"
    git clone --depth 1 --branch "${tag}" "https://github.com/${org}/installer.git" "${src}"

    echo "--> update-catalog (${org})"
    ( cd "${src}" && ./hack/scripts/update-catalog.sh )

    imagelist="${src}/catalog/imagelist.yaml"
    if [ ! -f "${imagelist}" ]; then
        echo "ERROR: ${imagelist} not found after update-catalog for ${org}/installer" >&2
        exit 1
    fi

    cp "${imagelist}" "${IMAGES_DIR}/${org}.yaml"
    echo "--> wrote ${IMAGES_DIR}/${org}.yaml"

    if [ "${org}" = "appscode-cloud" ]; then
        for chart in "${CHART_FILES[@]}"; do
            chartsrc="${src}/catalog/${chart}"
            if [ ! -f "${chartsrc}" ]; then
                echo "ERROR: ${chartsrc} not found for appscode-cloud/installer" >&2
                exit 1
            fi
            cp "${chartsrc}" "${CHARTS_DIR}/${chart}"
            echo "--> wrote ${CHARTS_DIR}/${chart}"
        done
    fi
done

echo
echo "Collected image lists in ${IMAGES_DIR}:"
ls -1 "${IMAGES_DIR}"
echo
echo "Collected chart lists in ${CHARTS_DIR}:"
ls -1 "${CHARTS_DIR}"
