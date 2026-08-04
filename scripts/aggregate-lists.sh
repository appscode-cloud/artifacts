#!/bin/bash

set -eou pipefail

# Aggregate the per-file lists under images/ and charts/ into single grouped
# files (all-images.yaml, all-charts.yaml). Each source file becomes a section
# headed by "# <filename-stem>" followed by its entries.
#
# Usage (from the branch root):
#   ./scripts/aggregate-lists.sh [base-dir]
# base-dir defaults to the current directory and must contain images/ and charts/.

BASE="${1:-.}"

aggregate() {
    local dir="$1" out="$2"
    : >"${out}"
    local first=1
    for f in "${dir}"/*.yaml; do
        [ -e "${f}" ] || continue
        local name
        name="$(basename "${f}" .yaml)"
        [ "${first}" -eq 1 ] || echo >>"${out}"
        first=0
        echo "# ${name}" >>"${out}"
        grep -E '^- ' "${f}" >>"${out}" || true
    done
}

aggregate "${BASE}/images" "${BASE}/all-images.yaml"
aggregate "${BASE}/charts" "${BASE}/all-charts.yaml"

echo "wrote ${BASE}/all-images.yaml and ${BASE}/all-charts.yaml"
