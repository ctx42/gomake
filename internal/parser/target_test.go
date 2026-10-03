// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"go/ast"
	"go/doc"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/modkit"

	gmt "github.com/ctx42/gomake/internal/cli/clitest"
)

func Test_newTarget_generic(t *testing.T) {
	// --- Given ---
	pkg := &Package{Name: "main"}
	fn := &doc.Func{
		Name: "Generic",
		Decl: &ast.FuncDecl{
			Name: ast.NewIdent("Generic"),
			Type: &ast.FuncType{
				TypeParams: &ast.FieldList{
					List: []*ast.Field{{
						Names: []*ast.Ident{ast.NewIdent("T")},
						Type:  ast.NewIdent("any"),
					}},
				},
			},
		},
	}

	// --- When ---
	have, err := newTarget(pkg, fn)

	// --- Then ---
	assert.ErrorIs(t, errGeneric, err)
	assert.Nil(t, have)
}

func Test_Targets_addFunc_alias(t *testing.T) {
	// --- Given ---
	pkg := &Package{
		Name:    "main",
		imports: map[string]string{"context": "context", "r": ringPath},
	}
	fn := &doc.Func{
		Name: "Aliased",
		Decl: &ast.FuncDecl{
			Name: ast.NewIdent("Aliased"),
			Type: &ast.FuncType{
				Params: &ast.FieldList{List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("ctx")},
						Type: &ast.SelectorExpr{
							X:   ast.NewIdent("context"),
							Sel: ast.NewIdent("Context"),
						},
					},
					{
						Names: []*ast.Ident{ast.NewIdent("rng")},
						Type: &ast.StarExpr{X: &ast.SelectorExpr{
							X:   ast.NewIdent("r"),
							Sel: ast.NewIdent("Ring"),
						}},
					},
				}},
				Results: &ast.FieldList{List: []*ast.Field{{
					Type: ast.NewIdent("error"),
				}}},
			},
		},
	}
	tgs := NewTargets()

	// --- When ---
	err := tgs.addFunc(pkg, fn)

	// --- Then ---
	assert.NoError(t, err)
	assert.Equal(t, 0, tgs.Len())
	assert.Equal(t, []string{"Aliased"}, tgs.skips)
}

func Test_Targets_reportSkips(t *testing.T) {
	// --- Given ---
	tst := ringtest.New(t).WetStderr()
	rng := tst.Ring()

	tgs := NewTargets()
	tgs.noteSkip("Aliased")

	// --- When ---
	tgs.reportSkips(rng)

	// --- Then ---
	want := "gomake: skipping Aliased: " +
		"aliased context or ring parameter\n"
	assert.Equal(t, want, tst.Stderr())
}

