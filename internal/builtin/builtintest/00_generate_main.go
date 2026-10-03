// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx42/gomake/internal/builtin"
)

// main generates the builtintest targets. go generate in builtintest.go
// runs it. ImpPath values are rewritten to paths relative to the module
// root so the committed files do not embed a machine-specific directory.
func main() {
	impSpecs := []string{"github.com/ctx42/gomake/testdata/imports/pkg2"}
	err := builtin.GenMain(
		impSpecs,
		builtin.WithGenName("builtintest"),
		builtin.WithoutGenEmptySrc,
	)
	if err != nil {
		fail(err)
	}
	root, err := moduleRoot()
	if err != nil {
		fail(err)
	}
	names := []string{
		"targets.go",
		filepath.Join("data", "targets_main.go_"),
	}
	for _, name := range names {
		if err = relativizeImpPath(name, root); err != nil {
			fail(err)
		}
	}
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// moduleRoot returns the module directory containing the working directory.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	start := dir
	for {
		_, err = os.Stat(filepath.Join(dir, "go.mod"))
		if err == nil {
			return dir, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("stat go.mod: %w", err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", start)
		}
		dir = parent
	}
}

// relativizeImpPath rewrites absolute ImpPath values under root to
// module-relative paths. A path equal to root becomes ".".
func relativizeImpPath(name, root string) error {
	src, err := os.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}
	info, err := os.Stat(name)
	if err != nil {
		return fmt.Errorf("stat %s: %w", name, err)
	}
	under := []byte("ImpPath:     \"" + root + "/")
	exact := []byte("ImpPath:     \"" + root + "\"")
	if !bytes.Contains(src, under) && !bytes.Contains(src, exact) {
		format := "%s: no ImpPath under %s"
		return fmt.Errorf(format, name, root)
	}
	out := bytes.ReplaceAll(src, under, []byte("ImpPath:     \""))
	out = bytes.ReplaceAll(out, exact, []byte("ImpPath:     \".\""))
	if err = os.WriteFile(name, out, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}
