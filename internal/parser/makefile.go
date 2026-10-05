// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
	"path/filepath"
	"strings"

	"github.com/ctx42/ring/pkg/ring"
)

// Makefile represents the targets discovered in a package directory, together
// with any targets pulled in through `gomake:import` imports.
type Makefile struct {
	// Package documentation collapsed to a single line.
	Doc string

	// Targets defined in the main package and its `gomake:import` imports.
	Targets *Targets

	// Default target name.
	Default string

	// Build environment.
	env *ring.Ring

	// Package representing the main gomake package.
	pkg *Package
}

// NewMakefile returns [Makefile] for package in given directory. The path must
// be absolute.
func NewMakefile(rng *ring.Ring, impPath string) (*Makefile, error) {
	pkg, err := NewPackage(rng, impPath)
	if err != nil {
		return nil, err
	}
	return MakefileFromPackage(rng, pkg)
}

// MakefileFromPackage returns [Makefile] for given package.
func MakefileFromPackage(rng *ring.Ring, pkg *Package) (*Makefile, error) {
	pmf := &Makefile{
		Targets: NewTargets(),
		env:     rng,
		pkg:     pkg,
	}

	docPkg, err := pmf.addTargets()
	switch {
	case errors.Is(err, ErrAstEmpty):
		// When the import path has no targets, that is not really a problem
		// since gomake might have been called anywhere with some core target.
		return pmf, nil

	case err != nil:
		return nil, err
	}
	pmf.Doc = toOneLine(docPkg.Doc)
	pmf.Targets.reportSkips(rng)
	return pmf, nil
}

// addTargets discovers the targets in the makefile's package and records them
// on the [Makefile]. It adds targets from the package functions and types,
// pulls in targets from `gomake:import` imports, and marks the default target.
// Returns the package documentation.
func (pmf *Makefile) addTargets() (*doc.Package, error) {
	pkg := pmf.pkg

	// Always non-nil so an empty go-list selection is not a dir scan.
	files := make([]string, 0, len(pkg.Files))
	for _, name := range pkg.Files {
		files = append(files, filepath.Join(pkg.ImpPath, name))
	}
	astPkg, docPkg, err := astAndDocPkg(pkg.ImpPath, files)
	if err != nil {
		return nil, err
	}
	pkg.files = astPkg
	pmf.Targets.reserve(topLevelNames(astPkg)...)
	if err = pmf.Targets.addFunc(pkg, docPkg.Funcs...); err != nil {
		return nil, err
	}
	if err = pmf.Targets.addType(pkg, docPkg.Types...); err != nil {
		return nil, err
	}

	// Add targets from imports tagged with `gomake:import` in any of the
	// package files, resolving an import tagged in several files once.
	var decls []ast.Decl
	for _, name := range files {
		if fil, ok := astPkg[name]; ok {
			decls = append(decls, fil.Decls...)
		}
	}
	if err = pmf.addGmImports(decls...); err != nil {
		return nil, err
	}
	if err = pmf.markDefault(astPkg, docPkg.Vars...); err != nil {
		return nil, err
	}
	return docPkg, nil
}

// markDefault uses findDefault to locate the default target definition and,
// when found, marks the matching target as default. A leading import local
// name restricts the match to the targets of that import. A declared Default
// that matches no target is an error.
func (pmf *Makefile) markDefault(
	astPkg map[string]*ast.File,
	vars ...*doc.Value,
) error {

	ref, pos := findDefault(vars...)
	if ref == nil {
		return nil
	}
	// Resolve a leading import local name to its import path, so a package
	// name shared by several imports picks the target from the right one.
	defRef := strings.Join(ref, ".")
	var impSpec string
	if len(ref) >= 2 {
		if path, ok := fileImports(astPkg, pos)[ref[0]]; ok {
			if pkgName := pmf.Targets.pkgNameForImp(path); pkgName != "" {
				impSpec = path
				defRef = pkgName + "." + strings.Join(ref[1:], ".")
			}
		}
	}
	if name := pmf.Targets.markDefault(impSpec, defRef); name != "" {
		pmf.Default = name
		return nil
	}
	return fmt.Errorf("%w: %q", ErrNoDefault, strings.Join(ref, "."))
}

// addGmImports resolves the `gomake:import` imports in decls with
// gmImpPackages and adds the function and type targets found in each
// imported package.
func (pmf *Makefile) addGmImports(decls ...ast.Decl) error {
	pks, err := gmImpPackages(pmf.env, pmf.pkg.ImpPath, decls...)
	if err != nil {
		return err
	}
	for _, pkg := range pks {
		var astPkg map[string]*ast.File
		var docPkg *doc.Package
		// Non-nil even when empty: empty import packages must not re-scan.
		impFiles := make([]string, len(pkg.Files))
		copy(impFiles, pkg.Files)
		astPkg, docPkg, err = astAndDocPkg(pkg.ImpPath, impFiles)
		if errors.Is(err, ErrAstEmpty) {
			// Soft-skip: same as main package when go list selected no files.
			continue
		}
		if err != nil {
			return err
		}
		pkg.files = astPkg
		err = pmf.Targets.addFunc(pkg, docPkg.Funcs...)
		if err != nil {
			return err
		}
		err = pmf.Targets.addType(pkg, docPkg.Types...)
		if err != nil {
			return err
		}
	}
	return nil
}
