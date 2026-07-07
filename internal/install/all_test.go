// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package install

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ctx42/testing/pkg/tester"
)

// goListDir returns the single trimmed line printed by `go list` with args,
// failing the test on error.
func goListDir(t tester.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"list"}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// copyTree recursively copies src into dst, skipping the .git and dist trees.
func copyTree(t tester.T, src, dst string) {
	t.Helper()
	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || rel == "dist" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	}
	if err := filepath.WalkDir(src, walk); err != nil {
		t.Fatalf("copy tree: %v", err)
	}
}

// goModEdit runs `go mod edit` with args in dir, failing the test on error.
func goModEdit(t tester.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", append([]string{"mod", "edit"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod edit %v: %v: %s", args, err, out)
	}
}

// runBin runs bin with args in a fresh empty directory and returns its combined
// output, failing the test on a non-zero exit.
func runBin(t tester.T, bin string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s %v: %v: %s", bin, args, err, out)
	}
	return string(out)
}
