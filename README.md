# artifacts

Collects the image and chart lists for an ACE release into a single per-release
directory, and publishes it as an orphan branch named after the release tag.

## What it does

On `workflow_dispatch` (`.github/workflows/collect-images.yml`) it takes a single
git tag — `appscode_cloud_tag` — and runs, in order:

1. `go run . from-orgs` — clone `appscode-cloud/installer` at that
   tag, regenerate its catalog via its own `hack/scripts/update-catalog.sh` (which
   drives `image-packer`), copy `catalog/imagelist.yaml` to
   `images/appscode-cloud.yaml`, `catalog/feature-chart-images.yaml` to
   `images/feature-charts.yaml`, and the catalog chart lists into `charts/`. Then
   derive each component installer's tag from those chart lists (see
   [Anchor charts](#anchor-charts)) and do the same clone + catalog + copy for
   each one.
2. `go run . kluster-manager` — `helm template` the two kluster-manager charts
   whose images sit inside CR specs (so `image-packer` does not see them) and merge
   them into `images/kluster-manager.yaml`. See
   [CR-embedded images](#cr-embedded-images).
3. `bare-scripts/aggregate-lists.sh` — merge `images/*.yaml` and `charts/*.yaml`
   into grouped `all-images.yaml` / `all-charts.yaml`, each source file becoming a
   `# <name>` section.
4. push the directory to an orphan branch named after the appscode-cloud tag,
   flattened to the branch root, with `bare-scripts/notes.md` as its `README.md`.

`image-packer` (`kmodules.xyz/image-packer`) is built from source by
`from-orgs` at the version pinned in `appscode-cloud/installer`'s
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
│   ├── kubevault.yaml
│   ├── kubeops.yaml
│   ├── kluster-manager.yaml
│   ├── open-viz.yaml
│   ├── opnpulse.yaml
│   ├── stashed.yaml
│   ├── voyagermesh.yaml
│   ├── virtual-secrets.yaml
│   └── feature-charts.yaml
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

Installer repos (`go run . from-orgs`) — each cloned at its own tag:

| repo | tag | output |
|------|-----|--------|
| `appscode-cloud/installer` | `APPSCODE_CLOUD_TAG` (the input) | `images/appscode-cloud.yaml` + `charts/*.yaml` |
| `kubedb/installer` | derived | `images/kubedb.yaml` |
| `kubestash/installer` | derived | `images/kubestash.yaml` |
| `kubevault/installer` | derived | `images/kubevault.yaml` |
| `kubeops/installer` | derived | `images/kubeops.yaml` |
| `kluster-manager/installer` | derived | `images/kluster-manager.yaml` |
| `open-viz/installer` | derived | `images/open-viz.yaml` |
| `opnpulse/installer` | derived | `images/opnpulse.yaml` |
| `stashed/installer` | derived | `images/stashed.yaml` |
| `voyagermesh/installer` | derived | `images/voyagermesh.yaml` |
| `virtual-secrets/installer` | derived | `images/virtual-secrets.yaml` |

### Anchor charts

`appscode-cloud/installer` pins the version of every chart an ACE release deploys
(in `charts/opscenter-features/values.yaml`, mirrored into the `catalog/*.yaml`
lists this repo copies to `charts/`). Each component repo owns one **anchor chart**
there, and its pinned version is that repo's tag:

| repo | anchor chart | tag env var |
|------|--------------|-------------|
| `kubedb/installer` | `kubedb` | `KUBEDB_TAG` |
| `kubestash/installer` | `kubestash` | `KUBESTASH_TAG` |
| `kubevault/installer` | `kubevault` | `KUBEVAULT_TAG` |
| `kubeops/installer` | `kube-ui-server` | `KUBEOPS_TAG` |
| `kluster-manager/installer` | `cluster-profile-manager` | `KLUSTER_MANAGER_TAG` |
| `open-viz/installer` | `monitoring-operator` | `OPEN_VIZ_TAG` |
| `opnpulse/installer` | `appscode-otel-stack` | `OPNPULSE_TAG` |
| `stashed/installer` | `stash` | `STASH_TAG` |
| `voyagermesh/installer` | `voyager` | `VOYAGER_TAG` |
| `virtual-secrets/installer` | `virtual-secrets-server` | `VIRTUAL_SECRETS_TAG` |

A repo's other charts are pinned on their own cadence and are **not** valid tag
sources — nor is the repo's latest tag. Choosing a component tag by hand collects
images for chart versions the release does not deploy, so the mirrored list is
missing the images ACE actually pulls and an air-gapped install fails.

Each derived tag can still be overridden by exporting its env var (e.g. to collect
an rc ahead of an ACE release); every override is logged as a `WARNING:` line. If
an anchor chart is not found in `charts/*.yaml` — a rename in a newer release — the
run fails rather than falling back to a guess; update `components` in
`pkg/collect/orgs.go`.

### Feature chart images

Every chart an ACE release deploys is pinned in `charts/feature-charts.yaml`, but
most of those charts' **images** are published by the installer repo that owns
them. The rest — `reloader`, `prometheus-adapter`, `kyverno`, `longhorn`,
`opencost`, `cert-manager`, `flux2`, `keda`, `kube-prometheus-stack`,
`snapshot-controller` and friends — belong to no installer repo.

`appscode-cloud/installer` renders those at their pinned version, using the
values the `Feature` itself carries, into `catalog/feature-chart-images.yaml`;
step 1 copies it to `images/feature-charts.yaml`. Which charts are skipped as
already-published is decided there, in `hack/scripts/update-catalog.sh`.

`pkg/collect/externals.go` and `hack/ci/*-ci-values.yaml` did this for six of
those charts from hand-maintained values. They are retained but no longer run:
the curated values had drifted from what ACE deploys (they built the flux2
kustomize and notification controllers, which the ACE `Feature` disables).

### CR-embedded images

Two kluster-manager charts declare images inside a CR spec rather than a pod spec,
so they never appear in `kluster-manager/installer`'s `catalog/imagelist.yaml`.
`go run . kluster-manager` renders just those templates (versions resolved from
`charts/*.yaml`, like the external charts) and extracts them:

| chart | template | images from |
|-------|----------|-------------|
| `cluster-manager-hub` | `templates/clustermanager.cr.yaml` | `ClusterManager` `*ImagePullSpec` fields |
| `fluxcd-manager` | `templates/ocm/addon/fluxcd_config.yaml` | `FluxCDConfig` `image` fields |

Both charts belong to kluster-manager, so their images are merged into the
`images/kluster-manager.yaml` written in step 1 (union, sorted and deduped) instead
of becoming their own `all-images.yaml` sections. The merge is a union, so the
command is safe to re-run.

`FluxCDConfig` overrides the flux image repositories but not their tags — those
stay at the defaults of the flux2 chart embedded in the `fluxcd-addon` binary. So
each repo is joined with its tag from
`kluster-manager/fluxcd-addon`'s `pkg/manager/agent-manifests/flux2/values.yaml`
at the chart's `appVersion`, matched by section name (`helmController`,
`sourceController`, …). A repo with no matching tag fails the run rather than
emitting an untagged, unmirrorable ref.

## Run locally (on a VM)

```sh
export APPSCODE_CLOUD_TAG=v2026.7.22
make collect
```

Output lands in `./$APPSCODE_CLOUD_TAG/`. To also produce the grouped lists:

```sh
bash bare-scripts/aggregate-lists.sh "$APPSCODE_CLOUD_TAG"
```
