// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"go/ast"
	"go/doc"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/modkit"

	gmt "github.com/ctx42/gomake/internal/cli/clitest"
)

func Test_newTarget_main_package(t *testing.T) {
	relPath := "testdata/projects/showcase_targets/project"
	absPath := modkit.Path(relPath)
	impSpec := gmt.JoinImpSpec(t, gmt.GmModName, relPath)

	t.Run("basic target", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("Basic"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, impSpec, tgt.ImpSpec)
		assert.Equal(t, absPath, tgt.ImpPath)
		assert.Equal(t, "main", tgt.PkgName)
		assert.Equal(t, "", tgt.PkgNS)
		assert.Nil(t, tgt.Breadcrumbs)
		assert.Equal(t, "", tgt.Receiver)
		assert.Equal(t, "Basic", tgt.FuncName)
		assert.Equal(t, "basic", tgt.Name)
		assert.Equal(t, "", tgt.VarName)
		assert.Equal(t, "Basic", tgt.CodeRef)
		assert.Equal(t, "Basic", tgt.DefRef)
		assert.False(t, tgt.Default)
		assert.Equal(t, "prints its name to stdout", tgt.Synopsis)
		want := "prints its name to stdout. Some more detailed " +
			"documentation here. Even more detailed documentation."
		assert.Equal(t, want, tgt.Doc)
		assert.False(t, tgt.Hidden)
		assert.NotNil(t, tgt.Run)
		assert.Fields(t, 16, tgt)
	})

	t.Run("package imported with namespace", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath, withPkgNS("ns"))

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("Basic"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "ns", tgt.PkgNS)
		assert.Nil(t, tgt.Breadcrumbs)
		assert.Equal(t, "ns:basic", tgt.Name)

	})

	t.Run("kebab case namespace", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)
		met, nsp := tst.Method("KebabCase", "HelloWorld")

		// --- When ---
		tgt, err := newTarget(tst.pkg, met, nsp...)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", tgt.PkgNS)
		assert.Equal(t, []string{rootCrumb, "KebabCase"}, tgt.Breadcrumbs)
		assert.Equal(t, "kebab-case:hello-world", tgt.Name)
		assert.Equal(t, "_vmainKebabCase", tgt.VarName)
		assert.Equal(t, "_vmainKebabCase.HelloWorld", tgt.CodeRef)
	})

	t.Run("not exported error", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("notExported"))

		// --- Then ---
		assert.ErrorIs(t, errNotExported, err)
		assert.Nil(t, tgt)
	})

	t.Run("invalid argument type", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("ArgInvType"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, tgt)
	})

	t.Run("invalid multi context", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("ArgInvMultiCtx"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, tgt)
	})

	t.Run("invalid multi context with reused arguments", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("ArgInvMultiCtxReuse"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, tgt)
	})

	t.Run("invalid context argument position", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("ArgInvCtxPosition"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, tgt)
	})

	t.Run("invalid no arguments", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("ArgInvNoArgs"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, tgt)
	})

	t.Run("invalid no context argument", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("ArgInvNoCtx"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, tgt)
	})

	t.Run("invalid returning multiple values", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("RetInvMulti"))

		// --- Then ---
		assert.ErrorIs(t, errInvResult, err)
		assert.Nil(t, tgt)
	})

	t.Run("invalid to many arguments", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("ArgInvToMany"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, tgt)
	})

	t.Run("invalid to many reused arguments", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("ArgInvToManyReuse"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, tgt)
	})

	t.Run("invalid return type", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("RetInvType"))

		// --- Then ---
		assert.ErrorIs(t, errInvResult, err)
		assert.Nil(t, tgt)
	})
}

