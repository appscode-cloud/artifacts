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

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"go.bytebuilders.dev/artifacts/pkg/collect"
)

func main() {
	if len(os.Args) != 2 {
		usage()
	}

	var err error
	switch os.Args[1] {
	case "from-orgs":
		err = collect.FromOrgs()
	case "externals":
		err = collect.Externals()
	case "kluster-manager":
		err = collect.KlusterManager()
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage: %s <command>

Commands:
  from-orgs         collect per-repo catalog image lists from the installer orgs
  externals         collect image lists for external charts with curated CI values
  kluster-manager   collect kluster-manager images embedded in CR specs

Required env var:
  APPSCODE_CLOUD_TAG   appscode-cloud/installer tag; names the output dir
`, filepath.Base(os.Args[0]))
	os.Exit(2)
}
