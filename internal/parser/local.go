// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"bufio"
	"bytes"
	"errors"
	"go/build"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/pkg/gomake"
)

// errNoModule is returned when go.mod has no "module" directive. It only
// triggers the `go list` fallback and is never surfaced to callers.
var errNoModule = errors.New("no module directive")

// newLocalPackage fills pkg with package facts derived in-process for a local
// absolute directory, avoiding the `go list` subprocess. The facts it resolves
// (package name, build-constrained Go files, and the enclosing module) depend
// only on the directory and its go.mod, never on the dependency graph, so the
// result matches `go list` regardless of replace directives, vendoring, or
// workspaces. It reports false when the directory cannot be resolved this way,
// in which case the caller falls back to `go list`.
func newLocalPackage(rng *ring.Ring, pkg *Package) bool {
	root, err := gomake.Root(pkg.ImpPath)
	if err != nil {
		return false
	}

	bp, err := importDir(rng, pkg.ImpPath)
	if err != nil {
		return false
	}

	modPath := filepath.Join(root, "go.mod")
	modSpec, err := readModulePath(modPath)
	if err != nil {
		return false
	}

	pkg.Name = bp.Name
	pkg.Files = bp.GoFiles
	pkg.Module = module{
		ImpSpec: modSpec,
		ImpPath: root,
		ModPath: modPath,
	}
	pkg.ImpSpec = importSpec(modSpec, root, pkg.ImpPath)
	return true
}

// importDir reads the Go package in dir using the same build tag, GOOS, and
// GOARCH selection that `go list` would apply, without spawning a subprocess.
func importDir(rng *ring.Ring, dir string) (*build.Package, error) {
	ctxt := build.Default
	if goos := rng.EnvGet("GOOS"); goos != "" {
		ctxt.GOOS = goos
	}
	if goarch := rng.EnvGet("GOARCH"); goarch != "" {
		ctxt.GOARCH = goarch
	}
	// Start from Default tags, then GOFLAGS -tags, then meta build tag.
	tags := append([]string{}, ctxt.BuildTags...)
	tags = append(tags, goFlagsTags(rng.EnvGet("GOFLAGS"))...)
	if tag := GetBuildTag(rng); tag != "" {
		tags = append(tags, tag)
	}
	ctxt.BuildTags = tags
	return ctxt.ImportDir(dir, 0)
}

// goFlagsTags extracts build tags from a GOFLAGS value (e.g. "-tags=a,b"
// or "-tags a,b"). Other flags are ignored.
func goFlagsTags(goflags string) []string {
	if goflags == "" {
		return nil
	}
	fields := strings.Fields(goflags)
	var out []string
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		switch {
		case strings.HasPrefix(f, "-tags="):
			out = append(out, splitTags(strings.TrimPrefix(f, "-tags="))...)
		case f == "-tags" || f == "--tags":
			if i+1 < len(fields) {
				i++
				out = append(out, splitTags(fields[i])...)
			}
		}
	}
	return out
}

// splitTags splits a comma-separated tags list into non-empty tags.
func splitTags(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// importSpec returns the import path of the package at dir given its module
// path modSpec rooted at root. It returns modSpec when dir is the module root.
func importSpec(modSpec, root, dir string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." || rel == "" {
		return modSpec
	}
	return modSpec + "/" + filepath.ToSlash(rel)
}

// readModulePath returns the module path declared on the "module" line of the
// go.mod file at path. It returns an error when the file cannot be read or has
// no module directive.
func readModulePath(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	scn := bufio.NewScanner(bytes.NewReader(data))
	for scn.Scan() {
		line := strings.TrimSpace(scn.Text())
		rest, ok := strings.CutPrefix(line, "module")
		if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		rest = strings.TrimSpace(rest)
		if i := strings.Index(rest, "//"); i >= 0 {
			rest = strings.TrimSpace(rest[:i])
		}
		if rest = strings.Trim(rest, "\"`"); rest != "" {
			return rest, nil
		}
	}
	return "", errNoModule
}
