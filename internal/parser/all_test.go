// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"go/ast"
	"go/doc"
	"go/token"
	"path/filepath"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/notice"
	"github.com/ctx42/testing/pkg/tester"
)

// pkgName returns the package name from the first file in the map.
func pkgName(fls map[string]*ast.File) string {
	for _, f := range fls {
		return f.Name.Name
	}
	return ""
}

// getFile checks `fls` map has key equal to path build from `elems`. When path
// exists function returns the file instance at that key. Otherwise, it marks
// the test as failed, writes error message to test log and returns nil.
func getFile(t tester.T, fls map[string]*ast.File, elem ...string) *ast.File {
	t.Helper()
	pth := filepath.Join(elem...)
	if fil, ok := fls[pth]; ok {
		return fil
	}
	msg := notice.New("expected map to have a file").Append("path", "%s", pth)
	t.Error(msg)
	return nil
}

// TestHelper is a test helper for the parser package. It provides helper
// methods to easily retrieve package related "ast" and "doc" structures.
type TestHelper struct {
	impPath string               // Absolute path to the package directory.
	files   map[string]*ast.File // AST files keyed by absolute path.
	docPkg  *doc.Package         // Package documentation.
	pkg     *Package             // Package for impPath.
	t       tester.T             // Test manager.
}

// NewTestHelper returns new instance of TestHelper.
func NewTestHelper(
	t tester.T,
	rng *ring.Ring,
	impPath string,
	pkgOpts ...PackageOpt,
) *TestHelper {

	t.Helper()
	tst := &TestHelper{
		impPath: impPath,
		t:       t,
	}

	var err error
	tst.pkg, err = NewPackage(rng, impPath, pkgOpts...)
	if err != nil {
		t.Error(err)
		return nil
	}

	// Use only files in the package to built AST.
	var files []string
	for _, name := range tst.pkg.Files {
		files = append(files, filepath.Join(tst.pkg.ImpPath, name))
	}
	tst.files, tst.docPkg, err = astAndDocPkg(impPath, files...)
	if err != nil {
		t.Error(err)
		return nil
	}
	return tst
}

// Funcs returns documentation for all functions.
func (tst *TestHelper) Funcs() []*doc.Func {
	return tst.docPkg.Funcs
}

// Func returns documentation for function with given name.
func (tst *TestHelper) Func(name string) *doc.Func {
	tst.t.Helper()
	for _, fn := range tst.docPkg.Funcs {
		if fn.Name == name {
			return fn
		}
	}
	tst.t.Errorf("function %q not found", name)
	return nil
}

// Method returns documentation for method with given receiver and name.
func (tst *TestHelper) Method(rcv, name string) (*doc.Func, []string) {
	tst.t.Helper()
	for _, typ := range tst.docPkg.Types {
		for _, met := range typ.Methods {
			if met.Recv == rcv && met.Name == name {
				return met, breadcrumbs(typ)
			}
		}
	}
	tst.t.Errorf("method `%s.%s` not found", rcv, name)
	return nil, nil
}

// Types returns all types.
func (tst *TestHelper) Types() []*doc.Type {
	return tst.docPkg.Types
}

// Type returns type by name.
func (tst *TestHelper) Type(name string) *doc.Type {
	tst.t.Helper()
	for _, t := range tst.docPkg.Types {
		if t.Name == name {
			return t
		}
	}
	tst.t.Errorf("type %#q not found", name)
	return nil
}

// File returns file by filename. The name may be just a filename or absolute
// path rooted at import path passed to the constructor function.
func (tst *TestHelper) File(name string) *ast.File {
	tst.t.Helper()
	if !filepath.IsAbs(name) {
		name = filepath.Join(tst.impPath, name)
	}
	if fil, ok := tst.files[name]; ok {
		return fil
	}
	tst.t.Errorf("file %#q not found", name)
	return nil
}

// ImportSpecs returns given file import specs.
func (tst *TestHelper) ImportSpecs(name string) []*ast.ImportSpec {
	tst.t.Helper()
	fil := tst.File(name)
	var specs []*ast.ImportSpec
	for _, decl := range fil.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for i := 0; i < len(gen.Specs); i++ {
			if spec, ok := gen.Specs[i].(*ast.ImportSpec); ok {
				specs = append(specs, spec)
			}
		}
	}
	return specs
}

// Values returns package declared variables / constants.
func (tst *TestHelper) Values() []*doc.Value { return tst.docPkg.Vars }
