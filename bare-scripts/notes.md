1) Mirror images to your own registry, direct network path	: Use `copy-images.sh`
2) Air-gapped: move images via tarball:	Use `export-images.sh` then `import-images.sh`

## Grouped image/chart lists

`images/` and `charts/` hold one list per source (`<org>.yaml`, `ace.yaml`, ...).
`aggregate-lists.sh` merges them into two grouped files at the branch root:

- `all-images.yaml` — every `images/*.yaml`, each under a `# <org>` header.
- `all-charts.yaml` — every `charts/*.yaml`, each under a `# <name>` header.

```
# kubedb
- img1
- img2

# kubestash
- img1
```
