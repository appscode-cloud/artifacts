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

// Collect the kluster-manager images that live inside CR specs instead of pod
// specs, which is why image-packer's catalog for kluster-manager/installer does
// not see them:
//
//   - cluster-manager-hub/templates/clustermanager.cr.yaml carries the OCM
//     control-plane images as ClusterManager *ImagePullSpec fields.
//   - fluxcd-manager/templates/ocm/addon/fluxcd_config.yaml carries the flux
//     images as FluxCDConfig image fields.
//
// Chart versions are resolved from the chart lists collected by from-orgs
// (<APPSCODE_CLOUD_TAG>/charts/*.yaml), so this stays in sync with the release.
//
// Must run AFTER from-orgs (needs charts/ for versions and images/ to exist).

package collect

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const fluxAddonValuesURL = "https://raw.githubusercontent.com/kluster-manager/fluxcd-addon/%s/pkg/manager/agent-manifests/flux2/values.yaml"

var (
	imagePullSpecRE = regexp.MustCompile(`ImagePullSpec:[^\S\n]*"?([^"\s]+)`)
	appVersionRE    = regexp.MustCompile(`(?m)^appVersion:\s*"?([^"\s]+)`)

	// Shallow scan of a values-shaped document: a bare `key:` opens a section and
	// the image/tag fields nested under it belong to that section.
	sectionRE  = regexp.MustCompile(`^ {0,2}([A-Za-z][A-Za-z0-9]*):\s*$`)
	imageTagRE = regexp.MustCompile(`^\s+(image|tag):\s*"?([^"\s]+)`)
)

var crCharts = []struct {
	name   string // == OCI chart name == output basename
	images func(version string) ([]string, error)
}{
	{name: "cluster-manager-hub", images: clusterManagerHubImages},
	{name: "fluxcd-manager", images: fluxcdManagerImages},
}

func KlusterManager() error {
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

	for _, c := range crCharts {
		ver, err := chartVersion(l.charts, c.name)
		if err != nil {
			return err
		}
		if ver == "" {
			return fmt.Errorf("could not resolve version for %s from %s/*.yaml", c.name, l.charts)
		}

		fmt.Printf("==> %s @ %s\n", c.name, ver)
		refs, err := c.images(ver)
		if err != nil {
			return err
		}

		out := filepath.Join(l.images, c.name+".yaml")
		if err := writeList(out, refs); err != nil {
			return err
		}
		fmt.Printf("--> wrote %s (%d images)\n", out, len(refs))
	}
	return nil
}

func clusterManagerHubImages(version string) ([]string, error) {
	render, err := renderTemplate("cluster-manager-hub", version, "templates/clustermanager.cr.yaml")
	if err != nil {
		return nil, err
	}

	var refs []string
	for _, m := range imagePullSpecRE.FindAllStringSubmatch(render, -1) {
		refs = append(refs, m[1])
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("no ImagePullSpec images in cluster-manager-hub@%s clustermanager.cr.yaml", version)
	}
	return sortUnique(refs), nil
}

// FluxCDConfig overrides the flux image repos but not their tags: the tags stay
// at the defaults of the flux2 chart embedded in the fluxcd-addon binary. Join
// the repo from the rendered CR with the tag from the addon's values.yaml at the
// chart's appVersion, otherwise the refs are untagged and not mirrorable.
func fluxcdManagerImages(version string) ([]string, error) {
	render, err := renderTemplate("fluxcd-manager", version, "templates/ocm/addon/fluxcd_config.yaml")
	if err != nil {
		return nil, err
	}
	repos, _ := parseImageTags(render)
	if len(repos) == 0 {
		return nil, fmt.Errorf("no images in fluxcd-manager@%s fluxcd_config.yaml", version)
	}

	appVersion, err := chartAppVersion("fluxcd-manager", version)
	if err != nil {
		return nil, err
	}
	fmt.Printf("--> flux image tags from kluster-manager/fluxcd-addon @ %s\n", appVersion)

	values, err := httpGet(fmt.Sprintf(fluxAddonValuesURL, appVersion))
	if err != nil {
		return nil, err
	}
	_, tags := parseImageTags(values)

	var refs []string
	for section, repo := range repos {
		tag, ok := tags[section]
		if !ok {
			return nil, fmt.Errorf("no tag for %s (%s) in fluxcd-addon@%s flux2 values.yaml", section, repo, appVersion)
		}
		refs = append(refs, repo+":"+tag)
	}
	return sortUnique(refs), nil
}

func renderTemplate(chart, version, tmpl string) (string, error) {
	return output("", "helm", "template", chart, ociPrefix+"/"+chart,
		"--version", version, "-n", chart, "--show-only", tmpl)
}

func chartAppVersion(chart, version string) (string, error) {
	out, err := output("", "helm", "show", "chart", ociPrefix+"/"+chart, "--version", version)
	if err != nil {
		return "", err
	}
	m := appVersionRE.FindStringSubmatch(out)
	if m == nil {
		return "", fmt.Errorf("could not resolve appVersion for %s@%s", chart, version)
	}
	return m[1], nil
}

// parseImageTags maps section name -> image and section name -> tag. Deeper
// blocks under a section (resources, serviceAccount, ...) also read as sections
// here, so the first value per section wins: in both documents a section's own
// image/tag precede its nested blocks.
func parseImageTags(doc string) (images, tags map[string]string) {
	images, tags = map[string]string{}, map[string]string{}

	section := ""
	for line := range strings.SplitSeq(doc, "\n") {
		if m := sectionRE.FindStringSubmatch(line); m != nil {
			section = m[1]
			continue
		}
		m := imageTagRE.FindStringSubmatch(line)
		if m == nil || section == "" {
			continue
		}
		dst := images
		if m[1] == "tag" {
			dst = tags
		}
		if _, ok := dst[section]; !ok {
			dst[section] = m[2]
		}
	}
	return images, tags
}
