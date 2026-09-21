// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package version

import (
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testing/pkg/tester"
)

// saveVars restores the package build-metadata variables after the test.
func saveVars(t tester.T) {
	t.Helper()
	rev := scmRev
	t.Cleanup(func() { scmRev = rev })
}

// declaredVarNames returns the names of the package-level variables declared
// in the Go source file at path.
func declaredVarNames(t tester.T, path string) []string {
	t.Helper()
	src := must.Value(parser.ParseFile(token.NewFileSet(), path, nil, 0))
	names := make([]string, 0, 1)
	for _, dec := range src.Decls {
		gen, ok := dec.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			for _, id := range spec.(*ast.ValueSpec).Names {
				names = append(names, id.Name)
			}
		}
	}
	return names
}
