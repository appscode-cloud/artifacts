# artifacts

Collects the image and chart lists for an ACE release into a single per-release
directory, and publishes it as an orphan branch named after the release tag.

## What it does

On `workflow_dispatch` (`.github/workflows/collect-images.yml`) it takes a single
git tag — `appscode_cloud_tag` — and runs, in order:

1. `hack/scripts/collect-from-orgs.sh` — clone `appscode-cloud/installer` at that
   tag, regenerate its catalog via its own `hack/scripts/update-catalog.sh` (which
   drives `image-packer`), copy `catalog/imagelist.yaml` to
   `images/appscode-cloud.yaml` and the catalog chart lists into `charts/`. Then
   derive each component installer's tag from those chart lists (see
   [Anchor charts](#anchor-charts)) and do the same clone + catalog + copy for
   each one.
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

| repo | tag | output |
|------|-----|--------|
| `appscode-cloud/installer` | `APPSCODE_CLOUD_TAG` (the input) | `images/appscode-cloud.yaml` + `charts/*.yaml` |
| `kubedb/installer` | derived | `images/kubedb.yaml` |
| `kubestash/installer` | derived | `images/kubestash.yaml` |
| `kubeops/installer` | derived | `images/kubeops.yaml` |
| `kluster-manager/installer` | derived | `images/kluster-manager.yaml` |
| `open-viz/installer` | derived | `images/open-viz.yaml` |
| `opnpulse/installer` | derived | `images/opnpulse.yaml` |

### Anchor charts

`appscode-cloud/installer` pins the version of every chart an ACE release deploys
(in `charts/opscenter-features/values.yaml`, mirrored into the `catalog/*.yaml`
lists this repo copies to `charts/`). Each component repo owns one **anchor chart**
there, and its pinned version is that repo's tag:

| repo | anchor chart | tag env var |
|------|--------------|-------------|
| `kubedb/installer` | `kubedb` | `KUBEDB_TAG` |
| `kubestash/installer` | `kubestash` | `KUBESTASH_TAG` |
| `kubeops/installer` | `kube-ui-server` | `KUBEOPS_TAG` |
| `kluster-manager/installer` | `cluster-profile-manager` | `KLUSTER_MANAGER_TAG` |
| `open-viz/installer` | `monitoring-operator` | `OPEN_VIZ_TAG` |
| `opnpulse/installer` | `appscode-otel-stack` | `OPNPULSE_TAG` |

A repo's other charts are pinned on their own cadence and are **not** valid tag
sources — nor is the repo's latest tag. Choosing a component tag by hand collects
images for chart versions the release does not deploy, so the mirrored list is
missing the images ACE actually pulls and an air-gapped install fails.

Each derived tag can still be overridden by exporting its env var (e.g. to collect
an rc ahead of an ACE release); every override is logged as a `WARNING:` line. If
an anchor chart is not found in `charts/*.yaml` — a rename in a newer release — the
run fails rather than falling back to a guess; update `COMPONENTS` in
`hack/scripts/collect-from-orgs.sh`.

External OCI charts from `ghcr.io/appscode-charts` (`hack/scripts/collect-externals.sh`):

| chart | CI values | output |
|-------|-----------|--------|
| `kube-prometheus-stack` | `hack/ci/prometheus-stack-ci-values.yaml` | `images/kube-prometheus-stack.yaml` |
| `cert-manager` | `hack/ci/cert-manager-ci-values.yaml` | `images/cert-manager.yaml` |
| `flux2` | `hack/ci/flux2-ci-values.yaml` | `images/flux2.yaml` |
| `keda` | `hack/ci/keda-ci-values.yaml` | `images/keda.yaml` |
| `keda-add-ons-http` | `hack/ci/keda-add-ons-http-ci-values.yaml` | `images/keda-add-ons-http.yaml` |
| `snapshot-controller` | `hack/ci/snapshot-controller-ci-values.yaml` | `images/snapshot-controller.yaml` |

## Run locally (on a VM)

```sh
export APPSCODE_CLOUD_TAG=v2026.7.22
make collect
```

Output lands in `./$APPSCODE_CLOUD_TAG/`. To also produce the grouped lists:

```sh
bash bare-scripts/aggregate-lists.sh "$APPSCODE_CLOUD_TAG"
```
