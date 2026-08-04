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

package collect

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type layout struct {
	root    string
	tag     string
	out     string
	images  string
	charts  string
	scripts string
}

func newLayout() (*layout, error) {
	tag := os.Getenv("APPSCODE_CLOUD_TAG")
	if tag == "" {
		return nil, errors.New("APPSCODE_CLOUD_TAG must be set (names the output dir)")
	}

	root := os.Getenv("REPO_ROOT")
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		root = wd
	}

	out := filepath.Join(root, tag)
	return &layout{
		root:    root,
		tag:     tag,
		out:     out,
		images:  filepath.Join(out, "images"),
		charts:  filepath.Join(out, "charts"),
		scripts: filepath.Join(out, "scripts"),
	}, nil
}

// chartVersion resolves the version appscode-cloud/installer pins for a chart by
// scanning the collected catalog chart lists, so it stays in sync with the
// release without a hardcoded version. Files are scanned in glob order and the
// first match wins, matching what the shell pipeline did.
func chartVersion(chartsDir, chart string) (string, error) {
	re := regexp.MustCompile(`appscode-charts/` + regexp.QuoteMeta(chart) + `:([^"\s]+)`)

	files, err := filepath.Glob(filepath.Join(chartsDir, "*.yaml"))
	if err != nil {
		return "", err
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		if m := re.FindSubmatch(data); m != nil {
			return string(m[1]), nil
		}
	}
	return "", nil
}

func readList(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var refs []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if ref, ok := strings.CutPrefix(strings.TrimSpace(line), "- "); ok {
			refs = append(refs, strings.TrimSpace(ref))
		}
	}
	return refs, nil
}

func writeList(path string, refs []string) error {
	var sb strings.Builder
	for _, ref := range refs {
		sb.WriteString("- ")
		sb.WriteString(ref)
		sb.WriteString("\n")
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func sortUnique(refs []string) []string {
	slices.Sort(refs)
	return slices.Compact(refs)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyDirContents(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(srcDir, e.Name()), filepath.Join(dstDir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func listDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		fmt.Println(e.Name())
	}
	return nil
}