func Test_newTarget_main_package(t *testing.T) {
	relPath := "testdata/projects/showcase_targets/project"
	absPath := modkit.Path(relPath)
	impSpec := gmt.JoinImpSpec(t, gmt.GmModName, relPath)

	t.Run("basic target", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		have, err := newTarget(tst.pkg, tst.Func("Basic"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, impSpec, have.ImpSpec)
		assert.Equal(t, absPath, have.ImpPath)
		assert.Equal(t, "main", have.PkgName)
		assert.Equal(t, "", have.PkgNS)
		assert.Nil(t, have.Breadcrumbs)
		assert.Equal(t, "", have.Receiver)
		assert.Equal(t, "Basic", have.FuncName)
		assert.Equal(t, "basic", have.Name)
		assert.Equal(t, "", have.VarName)
		assert.Equal(t, "Basic", have.CodeRef)
		assert.Equal(t, "Basic", have.DefRef)
		assert.False(t, have.Default)
		assert.Equal(t, "prints its name to stdout", have.Synopsis)
		want := "prints its name to stdout. Some more detailed " +
			"documentation here. Even more detailed documentation."
		assert.Equal(t, want, have.Doc)
		assert.False(t, have.Hidden)
		assert.NotNil(t, have.Run)
		assert.Fields(t, 16, have)
	})

	t.Run("package imported with namespace", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath, withPkgNS("ns"))

		// --- When ---
		have, err := newTarget(tst.pkg, tst.Func("Basic"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "ns", have.PkgNS)
		assert.Nil(t, have.Breadcrumbs)
		assert.Equal(t, "ns:basic", have.Name)

	})

	t.Run("kebab case namespace", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)
		met, nsp := tst.Method("KebabCase", "HelloWorld")

		// --- When ---
		have, err := newTarget(tst.pkg, met, nsp...)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", have.PkgNS)
		assert.Equal(t, []string{rootCrumb, "KebabCase"}, have.Breadcrumbs)
		assert.Equal(t, "kebab-case:hello-world", have.Name)
		assert.Equal(t, "_vmainKebabCase", have.VarName)
		assert.Equal(t, "_vmainKebabCase.HelloWorld", have.CodeRef)
	})

	t.Run("error - not exported", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		have, err := newTarget(tst.pkg, tst.Func("notExported"))

		// --- Then ---
		assert.ErrorIs(t, errNotExported, err)
		assert.Nil(t, have)
	})

	t.Run("error - invalid argument type", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		have, err := newTarget(tst.pkg, tst.Func("ArgInvType"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, have)
	})

	t.Run("invalid multi context", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		have, err := newTarget(tst.pkg, tst.Func("ArgInvMultiCtx"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, have)
	})

	t.Run("invalid multi context with reused arguments", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		have, err := newTarget(tst.pkg, tst.Func("ArgInvMultiCtxReuse"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, have)
	})

	t.Run("invalid context argument position", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		have, err := newTarget(tst.pkg, tst.Func("ArgInvCtxPosition"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, have)
	})

	t.Run("invalid no arguments", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		have, err := newTarget(tst.pkg, tst.Func("ArgInvNoArgs"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, have)
	})

	t.Run("invalid no context argument", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		have, err := newTarget(tst.pkg, tst.Func("ArgInvNoCtx"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, have)
	})

	t.Run("invalid returning multiple values", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		have, err := newTarget(tst.pkg, tst.Func("RetInvMulti"))

		// --- Then ---
		assert.ErrorIs(t, errInvResult, err)
		assert.Nil(t, have)
	})

	t.Run("invalid to many arguments", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		have, err := newTarget(tst.pkg, tst.Func("ArgInvToMany"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, have)
	})

	t.Run("invalid to many reused arguments", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		have, err := newTarget(tst.pkg, tst.Func("ArgInvToManyReuse"))

		// --- Then ---
		assert.ErrorIs(t, errInvArg, err)
		assert.Nil(t, have)
	})

	t.Run("invalid return type", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		have, err := newTarget(tst.pkg, tst.Func("RetInvType"))

		// --- Then ---
		assert.ErrorIs(t, errInvResult, err)
		assert.Nil(t, have)
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
		have, err := newTarget(tst.pkg, tst.Func("Pkg1"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, impSpec, have.ImpSpec)
		assert.Equal(t, absPath, have.ImpPath)
		assert.Equal(t, "pkg1", have.PkgName)
		assert.Equal(t, "", have.PkgNS)
		assert.Nil(t, have.Breadcrumbs)
		assert.Equal(t, "", have.Receiver)
		assert.Equal(t, "Pkg1", have.FuncName)
		assert.Equal(t, "pkg1", have.Name)
		assert.Equal(t, "", have.VarName)
		assert.Equal(t, "pkg1.Pkg1", have.CodeRef)
		assert.Equal(t, "pkg1.Pkg1", have.DefRef)
		assert.False(t, have.Default)
		assert.Equal(t, "", have.Synopsis)
		assert.Equal(t, "", have.Doc)
		assert.False(t, have.Hidden)
		assert.NotNil(t, have.Run)
		assert.Fields(t, 16, have)
	})

	t.Run("target imported with namespace", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/imports/pkg1"
		absPath := modkit.Path(relPath)
		impSpec := gmt.JoinImpSpec(t, gmt.GmModName, relPath)

		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath, withPkgNS("aa"))

		// --- When ---
		have, err := newTarget(tst.pkg, tst.Func("Pkg1"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, impSpec, have.ImpSpec)
		assert.Equal(t, absPath, have.ImpPath)
		assert.Equal(t, "pkg1", have.PkgName)
		assert.Equal(t, "aa", have.PkgNS)
		assert.Nil(t, have.Breadcrumbs)
		assert.Equal(t, "", have.Receiver)
		assert.Equal(t, "Pkg1", have.FuncName)
		assert.Equal(t, "aa:pkg1", have.Name)
		assert.Equal(t, "", have.VarName)
		assert.Equal(t, "pkg1.Pkg1", have.CodeRef)
		assert.Equal(t, "pkg1.Pkg1", have.DefRef)
		assert.False(t, have.Default)
		assert.Equal(t, "", have.Synopsis)
		assert.Equal(t, "", have.Doc)
		assert.False(t, have.Hidden)
		assert.NotNil(t, have.Run)
		assert.Fields(t, 16, have)
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
		have, err := newTarget(tst.pkg, met, nsp...)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, impSpec, have.ImpSpec)
		assert.Equal(t, absPath, have.ImpPath)
		assert.Equal(t, "pkg4", have.PkgName)
		assert.Equal(t, "", have.PkgNS)
		assert.Equal(t, []string{"__root__", "NS"}, have.Breadcrumbs)
		assert.Equal(t, "NS", have.Receiver)
		assert.Equal(t, "M0", have.FuncName)
		assert.Equal(t, "ns:m0", have.Name)
		assert.Equal(t, "_vpkg4NS", have.VarName)
		assert.Equal(t, "_vpkg4NS.M0", have.CodeRef)
		assert.Equal(t, "pkg4.NS.M0", have.DefRef)
		assert.False(t, have.Default)
		assert.Equal(t, "is a method in namespace NS", have.Synopsis)
		want := "is a method in namespace NS. The rest of the doc string."
		assert.Equal(t, want, have.Doc)
		assert.False(t, have.Hidden)
		assert.NotNil(t, have.Run)
		assert.Fields(t, 16, have)
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
		have, err := newTarget(tst.pkg, met, nsp...)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, impSpec, have.ImpSpec)
		assert.Equal(t, absPath, have.ImpPath)
		assert.Equal(t, "pkg4", have.PkgName)
		assert.Equal(t, "aa", have.PkgNS)
		assert.Equal(t, []string{"__root__", "NS"}, have.Breadcrumbs)
		assert.Equal(t, "NS", have.Receiver)
		assert.Equal(t, "M0", have.FuncName)
		assert.Equal(t, "aa:ns:m0", have.Name)
		assert.Equal(t, "_vpkg4NS", have.VarName)
		assert.Equal(t, "_vpkg4NS.M0", have.CodeRef)
		assert.Equal(t, "pkg4.NS.M0", have.DefRef)
		assert.False(t, have.Default)
		assert.Equal(t, "is a method in namespace NS", have.Synopsis)
		want := "is a method in namespace NS. The rest of the doc string."
		assert.Equal(t, want, have.Doc)
		assert.False(t, have.Hidden)
		assert.NotNil(t, have.Run)
		assert.Fields(t, 16, have)
	})
}

func Test_newTarget_strips_nolint_from_doc(t *testing.T) {
	// --- Given ---
	pkg := &Package{Name: MainName}

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

	// --- When ---
	have, err := newTarget(pkg, df)

	// --- Then ---
	assert.NoError(t, err)
	assert.Equal(t, "does stuff. More docs.", have.Doc)
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