func Test_newTarget_non_main_package(t *testing.T) {
	t.Run("target", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/imports/pkg1"
		absPath := modkit.Path(relPath)
		impSpec := gmt.JoinImpSpec(t, gmt.GmModName, relPath)

		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("Pkg1"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, impSpec, tgt.ImpSpec)
		assert.Equal(t, absPath, tgt.ImpPath)
		assert.Equal(t, "pkg1", tgt.PkgName)
		assert.Equal(t, "", tgt.PkgNS)
		assert.Nil(t, tgt.Breadcrumbs)
		assert.Equal(t, "", tgt.Receiver)
		assert.Equal(t, "Pkg1", tgt.FuncName)
		assert.Equal(t, "pkg1", tgt.Name)
		assert.Equal(t, "", tgt.VarName)
		assert.Equal(t, "pkg1.Pkg1", tgt.CodeRef)
		assert.Equal(t, "pkg1.Pkg1", tgt.DefRef)
		assert.False(t, tgt.Default)
		assert.Equal(t, "", tgt.Synopsis)
		assert.Equal(t, "", tgt.Doc)
		assert.False(t, tgt.Hidden)
		assert.NotNil(t, tgt.Run)
		assert.Fields(t, 16, tgt)
	})

	t.Run("target imported with namespace", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/imports/pkg1"
		absPath := modkit.Path(relPath)
		impSpec := gmt.JoinImpSpec(t, gmt.GmModName, relPath)

		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath, withPkgNS("aa"))

		// --- When ---
		tgt, err := newTarget(tst.pkg, tst.Func("Pkg1"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, impSpec, tgt.ImpSpec)
		assert.Equal(t, absPath, tgt.ImpPath)
		assert.Equal(t, "pkg1", tgt.PkgName)
		assert.Equal(t, "aa", tgt.PkgNS)
		assert.Nil(t, tgt.Breadcrumbs)
		assert.Equal(t, "", tgt.Receiver)
		assert.Equal(t, "Pkg1", tgt.FuncName)
		assert.Equal(t, "aa:pkg1", tgt.Name)
		assert.Equal(t, "", tgt.VarName)
		assert.Equal(t, "pkg1.Pkg1", tgt.CodeRef)
		assert.Equal(t, "pkg1.Pkg1", tgt.DefRef)
		assert.False(t, tgt.Default)
		assert.Equal(t, "", tgt.Synopsis)
		assert.Equal(t, "", tgt.Doc)
		assert.False(t, tgt.Hidden)
		assert.NotNil(t, tgt.Run)
		assert.Fields(t, 16, tgt)
	})

	t.Run("namespaced target", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/imports/pkg4"
		absPath := modkit.Path(relPath)
		impSpec := gmt.JoinImpSpec(t, gmt.GmModName, relPath)

		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)
		met, nsp := tst.Method("NS", "M0")

		// --- When ---
		tgt, err := newTarget(tst.pkg, met, nsp...)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, impSpec, tgt.ImpSpec)
		assert.Equal(t, absPath, tgt.ImpPath)
		assert.Equal(t, "pkg4", tgt.PkgName)
		assert.Equal(t, "", tgt.PkgNS)
		assert.Equal(t, []string{"__root__", "NS"}, tgt.Breadcrumbs)
		assert.Equal(t, "NS", tgt.Receiver)
		assert.Equal(t, "M0", tgt.FuncName)
		assert.Equal(t, "ns:m0", tgt.Name)
		assert.Equal(t, "_vpkg4NS", tgt.VarName)
		assert.Equal(t, "_vpkg4NS.M0", tgt.CodeRef)
		assert.Equal(t, "pkg4.NS.M0", tgt.DefRef)
		assert.False(t, tgt.Default)
		assert.Equal(t, "is a method in namespace NS", tgt.Synopsis)
		want := "is a method in namespace NS. The rest of the doc string."
		assert.Equal(t, want, tgt.Doc)
		assert.False(t, tgt.Hidden)
		assert.NotNil(t, tgt.Run)
		assert.Fields(t, 16, tgt)
	})

	t.Run("namespaced target imported with namespace", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/imports/pkg4"
		absPath := modkit.Path(relPath)
		impSpec := gmt.JoinImpSpec(t, gmt.GmModName, relPath)

		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath, withPkgNS("aa"))
		met, nsp := tst.Method("NS", "M0")

		// --- When ---
		tgt, err := newTarget(tst.pkg, met, nsp...)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, impSpec, tgt.ImpSpec)
		assert.Equal(t, absPath, tgt.ImpPath)
		assert.Equal(t, "pkg4", tgt.PkgName)
		assert.Equal(t, "aa", tgt.PkgNS)
		assert.Equal(t, []string{"__root__", "NS"}, tgt.Breadcrumbs)
		assert.Equal(t, "NS", tgt.Receiver)
		assert.Equal(t, "M0", tgt.FuncName)
		assert.Equal(t, "aa:ns:m0", tgt.Name)
		assert.Equal(t, "_vpkg4NS", tgt.VarName)
		assert.Equal(t, "_vpkg4NS.M0", tgt.CodeRef)
		assert.Equal(t, "pkg4.NS.M0", tgt.DefRef)
		assert.False(t, tgt.Default)
		assert.Equal(t, "is a method in namespace NS", tgt.Synopsis)
		want := "is a method in namespace NS. The rest of the doc string."
		assert.Equal(t, want, tgt.Doc)
		assert.False(t, tgt.Hidden)
		assert.NotNil(t, tgt.Run)
		assert.Fields(t, 16, tgt)
	})
}

