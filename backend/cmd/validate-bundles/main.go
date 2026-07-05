// Command validate-bundles is the content-CI half of the "validation happens
// twice" rule (docs/history/challenges-backend-architecture.md §4): it lints
// every challenge bundle directory pre-merge with the SAME Go validator the
// backend runs at watch time, so content that passes CI cannot be quarantined
// for schema reasons at runtime.
//
// Usage:
//
//	validate-bundles [-root <dir>]
//
// where <dir> contains one directory per challenge:
//
//	<dir>/<id>/challenge.yaml
//	<dir>/<id>/seed/*.yaml
//
// Exit status is non-zero if any bundle fails validation. The kubeconform +
// dry-run-apply "must-apply gate" is separate CI wiring (it needs a throwaway
// vcluster); this CLI is the schema/structure gate.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jeremy-misola/kubesandbox/backend/internal/content"
)

func main() {
	root := flag.String("root", "kubesandbox-charts/kubesandbox/charts/kubesandbox-challenges/challenges",
		"directory containing one subdirectory per challenge bundle")
	flag.Parse()

	entries, err := os.ReadDir(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *root, err)
		os.Exit(2)
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	if len(dirs) == 0 {
		fmt.Fprintf(os.Stderr, "no bundle directories under %s\n", *root)
		os.Exit(2)
	}

	failed := 0
	ids := map[string]string{}
	for _, dir := range dirs {
		errs := validateOne(*root, dir, ids)
		if len(errs) > 0 {
			failed++
			fmt.Printf("FAIL %s\n", dir)
			for _, e := range errs {
				fmt.Printf("     - %s\n", e)
			}
			continue
		}
		fmt.Printf("ok   %s\n", dir)
	}

	fmt.Printf("\n%d bundle(s), %d failed\n", len(dirs), failed)
	if failed > 0 {
		os.Exit(1)
	}
}

// validateOne loads and lints one bundle directory. ids tracks id uniqueness
// across the whole content set.
func validateOne(root, dir string, ids map[string]string) []string {
	challengeYAML, err := os.ReadFile(filepath.Join(root, dir, "challenge.yaml"))
	if err != nil {
		return []string{fmt.Sprintf("challenge.yaml: %v", err)}
	}

	seed := map[string][]byte{}
	seedDir := filepath.Join(root, dir, "seed")
	seedEntries, err := os.ReadDir(seedDir)
	if err != nil {
		return []string{fmt.Sprintf("seed/: %v", err)}
	}
	for _, se := range seedEntries {
		if se.IsDir() {
			return []string{fmt.Sprintf("seed/%s: nested directories are not supported (ConfigMap keys are flat)", se.Name())}
		}
		data, err := os.ReadFile(filepath.Join(seedDir, se.Name()))
		if err != nil {
			return []string{fmt.Sprintf("seed/%s: %v", se.Name(), err)}
		}
		seed[se.Name()] = data
	}

	b, errs := content.ValidateDir(dir, challengeYAML, seed)
	if len(errs) > 0 {
		return errs
	}
	if prev, dup := ids[b.ID]; dup {
		return []string{fmt.Sprintf("duplicate id %q (also used by %s)", b.ID, prev)}
	}
	ids[b.ID] = dir
	return nil
}
