// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"errors"
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
	return pmf, nil
}

// addTargets discovers the targets in the makefile's package and records them
// on the [Makefile]. It adds targets from the package functions and types,
// pulls in targets from `gomake:import` imports, and marks the default target.
// Returns the package documentation.
func (pmf *Makefile) addTargets() (*doc.Package, error) {
	pkg := pmf.pkg

	var files []string
	for _, name := range pkg.Files {
		files = append(files, filepath.Join(pkg.ImpPath, name))
	}
	astPkg, docPkg, err := astAndDocPkg(pkg.ImpPath, files...)
	if err != nil {
		return nil, err
	}
	if err = pmf.Targets.addFunc(pkg, docPkg.Funcs...); err != nil {
		return nil, err
	}
	if err = pmf.Targets.addType(pkg, docPkg.Types...); err != nil {
		return nil, err
	}

	// Go over all the files in the package and add targets
	// from imports tagged with `gomake:import`.
	for _, name := range files {
		if fil, ok := astPkg[name]; ok {
			if err = pmf.adGmImports(fil); err != nil {
				return nil, err
			}
		}
	}
	pmf.markDefault(docPkg.Vars...)
	return docPkg, nil
}

// markDefault uses [findDefault] to locate the default target definition and,
// when found, marks the matching target as default.
func (pmf *Makefile) markDefault(vars ...*doc.Value) {
	if ref := findDefault(vars...); ref != nil {
		defRef := strings.Join(ref, ".")
		pmf.Default = pmf.Targets.MarkDefault(defRef)
	}
}

// adGmImports resolves the file's `gomake:import` imports with [gmImpPackages]
// and adds the function and type targets found in each imported package.
func (pmf *Makefile) adGmImports(fil *ast.File) error {
	pks, err := gmImpPackages(pmf.env, pmf.pkg.ImpPath, fil.Decls...)
	if err != nil {
		return err
	}
	for _, pkg := range pks {
		var docPkg *doc.Package
		_, docPkg, err = astAndDocPkg(pkg.ImpPath, pkg.Files...)
		if err != nil {
			return err
		}
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
