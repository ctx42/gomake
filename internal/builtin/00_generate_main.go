// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx42/ring/pkg/ring"

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
		f, err2 := os.Open(gomod)
		if err2 != nil {
			if !errors.Is(err2, os.ErrNotExist) {
				return "", fmt.Errorf("open %s: %w", gomod, err2)
			}
		} else {
			mod, readErr := readModuleLine(f)
			_ = f.Close()
			if readErr != nil {
				return "", fmt.Errorf("read %s: %w", gomod, readErr)
			}
			if mod == "github.com/ctx42/gomake" {
				return dir, nil
			}
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

// readModuleLine returns the module path declared on the first "module" line
// of f, or empty string when no such line is found. A trailing // comment
// is not part of the path.
func readModuleLine(f *os.File) (string, error) {
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		rest, ok := strings.CutPrefix(line, "module")
		if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		return moduleDirective(rest), nil
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("scan: %w", err)
	}
	return "", nil
}

// moduleDirective drops a trailing // comment and surrounding quotes from a
// module directive value.
func moduleDirective(rest string) string {
	rest = strings.TrimSpace(rest)
	inQuote := false
	escaped := false
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' || c == '`' {
				inQuote = false
			}
			continue
		}
		if c == '"' || c == '`' {
			inQuote = true
			continue
		}
		if c == '/' && i+1 < len(rest) && rest[i+1] == '/' {
			rest = rest[:i]
			break
		}
	}
	return strings.Trim(strings.TrimSpace(rest), "\"`")
}
