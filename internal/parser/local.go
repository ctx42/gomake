// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// importDir reads the Go package in dir using the build tag, GOOS, and
// GOARCH from rng. Release tags and cgo come from the go tool started
// with rng's environment, which is what `go list` would use. An error
// from that query is returned so the caller can fall back to `go list`.
func importDir(rng *ring.Ring, dir string) (*build.Package, error) {
	ctxt := build.Default
	if goos := rng.EnvGet("GOOS"); goos != "" {
		ctxt.GOOS = goos
	}
	if goarch := rng.EnvGet("GOARCH"); goarch != "" {
		ctxt.GOARCH = goarch
	}
	rel, cgo, err := toolchainFacts(rng)
	if err != nil {
		return nil, err
	}
	ctxt.ReleaseTags = rel
	ctxt.CgoEnabled = cgo
	// Start from Default tags, then the same user list `go list -tags` gets.
	tags := append([]string{}, ctxt.BuildTags...)
	tags = append(tags, buildTags(rng)...)
	ctxt.BuildTags = tags
	return ctxt.ImportDir(dir, 0)
}

// toolchainFacts asks the go tool, under env, for the release tags of its
// version and whether cgo is enabled. CGO_ENABLED in env selects cgo.
func toolchainFacts(env ring.Environ) (tags []string, cgo bool, err error) {
	cmd := exec.Command("go", "env", "GOVERSION", "CGO_ENABLED") //nolint:noctx
	cmd.Env = env.EnvAll()
	out, err := cmd.Output()
	if err != nil {
		return nil, false, fmt.Errorf("go env: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		format := "go env GOVERSION CGO_ENABLED: %q"
		return nil, false, fmt.Errorf(format, strings.TrimSpace(string(out)))
	}
	tags, ok := releaseTagsFor(strings.TrimSpace(lines[0]))
	if !ok {
		format := "go env GOVERSION: %q"
		return nil, false, fmt.Errorf(format, strings.TrimSpace(lines[0]))
	}
	switch strings.TrimSpace(lines[1]) {
	case "1":
		cgo = true
	case "0":
		cgo = false
	default:
		format := "go env CGO_ENABLED: %q"
		return nil, false, fmt.Errorf(format, strings.TrimSpace(lines[1]))
	}
	return tags, cgo, nil
}

// releaseTagsFor returns the go1.x release tags for a Go version string
// such as "go1.26.0". The last tag is the version's own minor release.
func releaseTagsFor(version string) ([]string, bool) {
	const prefix = "go1."
	i := strings.Index(version, prefix)
	if i < 0 {
		return nil, false
	}
	rest := version[i+len(prefix):]
	n := 0
	digits := 0
	for _, c := range rest {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
		digits++
	}
	if digits == 0 {
		return nil, false
	}
	tags := make([]string, 0, n)
	for v := 1; v <= n; v++ {
		tags = append(tags, "go1."+strconv.Itoa(v))
	}
	return tags, true
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

// buildTags returns the user build tags for package loading: tags from
// GOFLAGS, then the meta build tag when one is set. `go list` replaces a
// GOFLAGS -tags list when -tags is also passed on the command line, so that
// invocation must pass this whole list.
func buildTags(rng *ring.Ring) []string {
	tags := goFlagsTags(rng.EnvGet("GOFLAGS"))
	if tag := GetBuildTag(rng); tag != "" {
		tags = append(tags, tag)
	}
	return tags
}

// listTagArgs returns the `-tags` arguments for `go list`, or nil when
// buildTags is empty. The tag list is comma-joined so one flag carries
// every tag.
func listTagArgs(rng *ring.Ring) []string {
	tags := buildTags(rng)
	if len(tags) == 0 {
		return nil
	}
	return []string{"-tags", strings.Join(tags, ",")}
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
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
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
