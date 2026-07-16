// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// astFiles parses the Go files in impPath (or the given paths) and returns a
// file set and a map from the absolute path to parsed [ast.File]. Returns error
// if none or more than one package name is found, or a parse error occurred.
//
// A nil files means scan every non-directory *.go under impPath. A non-nil
// files (including empty) uses only that list so an empty go-list selection
// yields [ErrAstEmpty] instead of re-scanning the directory.
//
// go/parser.ParseFile is used instead of go/packages because go/packages
// invokes the full build system; ParseFile is sufficient for AST-only parsing.
func astFiles(
	impPath string,
	files []string,
) (*token.FileSet, map[string]*ast.File, error) {

	var toparse []string
	if files != nil {
		toparse = make([]string, 0, len(files))
		for _, f := range files {
			if !filepath.IsAbs(f) {
				f = filepath.Join(impPath, f)
			}
			toparse = append(toparse, f)
		}
	} else {
		entries, err := os.ReadDir(impPath)
		if err != nil {
			format := "%w at %s: %w"
			return nil, nil, fmt.Errorf(format, errAstParse, impPath, err)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
				toparse = append(toparse, filepath.Join(impPath, e.Name()))
			}
		}
	}

	if len(toparse) == 0 {
		return nil, nil, fmt.Errorf("%w: %s", ErrAstEmpty, impPath)
	}

	set := token.NewFileSet()
	fls := make(map[string]*ast.File, len(toparse))
	pkgNames := make(map[string]struct{})
	for _, path := range toparse {
		af, err := parser.ParseFile(set, path, nil, parser.ParseComments)
		if err != nil {
			format := "%w at %s: %w"
			return nil, nil, fmt.Errorf(format, errAstParse, impPath, err)
		}
		fls[path] = af
		pkgNames[af.Name.Name] = struct{}{}
	}

	if len(pkgNames) > 1 {
		names := make([]string, 0, len(pkgNames))
		for n := range pkgNames {
			names = append(names, n)
		}
		sort.Strings(names)
		return nil, nil, fmt.Errorf(
			"%w: %s: %s",
			errAstMultiPkg,
			impPath,
			strings.Join(names, ", "),
		)
	}

	return set, fls, nil
}

// astAndDocPkg returns the parsed AST files and documentation for the package
// at impPath (or the given subset of files). A nil files scans the directory;
// a non-nil files list is used as-is (see astFiles). Returns error if none
// or more than one package is detected or a parse error occurred.
func astAndDocPkg(
	impPath string,
	files []string,
) (map[string]*ast.File, *doc.Package, error) {

	set, fls, err := astFiles(impPath, files)
	if err != nil {
		return nil, nil, err
	}
	all := make([]*ast.File, 0, len(fls))
	for _, f := range fls {
		all = append(all, f)
	}
	docPkg, err := doc.NewFromFiles(set, all, impPath, doc.AllDecls)
	if err != nil {
		return nil, nil, err
	}
	return fls, docPkg, nil
}
