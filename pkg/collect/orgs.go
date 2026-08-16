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

// Collect per-repo catalog image lists into <APPSCODE_CLOUD_TAG>/images/<org>.yaml.
//
// For every installer repo it clones the given tag, regenerates the catalog via
// that repo's own hack/scripts/update-catalog.sh (which drives the image-packer
// binary), and copies catalog/imagelist.yaml into the output directory.
//
// For appscode-cloud/installer only, it also copies the catalog chart lists
// (ace.yaml, editor-charts.yaml, feature-charts.yaml, reusable-ui-charts.yaml)
// into <APPSCODE_CLOUD_TAG>/charts/, and catalog/feature-chart-images.yaml into
// images/feature-charts.yaml. That last one carries the images of the feature
// charts that belong to no installer repo, so nothing else here publishes them.
//
// APPSCODE_CLOUD_TAG is the only input; it names the output dir and the release
// being collected. Every component repo's tag is derived from it: each
// component owns one anchor chart whose version appscode-cloud/installer pins in
// its catalog lists, and that pinned version is the component's tag. Picking the
// component tags by hand collects images for chart versions this ACE release does
// not deploy, which silently breaks an air-gapped mirror.
//
// Each derived tag can still be overridden by exporting its env var (KUBEDB_TAG,
// KUBESTASH_TAG, KUBEVAULT_TAG, KUBEOPS_TAG, KLUSTER_MANAGER_TAG, OPEN_VIZ_TAG,
// OPNPULSE_TAG, STASH_TAG, VOYAGER_TAG, VIRTUAL_SECRETS_TAG),
// e.g. to collect an rc ahead of an ACE release; each override is logged.

package collect

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type component struct {
	org    string
	tagEnv string
	anchor string // anchor chart pinned by appscode-cloud/installer
}

var components = []component{
	{org: "kubedb", tagEnv: "KUBEDB_TAG", anchor: "kubedb"},
	{org: "kubestash", tagEnv: "KUBESTASH_TAG", anchor: "kubestash"},
	{org: "kubevault", tagEnv: "KUBEVAULT_TAG", anchor: "kubevault"},
	{org: "kubeops", tagEnv: "KUBEOPS_TAG", anchor: "kube-ui-server"},
	{org: "kluster-manager", tagEnv: "KLUSTER_MANAGER_TAG", anchor: "cluster-profile-manager"},
	{org: "open-viz", tagEnv: "OPEN_VIZ_TAG", anchor: "monitoring-operator"},
	{org: "opnpulse", tagEnv: "OPNPULSE_TAG", anchor: "appscode-otel-stack"},
	{org: "stashed", tagEnv: "STASH_TAG", anchor: "stash"},
	{org: "voyagermesh", tagEnv: "VOYAGER_TAG", anchor: "voyager"},
	{org: "virtual-secrets", tagEnv: "VIRTUAL_SECRETS_TAG", anchor: "virtual-secrets-server"},
}

// Chart lists copied from appscode-cloud/installer's catalog/ into charts/.
var chartFiles = []string{"ace.yaml", "editor-charts.yaml", "feature-charts.yaml", "reusable-ui-charts.yaml"}

// Image list copied from appscode-cloud/installer's catalog/ into images/.
const featureChartImages = "feature-chart-images.yaml"

var imagePackerVersionRE = regexp.MustCompile(`kmodules\.xyz/image-packer\s+(\S+)`)

// A pseudo-version (vX-YYYYMMDDhhmmss-<12-hex-commit>) resolves to its commit;
// a plain tag is used as-is.
var pseudoVersionRE = regexp.MustCompile(`-([0-9a-f]{12})$`)