func Test_newTarget_strips_nolint_from_doc(t *testing.T) {
	// --- Given ---
	// A target whose doc carries a spaced "// nolint" line. go/doc keeps such
	// a line in Func.Doc (only the "//" prefix is stripped), so NewTarget must
	// remove it from the rendered documentation.
	df := &doc.Func{
		Name: "Basic",
		Doc:  "Basic does stuff.\nnolint:gocyclo\nMore docs.",
		Decl: &ast.FuncDecl{
			Name: ast.NewIdent("Basic"),
			Type: &ast.FuncType{
				Params: &ast.FieldList{List: []*ast.Field{
					{Type: &ast.SelectorExpr{
						X:   ast.NewIdent("context"),
						Sel: ast.NewIdent("Context"),
					}},
					{Type: &ast.StarExpr{X: &ast.SelectorExpr{
						X:   ast.NewIdent("ring"),
						Sel: ast.NewIdent("Ring"),
					}}},
				}},
				Results: &ast.FieldList{List: []*ast.Field{
					{Type: ast.NewIdent("error")},
				}},
			},
		},
	}
	pkg := &Package{Name: MainName}

	// --- When ---
	tgt, err := newTarget(pkg, df)

	// --- Then ---
	assert.NoError(t, err)
	assert.Equal(t, "does stuff. More docs.", tgt.Doc)
}

func Test_targetName_tabular(t *testing.T) {
	tt := []struct {
		testN string

		ns       string
		nsPath   []string
		funcName string
		want     string
	}{
		{
			"func",
			"",
			nil,
			"Basic",
			"basic",
		},
		{
			"namespaced func",
			"ns",
			nil,
			"Basic",
			"ns:basic",
		},
		{
			"method",
			"",
			[]string{rootCrumb, "NS"},
			"Basic",
			"ns:basic",
		},
		{
			"namespaced method",
			"ns0",
			[]string{rootCrumb, "NS1"},
			"Basic",
			"ns0:ns1:basic",
		},
		{
			"namespaced chained method",
			"ns0",
			[]string{rootCrumb, "NS1", "NS2"},
			"Basic",
			"ns0:ns1:ns2:basic",
		},
		{
			"kebab case func",
			"",
			nil,
			"BasicArgs",
			"basic-args",
		},
		{
			"kebab case method",
			"",
			[]string{rootCrumb, "NS0"},
			"BasicArgs",
			"ns0:basic-args",
		},
		{
			"kebab case namespace",
			"",
			[]string{rootCrumb, "SomeType"},
			"BasicArgs",
			"some-type:basic-args",
		},
		{
			"default method",
			"",
			[]string{rootCrumb, "NS"},
			"Default",
			"ns",
		},
		{
			"namespaced default method",
			"ns0",
			[]string{rootCrumb, "NS1"},
			"Default",
			"ns0:ns1",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := targetName(tc.ns, tc.nsPath, tc.funcName)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
