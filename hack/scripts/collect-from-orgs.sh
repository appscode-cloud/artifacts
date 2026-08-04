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
# APPSCODE_CLOUD_TAG is the only input; it names the output dir and the release
# being collected. Every component repo's tag is derived from it: each
# component owns one anchor chart whose version appscode-cloud/installer pins in
# its catalog lists, and that pinned version is the component's tag. Picking the
# component tags by hand collects images for chart versions this ACE release does
# not deploy, which silently breaks an air-gapped mirror.
#
# Each derived tag can still be overridden by exporting its env var (KUBEDB_TAG,
# KUBESTASH_TAG, KUBEOPS_TAG, KLUSTER_MANAGER_TAG, OPEN_VIZ_TAG, OPNPULSE_TAG),
# e.g. to collect an rc ahead of an ACE release; each override is logged.

set -eou pipefail

# component org | tag env var | anchor chart pinned by appscode-cloud/installer
COMPONENTS=(
    "kubedb|KUBEDB_TAG|kubedb"
    "kubestash|KUBESTASH_TAG|kubestash"
    "kubeops|KUBEOPS_TAG|kube-ui-server"
    "kluster-manager|KLUSTER_MANAGER_TAG|cluster-profile-manager"
    "open-viz|OPEN_VIZ_TAG|monitoring-operator"
    "opnpulse|OPNPULSE_TAG|appscode-otel-stack"
)

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

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

collect_repo() {
    local org="$1" tag="$2" src="${WORK_DIR}/$1"

    echo "==> ${org}/installer @ ${tag}"
    git clone --depth 1 --branch "${tag}" "https://github.com/${org}/installer.git" "${src}"

    echo "--> update-catalog (${org})"
    ( cd "${src}" && ./hack/scripts/update-catalog.sh )

    local imagelist="${src}/catalog/imagelist.yaml"
    if [ ! -f "${imagelist}" ]; then
        echo "ERROR: ${imagelist} not found after update-catalog for ${org}/installer" >&2
        exit 1
    fi

    cp "${imagelist}" "${IMAGES_DIR}/${org}.yaml"
    echo "--> wrote ${IMAGES_DIR}/${org}.yaml"
}

# appscode-cloud/installer goes first: its catalog chart lists are what the
# component tags are derived from.
collect_repo appscode-cloud "${APPSCODE_CLOUD_TAG}"

for chart in "${CHART_FILES[@]}"; do
    chartsrc="${WORK_DIR}/appscode-cloud/catalog/${chart}"
    if [ ! -f "${chartsrc}" ]; then
        echo "ERROR: ${chartsrc} not found for appscode-cloud/installer" >&2
        exit 1
    fi
    cp "${chartsrc}" "${CHARTS_DIR}/${chart}"
    echo "--> wrote ${CHARTS_DIR}/${chart}"
done

echo
echo "==> deriving component tags from appscode-cloud/installer @ ${APPSCODE_CLOUD_TAG}"
for entry in "${COMPONENTS[@]}"; do
    org="${entry%%|*}"
    rest="${entry#*|}"
    tag_var="${rest%%|*}"
    anchor="${rest##*|}"

    # `|| true` keeps pipefail from killing the run on a no-match, so the
    # explicit error below is what the user sees.
    resolved="$(grep -hoE "appscode-charts/${anchor}:[^\"[:space:]]+" "${CHARTS_DIR}"/*.yaml | head -1 | sed -E "s#.*/${anchor}:##" || true)"
    if [ -z "${resolved}" ]; then
        echo "ERROR: anchor chart ${anchor} (${org}/installer) not pinned in ${CHARTS_DIR}/*.yaml;" >&2
        echo "       it may have been renamed in ${APPSCODE_CLOUD_TAG} — update COMPONENTS" >&2
        exit 1
    fi

    given="${!tag_var:-}"
    if [ -n "${given}" ] && [ "${given}" != "${resolved}" ]; then
        echo "WARNING: ${tag_var} overridden: ${resolved} (from ${anchor}) -> ${given}" >&2
    else
        echo "--> ${tag_var}=${resolved} (from ${anchor})"
        export "${tag_var}=${resolved}"
    fi
done
echo

for entry in "${COMPONENTS[@]}"; do
    org="${entry%%|*}"
    rest="${entry#*|}"
    tag_var="${rest%%|*}"
    collect_repo "${org}" "${!tag_var}"
done

echo
echo "Collected image lists in ${IMAGES_DIR}:"
ls -1 "${IMAGES_DIR}"
echo
echo "Collected chart lists in ${CHARTS_DIR}:"
ls -1 "${CHARTS_DIR}"
