// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build ignore

// Command 00_generate_main writes the builtintest target sources.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx42/gomake/internal/builtin"
)

// main generates the builtintest targets. go generate in builtintest.go
// runs it. ImpPath values are relative to the module root so the committed
// files do not embed a machine-specific directory.
func main() {
	root, err := moduleRoot()
	if err != nil {
		fail(err)
	}
	impSpecs := []string{"github.com/ctx42/gomake/testdata/imports/pkg2"}
	err = builtin.GenMain(
		impSpecs,
		builtin.WithGenName("builtintest"),
		builtin.WithGenImpPathRoot(root),
		builtin.WithoutGenEmptySrc,
	)
	if err != nil {
		fail(err)
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
