// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ctx42/ring/pkg/ring"
)

// PackageOpt is NewPackage option signature.
type PackageOpt func(*Package)

// withPkgSpec is a [NewPackage] option to treat "imp" argument as import spec.
func withPkgSpec(pkg *Package) {
	pkg.ImpPath, pkg.ImpSpec = "", pkg.ImpPath
}

// withPkgNS is a [NewPackage] option setting package namespace.
func withPkgNS(ns string) func(*Package) {
	return func(pkg *Package) { pkg.PkgNS = ns }
}

// withPkgDir is a [NewPackage] option setting the directory `go list` runs in
// to resolve an import spec. It matters only for import specs, whose resolution
// depends on the go.mod of the module rooted at dir; an empty dir falls back to
// the process working directory. Apply it after [withPkgSpec], which clears the
// package path.
func withPkgDir(dir string) func(*Package) {
	return func(pkg *Package) { pkg.ImpPath = dir }
}

// module represents Go module.
type module struct {
	// Import spec (github.com/ctx42/gomake).
	ImpSpec string `json:"Path"`

	// Absolute path to the module's root directory (where go.mod is).
	ImpPath string `json:"Dir"`

	// Absolute path to the module's go.mod file.
	ModPath string `json:"GoMod"`

	// Module version, empty for the main module. A non-empty version marks an
	// immutable module-cache package whose `go list` result is safe to cache.
	Version string `json:"Version"`

	// Replace is set when the module is redirected by a `replace` directive. A
	// replaced module may resolve to a mutable local directory, so its `go
	// list` result must never be cached.
	Replace *module `json:"Replace"`
}

// listError represents "go list" error.
type listError struct {
	Err string `json:"Err"`
}

// Package represents Go package.
type Package struct {
	// Package namespace.
	//
	// When set, it is prepended to breadcrumbs of all targets in the package.
	PkgNS string `json:"-"`

	// Absolute package path.
	//
	// This is also a working directory for "go list" command execution. May be
	// empty (meaning current working directory) when executing "go list" for
	// an import spec.
	ImpPath string `json:"Dir"`

	// Go import string (spec).
	ImpSpec string `json:"ImportPath"`

	// Package name.
	Name string `json:"Name"`

	// Filenames in the package.
	Files []string `json:"GoFiles"`

	// Module information.
	Module module `json:"Module"`

	// Execution error.
	Error listError `json:"Error"`

	// Arguments for the "go list" command.
	args []string
}

// NewPackage runs "go list" to get information about a package identified by
// either import spec or absolute path.
func NewPackage(
	rng *ring.Ring,
	impPath string,
	opts ...PackageOpt,
) (*Package, error) {

	pkg := &Package{
		ImpPath: impPath,
		args:    []string{"list", "-e", "-json"},
	}
	for _, opt := range opts {
		opt(pkg)
	}
	if pkg.ImpSpec == "" && !filepath.IsAbs(pkg.ImpPath) {
		return nil, fmt.Errorf("%w: %s", ErrAbsPath, impPath)
	}

	// A local absolute directory can be resolved in-process, avoiding the
	// `go list` subprocess. Fall back to `go list` on any ambiguity.
	if pkg.ImpSpec == "" && newLocalPackage(rng, pkg) {
		return pkg, nil
	}

	// Build command arguments.
	args := make([]string, len(pkg.args), len(pkg.args)+3)
	copy(args, pkg.args)
	if bt := GetBuildTag(rng); bt != "" {
		args = append(args, "-tags", bt)
	}
	if pkg.ImpSpec != "" {
		args = append(args, pkg.ImpSpec)
	}

	if pkg.ImpPath == "" && pkg.ImpSpec != "" && !filepath.IsAbs(pkg.ImpSpec) {
		src := strings.TrimSpace(rng.EnvGet("GOMAKE_PROJECT_DIR"))
		if src != "" {
			pkg.ImpPath = src
			if !filepath.IsAbs(pkg.ImpPath) {
				if wd, err := os.Getwd(); err == nil {
					pkg.ImpPath = filepath.Join(wd, pkg.ImpPath)
				}
			}
			pkg.ImpPath, _ = filepath.Abs(pkg.ImpPath)
		}
	}

	// Serve immutable external module resolutions from the on-disk cache.
	key, cacheable := "", false
	if pkg.ImpSpec != "" {
		key, cacheable = listCacheKey(rng, pkg.ImpPath, pkg.ImpSpec)
	}
	if cacheable {
		if data, ok := loadListCache(key); ok {
			// Unmarshal into a copy so a rejected entry does not clobber pkg.
			// Reject entries whose resolved directory has vanished (e.g. a
			// pruned module cache) and fall through to `go list`.
			tmp := *pkg
			if err := json.Unmarshal(data, &tmp); err == nil {
				if _, sErr := os.Stat(tmp.ImpPath); sErr == nil {
					*pkg = tmp
					return pkg, nil
				}
			}
		}
	}

	sout, eout := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := exec.Command("go", args...)
	cmd.Env = rng.EnvAll()
	cmd.Dir = pkg.ImpPath
	cmd.Stdout, cmd.Stderr = sout, eout
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(eout.String())
		if detail == "" {
			detail = strings.TrimSpace(sout.String())
		}
		if detail != "" {
			return nil, fmt.Errorf("%w %s: %s", ErrGoList, impPath, detail)
		}
		return nil, fmt.Errorf("%w %s", ErrGoList, impPath)
	}
	if err := json.Unmarshal(sout.Bytes(), pkg); err != nil {
		return nil, err
	}

	if pkg.Error.Err != "" {
		// Without a resolved package dir, directory discovery would walk the
		// current project tree and invent bogus subpaths (e.g. .../cmd/gomake).
		fatal := strings.TrimSpace(pkg.ImpPath) == ""
		for _, frag := range []string{
			"no required module provides package",
			"malformed import path",
			"replaced but not required",
		} {
			fatal = fatal || strings.Contains(pkg.Error.Err, frag)
		}
		if fatal {
			spec := pkg.ImpSpec
			if spec == "" {
				spec = impPath
			}
			return nil, fmt.Errorf("%w %s: %s", ErrGoList, spec, pkg.Error.Err)
		}
	}

	if cacheable && pkg.Error.Err == "" && cacheableModule(pkg.Module) {
		storeListCache(key, sout.Bytes())
	}
	return pkg, nil
}