func FromOrgs() error {
	l, err := newLayout()
	if err != nil {
		return err
	}

	workDir, err := os.MkdirTemp("", "collect-images-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	for _, dir := range []string{l.images, l.charts, l.scripts} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	// Ship the bare helper scripts (import/export/copy) and notes into the branch as-is.
	if err := copyDirContents(filepath.Join(l.root, "bare-scripts"), l.scripts); err != nil {
		return err
	}

	if err := installImagePacker(l.tag, workDir); err != nil {
		return err
	}

	// appscode-cloud/installer goes first: its catalog chart lists are what the
	// component tags are derived from.
	if err := collectRepo(l, workDir, "appscode-cloud", l.tag); err != nil {
		return err
	}

	for _, chart := range chartFiles {
		src := filepath.Join(workDir, "appscode-cloud", "catalog", chart)
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("%s not found for appscode-cloud/installer", src)
		}
		dst := filepath.Join(l.charts, chart)
		if err := copyFile(src, dst); err != nil {
			return err
		}
		fmt.Printf("--> wrote %s\n", dst)
	}

	// The images the feature charts deploy. They belong to no installer repo --
	// reloader, kyverno, longhorn, cert-manager, kube-prometheus-stack and the
	// rest -- so nothing else in this collection publishes them.
	src := filepath.Join(workDir, "appscode-cloud", "catalog", featureChartImages)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("%s not found for appscode-cloud/installer", src)
	}
	dst := filepath.Join(l.images, "feature-charts.yaml")
	if err := copyFile(src, dst); err != nil {
		return err
	}
	fmt.Printf("--> wrote %s\n", dst)

	fmt.Printf("\n==> deriving component tags from appscode-cloud/installer @ %s\n", l.tag)
	tags := make(map[string]string, len(components))
	for _, c := range components {
		resolved, err := chartVersion(l.charts, c.anchor)
		if err != nil {
			return err
		}
		if resolved == "" {
			return fmt.Errorf("anchor chart %s (%s/installer) not pinned in %s/*.yaml;\n"+
				"       it may have been renamed in %s — update components",
				c.anchor, c.org, l.charts, l.tag)
		}

		if given := os.Getenv(c.tagEnv); given != "" && given != resolved {
			fmt.Fprintf(os.Stderr, "WARNING: %s overridden: %s (from %s) -> %s\n", c.tagEnv, resolved, c.anchor, given)
			tags[c.org] = given
		} else {
			fmt.Printf("--> %s=%s (from %s)\n", c.tagEnv, resolved, c.anchor)
			tags[c.org] = resolved
		}
	}
	fmt.Println()

	for _, c := range components {
		if err := collectRepo(l, workDir, c.org, tags[c.org]); err != nil {
			return err
		}
	}

	fmt.Printf("\nCollected image lists in %s:\n", l.images)
	if err := listDir(l.images); err != nil {
		return err
	}
	fmt.Printf("\nCollected chart lists in %s:\n", l.charts)
	return listDir(l.charts)
}

// image-packer drives each repo's hack/scripts/update-catalog.sh. Install the
// version pinned by appscode-cloud/installer's go.mod at APPSCODE_CLOUD_TAG so it
// matches the release being collected. image-packer's go.mod carries replace
// directives, so `go install pkg@version` is rejected; instead build from source
// at the pinned ref, where its go.mod is the main module and replaces are honored.
func installImagePacker(tag, workDir string) error {
	fmt.Printf("==> resolving image-packer version from appscode-cloud/installer @ %s\n", tag)

	gomod, err := httpGet(fmt.Sprintf("https://raw.githubusercontent.com/appscode-cloud/installer/%s/go.mod", tag))
	if err != nil {
		return err
	}
	m := imagePackerVersionRE.FindStringSubmatch(gomod)
	if m == nil {
		return fmt.Errorf("could not resolve image-packer version from appscode-cloud/installer go.mod")
	}
	version := m[1]

	ref := version
	if pv := pseudoVersionRE.FindStringSubmatch(version); pv != nil {
		ref = pv[1]
	}

	gopath, err := output("", "go", "env", "GOPATH")
	if err != nil {
		return err
	}
	gobin := filepath.Join(strings.TrimSpace(gopath), "bin")
	if err := os.MkdirAll(gobin, 0o755); err != nil {
		return err
	}

	fmt.Printf("--> building kmodules.xyz/image-packer @ %s (from %s)\n", ref, version)
	src := filepath.Join(workDir, "image-packer")
	if err := run("", "git", "clone", "--filter=blob:none", "https://github.com/kmodules/image-packer.git", src); err != nil {
		return err
	}
	if err := run(src, "git", "checkout", "--quiet", ref); err != nil {
		return err
	}
	if err := run(src, "go", "build", "-o", filepath.Join(gobin, "image-packer"), "."); err != nil {
		return err
	}

	return os.Setenv("PATH", gobin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func collectRepo(l *layout, workDir, org, tag string) error {
	src := filepath.Join(workDir, org)

	fmt.Printf("==> %s/installer @ %s\n", org, tag)
	if err := run("", "git", "clone", "--depth", "1", "--branch", tag,
		fmt.Sprintf("https://github.com/%s/installer.git", org), src); err != nil {
		return err
	}

	fmt.Printf("--> update-catalog (%s)\n", org)
	if err := run(src, "./hack/scripts/update-catalog.sh"); err != nil {
		return err
	}

	imagelist := filepath.Join(src, "catalog", "imagelist.yaml")
	if _, err := os.Stat(imagelist); err != nil {
		return fmt.Errorf("%s not found after update-catalog for %s/installer", imagelist, org)
	}

	dst := filepath.Join(l.images, org+".yaml")
	if err := copyFile(imagelist, dst); err != nil {
		return err
	}
	fmt.Printf("--> wrote %s\n", dst)
	return nil
}
