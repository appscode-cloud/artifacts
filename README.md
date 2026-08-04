# artifacts

Collects the image and chart lists for an ACE release into a single per-release
directory, and publishes it as an orphan branch named after the release tag.

## What it does

On `workflow_dispatch` (`.github/workflows/collect-images.yml`) it takes one git
tag per installer repo and runs, in order:

1. `hack/scripts/collect-from-orgs.sh` — for each installer repo: clone it at the
   given tag, regenerate the catalog via that repo's own
   `hack/scripts/update-catalog.sh` (which drives `image-packer`), and copy
   `catalog/imagelist.yaml` to `images/<org>.yaml`. For
   `appscode-cloud/installer` only, the catalog chart lists are also copied into
   `charts/`.
2. `hack/scripts/collect-externals.sh` — for each external OCI chart with curated
   CI values under `hack/ci/`, `helm template` the chart and write the referenced
   images to `images/<chart>.yaml`. The chart version is resolved from the
   `charts/` lists collected in step 1, so it stays in sync with the release.
3. `bare-scripts/aggregate-lists.sh` — merge `images/*.yaml` and `charts/*.yaml`
   into grouped `all-images.yaml` / `all-charts.yaml`, each source file becoming a
   `# <name>` section.
4. push the directory to an orphan branch named after the appscode-cloud tag,
   flattened to the branch root, with `bare-scripts/notes.md` as its `README.md`.

`image-packer` (`kmodules.xyz/image-packer`) is built from source by
`collect-from-orgs.sh` at the version pinned in `appscode-cloud/installer`'s
`go.mod` for `APPSCODE_CLOUD_TAG`, so the tooling matches the release being
collected. A Go toolchain, `helm` (x-helm build), `yq` and `yqq` must be on `PATH`.

The **appscode-cloud tag names the output directory and the branch**.

## Output layout

```
<appscode_cloud_tag>/          # becomes the branch root
├── images/
│   ├── appscode-cloud.yaml
│   ├── kubedb.yaml
│   ├── kubestash.yaml
│   ├── kubeops.yaml
│   ├── kluster-manager.yaml
│   ├── open-viz.yaml
│   ├── opnpulse.yaml
│   ├── kube-prometheus-stack.yaml
│   ├── cert-manager.yaml
│   ├── flux2.yaml
│   ├── keda.yaml
│   ├── keda-add-ons-http.yaml
│   └── snapshot-controller.yaml
├── charts/
│   ├── ace.yaml
│   ├── editor-charts.yaml
│   ├── feature-charts.yaml
│   └── reusable-ui-charts.yaml
├── scripts/                   # copy of bare-scripts/ (mirror, export, import, aggregate)
├── all-images.yaml
├── all-charts.yaml
└── README.md                  # copy of bare-scripts/notes.md
```

## Sources collected

Installer repos (`hack/scripts/collect-from-orgs.sh`) — each cloned at its own tag:

| repo | tag env var | output |
|------|-------------|--------|
| `appscode-cloud/installer` | `APPSCODE_CLOUD_TAG` | `images/appscode-cloud.yaml` + `charts/*.yaml` |
| `kubedb/installer` | `KUBEDB_TAG` | `images/kubedb.yaml` |
| `kubestash/installer` | `KUBESTASH_TAG` | `images/kubestash.yaml` |
| `kubeops/installer` | `KUBEOPS_TAG` | `images/kubeops.yaml` |
| `kluster-manager/installer` | `KLUSTER_MANAGER_TAG` | `images/kluster-manager.yaml` |
| `open-viz/installer` | `OPEN_VIZ_TAG` | `images/open-viz.yaml` |
| `opnpulse/installer` | `OPNPULSE_TAG` | `images/opnpulse.yaml` |

External OCI charts from `ghcr.io/appscode-charts` (`hack/scripts/collect-externals.sh`):

| chart | CI values | output |
|-------|-----------|--------|
| `kube-prometheus-stack` | `hack/ci/prometheus-stack-ci-values.yaml` | `images/kube-prometheus-stack.yaml` |
| `cert-manager` | `hack/ci/cert-manager-ci-values.yaml` | `images/cert-manager.yaml` |
| `flux2` | `hack/ci/flux2-ci-values.yaml` | `images/flux2.yaml` |
| `keda` | `hack/ci/keda-ci-values.yaml` | `images/keda.yaml` |
| `keda-add-ons-http` | `hack/ci/keda-add-ons-http-ci-values.yaml` | `images/keda-add-ons-http.yaml` |
| `snapshot-controller` | `hack/ci/snapshot-controller-ci-values.yaml` | `images/snapshot-controller.yaml` |

## Default tags

`default-tags.env` holds one tag per repo. GitHub can't read a file to fill
`workflow_dispatch` defaults at runtime, so after editing it run:

```sh
make sync-defaults
```

That bakes the values into the workflow's `default:` fields (between the
`dispatch-defaults` markers) so they appear pre-filled in the "Run workflow" UI.
Commit the workflow change. `make collect` also reads `default-tags.env` for
local runs; explicit environment variables override it.

## Run locally (on a VM)

```sh
export APPSCODE_CLOUD_TAG=v2026.7.22
export KUBEDB_TAG=...
export KUBESTASH_TAG=...
export KUBEOPS_TAG=...
export KLUSTER_MANAGER_TAG=...
export OPEN_VIZ_TAG=...
export OPNPULSE_TAG=...
make collect
```

Output lands in `./$APPSCODE_CLOUD_TAG/`. To also produce the grouped lists:

```sh
bash bare-scripts/aggregate-lists.sh "$APPSCODE_CLOUD_TAG"
```
