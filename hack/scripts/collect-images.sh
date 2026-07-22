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

# Collect per-repo catalog image lists into <APPSCODE_CLOUD_TAG>/<org>-images.yaml.
#
# For every installer repo it clones the given tag, regenerates the catalog via
# that repo's own `make update-catalog`, and copies catalog/imagelist.yaml into
# the output directory.
#
# Required env vars (each is the git ref to checkout for that repo):
#   APPSCODE_CLOUD_TAG   -> appscode-cloud/installer   (also names the output dir)
#   KUBEDB_TAG           -> kubedb/installer
#   KUBESTASH_TAG        -> kubestash/installer
#   KUBEOPS_TAG          -> kubeops/installer
#   KLUSTER_MANAGER_TAG  -> kluster-manager/installer
#   OPEN_VIZ_TAG         -> open-viz/installer

set -eou pipefail

# repo org | tag env var | output file basename (== org)
REPOS=(
    "appscode-cloud|APPSCODE_CLOUD_TAG"
    "kubedb|KUBEDB_TAG"
    "kubestash|KUBESTASH_TAG"
    "kubeops|KUBEOPS_TAG"
    "kluster-manager|KLUSTER_MANAGER_TAG"
    "open-viz|OPEN_VIZ_TAG"
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

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

mkdir -p "${OUT_DIR}"

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

    echo "--> make update-catalog (${org})"
    make -C "${src}" update-catalog

    imagelist="${src}/catalog/imagelist.yaml"
    if [ ! -f "${imagelist}" ]; then
        echo "ERROR: ${imagelist} not found after update-catalog for ${org}/installer" >&2
        exit 1
    fi

    cp "${imagelist}" "${OUT_DIR}/${org}-images.yaml"
    echo "--> wrote ${OUT_DIR}/${org}-images.yaml"
done

echo
echo "Collected image lists in ${OUT_DIR}:"
ls -1 "${OUT_DIR}"
