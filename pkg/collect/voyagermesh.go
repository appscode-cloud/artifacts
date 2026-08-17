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

// Collect the envoy image, which no pod spec in voyagermesh/installer references,
// so image-packer's catalog for that repo does not see it.
//
// It is pinned by appscode-cloud/installer instead: charts/service-gateway
// renders envoy.image/envoy.tag into an EnvoyProxy CR
// (templates/gateway/gwclass.yaml), and the gateway deployment from the
// voyager-gateway subchart -- voyagermesh's chart, vendored into service-gateway
// -- reconciles that CR into the envoy Deployment + Service. The image is
// voyagermesh's, so it is merged into images/voyagermesh.yaml.
//
// The image is read off the rendered CR rather than joined from the raw values,
// so it stays correct if the chart templates the ref differently (a
// provisionerType of DaemonSet writes envoyDaemonSet.container.image instead).
//
// The chart version is resolved from the chart lists collected by from-orgs
// (<APPSCODE_CLOUD_TAG>/charts/*.yaml), so this stays in sync with the release.
//
// Must run AFTER from-orgs (needs charts/ for the version and images/voyagermesh.yaml).

package collect

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	serviceGatewayChart    = "service-gateway"
	serviceGatewayTemplate = "templates/gateway/gwclass.yaml"
)

func Voyagermesh() error {
	l, err := newLayout()
	if err != nil {
		return err
	}

	if _, err := os.Stat(l.charts); err != nil {
		return fmt.Errorf("%s not found; run from-orgs first", l.charts)
	}
	out := filepath.Join(l.images, "voyagermesh.yaml")
	existing, err := readList(out)
	if err != nil {
		return fmt.Errorf("%s not found; run from-orgs first: %w", out, err)
	}

	ver, err := chartVersion(l.charts, serviceGatewayChart)
	if err != nil {
		return err
	}
	if ver == "" {
		return fmt.Errorf("could not resolve version for %s from %s/*.yaml", serviceGatewayChart, l.charts)
	}

	fmt.Printf("==> %s @ %s\n", serviceGatewayChart, ver)
	render, err := renderTemplate(serviceGatewayChart, ver, serviceGatewayTemplate)
	if err != nil {
		return err
	}

	found := extractImages(render)
	if len(found) == 0 {
		return fmt.Errorf("no images in %s@%s %s", serviceGatewayChart, ver, serviceGatewayTemplate)
	}
	fmt.Printf("--> %d images\n", len(found))

	refs := sortUnique(existing)
	before := len(refs)
	refs = sortUnique(append(refs, found...))
	if err := writeList(out, refs); err != nil {
		return err
	}
	fmt.Printf("--> wrote %s (%d images, %d new from the EnvoyProxy CR)\n", out, len(refs), len(refs)-before)
	return nil
}
