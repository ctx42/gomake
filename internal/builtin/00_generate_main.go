// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx42/ring/pkg/ring"
	"golang.org/x/mod/modfile"

	"github.com/ctx42/gomake/internal/builtin"
	"github.com/ctx42/gomake/internal/cli"
	"github.com/ctx42/gomake/internal/parser"
)

// main is called by `go generate` in builtin.go. It reads [cli.TargetsFile]
// from the module root and regenerates built-in target code. Regenerate after
// changing [cli.TargetsFile].
func main() {
	modRoot, err := findGomakeRoot()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg, err := cli.LoadExternalTargets(
		context.Background(),
		ring.New(),
		filepath.Join(modRoot, cli.TargetsFile),
	)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	entries := cfg.Imports()
	imports := make([]parser.Import, 0, len(entries))
	for _, ent := range entries {
		imports = append(imports, parser.Import{
			Path:      ent.Path,
			Namespace: ent.Namespace,
		})
	}
	if err = builtin.GenImports(imports); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// findGomakeRoot walks up from the current directory until it finds the go.mod
// that declares module github.com/ctx42/gomake.
func findGomakeRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	start := dir
	for {
		gomod := filepath.Join(dir, "go.mod")
		var data []byte
		data, err = os.ReadFile(gomod)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("read %s: %w", gomod, err)
		}
		if modfile.ModulePath(data) == "github.com/ctx42/gomake" {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf(
		"could not find gomake module root from %s", start,
	)
}
