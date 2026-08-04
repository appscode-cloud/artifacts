/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Community License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Community-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Collect image lists for individual OCI charts that carry curated CI values.
//
// For each chart it renders `helm template` with the values file under hack/ci/
// and writes the referenced images into <APPSCODE_CLOUD_TAG>/images/<name>.yaml.
// The chart version is resolved from the chart lists already collected by
// from-orgs (<APPSCODE_CLOUD_TAG>/charts/*.yaml), so it stays in sync with the
// release without a hardcoded version here.
//
// Must run AFTER from-orgs (needs charts/ for versions and images/ to exist).

package collect

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

type externalChart struct {
	name   string // == OCI repo basename == output basename
	values string // values file under hack/ci/
}

var externalCharts = []externalChart{
	{name: "kube-prometheus-stack", values: "prometheus-stack-ci-values.yaml"},
	{name: "cert-manager", values: "cert-manager-ci-values.yaml"},
	{name: "flux2", values: "flux2-ci-values.yaml"},
	{name: "keda", values: "keda-ci-values.yaml"},
	{name: "keda-add-ons-http", values: "keda-add-ons-http-ci-values.yaml"},
	{name: "snapshot-controller", values: "snapshot-controller-ci-values.yaml"},
}

const ociPrefix = "oci://ghcr.io/appscode-charts"

// Image refs in a rendered manifest: both `image:` fields and container args of
// the form --flag=<registry>/... (config-reloader, thanos, acme solver),
// restricted to the registries these charts pull from.
//
// The gap after `image:` matches horizontal whitespace only ([^\S\n]); letting it
// cross a newline would make a bare `image:` key (keda has one) swallow the token
// on the following line.
var (
	imageFieldRE = regexp.MustCompile(`image:[^\S\n]*"?([^"\s]+)`)
	imageArgRE   = regexp.MustCompile(`--[a-z0-9-]+=((?:quay\.io|registry\.k8s\.io|docker\.io|ghcr\.io)[^"\s]+)`)
)

func Externals() error {
	l, err := newLayout()
	if err != nil {
		return err
	}

	if _, err := os.Stat(l.charts); err != nil {
		return fmt.Errorf("%s not found; run from-orgs first", l.charts)
	}
	if err := os.MkdirAll(l.images, 0o755); err != nil {
		return err
	}

	valuesDir := filepath.Join(l.root, "hack", "ci")
	for _, c := range externalCharts {
		valuesPath := filepath.Join(valuesDir, c.values)
		if _, err := os.Stat(valuesPath); err != nil {
			return fmt.Errorf("values file %s not found for %s", valuesPath, c.name)
		}

		ver, err := chartVersion(l.charts, c.name)
		if err != nil {
			return err
		}
		if ver == "" {
			return fmt.Errorf("could not resolve version for %s from %s/*.yaml", c.name, l.charts)
		}

		fmt.Printf("==> %s @ %s\n", c.name, ver)
		render, err := output("", "helm", "template", c.name, ociPrefix+"/"+c.name,
			"--version", ver, "-n", c.name, "-f", valuesPath)
		if err != nil {
			return err
		}

		refs := extractImages(render)
		out := filepath.Join(l.images, c.name+".yaml")
		if err := writeList(out, refs); err != nil {
			return err
		}
		fmt.Printf("--> wrote %s (%d images)\n", out, len(refs))
	}
	return nil
}

func extractImages(render string) []string {
	var refs []string
	for _, m := range imageFieldRE.FindAllStringSubmatch(render, -1) {
		refs = append(refs, m[1])
	}
	for _, m := range imageArgRE.FindAllStringSubmatch(render, -1) {
		refs = append(refs, m[1])
	}
	return sortUnique(refs)
}
