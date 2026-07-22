# images

Collects the catalog image lists of the AppsCode installer repos into a single
per-release directory.

## What it does

On `workflow_dispatch` (`.github/workflows/collect-images.yml`) it takes one git
tag per installer repo, and for each repo:

1. clones the repo at that tag,
2. regenerates the catalog via the repo's own `make update-catalog`,
3. copies `catalog/imagelist.yaml` into `<appscode_cloud_tag>/<org>-images.yaml`.

The **appscode-cloud tag names the output directory**.

## Repos collected

| repo | output file |
|------|-------------|
| `appscode-cloud/installer` | `appscode-cloud-images.yaml` |
| `kubedb/installer` | `kubedb-images.yaml` |
| `kubestash/installer` | `kubestash-images.yaml` |
| `kubeops/installer` | `kubeops-images.yaml` |
| `kluster-manager/installer` | `kluster-manager-images.yaml` |
| `open-viz/installer` | `open-viz-images.yaml` |

## Run locally (on a VM)

```sh
export APPSCODE_CLOUD_TAG=v2026.7.22
export KUBEDB_TAG=...
export KUBESTASH_TAG=...
export KUBEOPS_TAG=...
export KLUSTER_MANAGER_TAG=...
export OPEN_VIZ_TAG=...
make collect
```

Output lands in `./$APPSCODE_CLOUD_TAG/`.
