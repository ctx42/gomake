// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"go/ast"
	"go/doc"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/goldy"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"

	gmt "github.com/ctx42/gomake/internal/cli/clitest"
	"github.com/ctx42/gomake/internal/mkf"
)

func Test_genMakefileUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())

		relPath := "testdata/projects/showcase_imports/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		// --- When ---
		hCode, hTgs, err := genMakefileUser(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		wantNames := []string{
			"abc:ns0:hello",
			"abc:ns0:ns1:hello",
			"abc:ns0:ns1:ns2:hello-hello",
			"imported",
			"local",
			"mx:panic-string",
			"mx:print",
			"ns:pkg1",
			"pkg0",
		}
		assert.Equal(t, wantNames, hTgs.Names())

		prj.ChdirBack()
		gfp := "testdata/mkf_user_main.gld"
		gfd := map[string]any{"gmk_root": modkit.Root(), "prj_root": prj.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))
		assert.Equal(t, gld.String(), string(hCode))
	})

	t.Run("error - duplicated targets", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())

		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path("testdata/projects/dup_imported/project"))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		// --- When ---
		hCode, hTgs, err := genMakefileUser(rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.Nil(t, hTgs)
		assert.Nil(t, hCode)
	})
}

func Test_GenMakefileUserAndSave(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())

		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(modkit.Path("testdata/projects/no_targets/project"))
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		dst := prj.Path(mkf.MakefileUser)

		// --- When ---
		have, err := GenMakefileUserAndSave(rng, prj.Root(), dst)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 0, have.Len())
		gfp := "testdata/gen_no_targets_main_init.gld"
		gfd := map[string]any{"prj_root": modkit.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))
		assert.Equal(t, gld.String(), oskit.ReadFileStr(t, dst))
	})

	t.Run("error - generating code", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		absPath := modkit.Path("testdata/projects/dup_imported/project")

		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		dst := prj.Path(mkf.MakefileUser)

		// --- When ---
		have, err := GenMakefileUserAndSave(rng, prj.Root(), dst)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.Nil(t, have)
	})

	t.Run("error - writing file", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		absPath := modkit.Path("testdata/projects/no_targets/project")

		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		dst := t.TempDir()

		// --- When ---
		have, err := GenMakefileUserAndSave(rng, prj.Root(), dst)

		// --- Then ---
		var e *os.PathError
		assert.ErrorAs(t, &e, err)
		assert.Equal(t, dst, e.Path)
		assert.Nil(t, have)
	})
}

func Test_GetBuildTag(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		// --- When ---
		have := GetBuildTag(ring.New())

		// --- Then ---
		assert.Empty(t, have)
	})

	t.Run("set", func(t *testing.T) {
		// --- When ---
		have := GetBuildTag(SetBuildTag(ring.New()))

		// --- Then ---
		assert.Equal(t, BuildTag, have)
	})
}

func Test_SetBuildTag(t *testing.T) {
	// --- Given ---
	rng := ring.New()

	// --- When ---
	have := SetBuildTag(rng)

	// --- Then ---
	assert.Equal(t, BuildTag, GetBuildTag(have))
	assert.Equal(t, rng.MetaAll(), have.MetaAll())
}

func Test_RemBuildTag(t *testing.T) {
	// --- Given ---
	rng := SetBuildTag(ring.New())

	// --- When ---
	have := RemBuildTag(rng)

	// --- Then ---
	assert.Empty(t, GetBuildTag(have))
	assert.Empty(t, GetBuildTag(rng))
}

func Test_toOneLine_tabular(t *testing.T) {
	tt := []struct {
		testN string

		in   string
		want string
	}{
		{"multi line", "a\nb\nc\nd", "a b c d"},
		{"multi line break", "a\nb\n\nc\nd", "a b  c d"},
		{"single line", "a b c d", "a b c d"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := toOneLine(tc.in)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_removeNoLintComments(t *testing.T) {
	t.Run("doc string with nolint", func(t *testing.T) {
		// --- When ---
		have := removeNoLintComments("abc\ndef\nnolint:xxx\nghi")

		// --- Then ---
		assert.Equal(t, "abc\ndef\nghi", have)
	})

	t.Run("doc string without nolint", func(t *testing.T) {
		// --- When ---
		have := removeNoLintComments("abc\ndef\nghi")

		// --- Then ---
		assert.Equal(t, "abc\ndef\nghi", have)
	})
}

func Test_isHidden(t *testing.T) {
	t.Run("hidden target", func(t *testing.T) {
		// --- When ---
		hDoc, hHidden := isHidden("abc\n\ngomake:hidden\ndef")

		// --- Then ---
		assert.Equal(t, "abc\n\ndef", hDoc)
		assert.True(t, hHidden)
	})

	t.Run("not hidden target", func(t *testing.T) {
		// --- When ---
		hDoc, hHidden := isHidden("abc\ndef\nghi")

		// --- Then ---
		assert.Equal(t, "abc\ndef\nghi", hDoc)
		assert.False(t, hHidden)
	})
}

func Test_removeLines(t *testing.T) {
	t.Run("line over scanner limit", func(t *testing.T) {
		// --- Given ---
		long := strings.Repeat("a", 70_000)

		// --- When ---
		have, hN := removeLines(long+"\nnolint:xxx", "nolint")

		// --- Then ---
		assert.Equal(t, long, have)
		assert.Equal(t, 1, hN)
	})
}

func Test_removeLines_tabular(t *testing.T) {
	tt := []struct {
		testN string

		in      string
		remove  string
		wantStr string
		wantNum int
	}{
		{
			"with",
			"abc def \nnolint:xxx",
			"nolint",
			"abc def",
			1,
		},
		{
			"with and following text",
			"abc def \nnolint:xxx ghi",
			"nolint",
			"abc def",
			1,
		},
		{
			"with and multi space",
			"abc def     \nnolint:xxx ghi",
			"nolint",
			"abc def",
			1,
		},
		{
			"without",
			"abc def \nghi",
			"nolint",
			"abc def \nghi",
			0,
		},
		{
			"with multiple",
			"abc\nnolint:xxx\ndef\nnolint:xxx\nghi",
			"nolint",
			"abc\ndef\nghi",
			2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have, hN := removeLines(tc.in, tc.remove)

			// --- Then ---
			assert.Equal(t, tc.wantStr, have)
			assert.Equal(t, tc.wantNum, hN)
		})
	}
}

func Test_targetSynopsis_tabular(t *testing.T) {
	tt := []struct {
		testN string

		doc  string
		want string
	}{
		{"sentence", "does stuff.", "does stuff"},
		{"no period", "does stuff", "does stuff"},
		{"empty", "", ""},
		{"single word", "FnName", "FnName"},
		{"first sentence", "does stuff.\nA lot of stuff.", "does stuff"},
		{
			"first of several",
			"does stuff. Some more stuff.\nA lot of stuff.",
			"does stuff",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := targetSynopsis(tc.doc)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_breadcrumbs(t *testing.T) {
	t.Run("broken type", func(t *testing.T) {
		// --- Given ---
		absPath := modkit.Path("testdata/projects/ns_crossfile/project")
		tst := NewTestHelper(t, ring.New(), absPath)

		typ := tst.docPkg.Types[0]
		typ.Decl.Specs = typ.Decl.Specs[:0] // Break the type.

		// --- When ---
		have := breadcrumbs(typ)

		// --- Then ---
		assert.Nil(t, have)
	})
}

func Test_breadcrumbs_tabular(t *testing.T) {
	absPath := modkit.Path("testdata/projects/ns_crossfile/project")
	tst := NewTestHelper(t, ring.New(), absPath)

	tt := []struct {
		testN string

		ns   string
		want []string
	}{
		{"NS0", "NS0", []string{rootCrumb, "NS0"}},
		{"NS1", "NS1", []string{rootCrumb, "NS0", "NS1"}},
		{"NS2", "NS2", []string{rootCrumb, "NS0", "NS1", "NS2"}},
		{"NS3", "NS3", []string{partCrumb, "NS0", "NS3"}},
		{"NS4", "NS4", []string{partCrumb, "NS0", "NS3", "NS4"}},
		{"NOT0", "NOT0", nil},
		{"NOT1", "NOT1", nil},
		{"NOT2", "NOT2", nil},
		{"NOT3", "NOT3", nil},
		{"NOT4", "NOT4", nil},
		{"NOT5", "NOT5", []string{partCrumb, "NOT2", "NOT5"}},
		{"NOT6", "NOT6", nil},
		{"KebabCase", "KebabCase", []string{rootCrumb, "KebabCase"}},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			typ := tst.Type(tc.ns)

			// --- When ---
			have := breadcrumbs(typ)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_isNS(t *testing.T) {
	t.Run("cyclic type chain", func(t *testing.T) {
		// The child process builds a cyclic type chain and calls isNS.
		// Without the cycle guard the recursion overflows the stack and the
		// child dies; the guard makes it return and the child exits cleanly.
		if os.Getenv("GOMAKE_ISNS_CYCLE") == "1" {
			const src = "package p\n\ntype A B\n\ntype B A\n"
			set := token.NewFileSet()
			file, err := goparser.ParseFile(
				set,
				"cycle.go",
				src,
				goparser.ParseComments,
			)
			if err != nil {
				panic(err)
			}

			var spc *ast.TypeSpec
			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, sp := range gd.Specs {
					ts, ok := sp.(*ast.TypeSpec)
					if ok && ts.Name.Name == "A" {
						spc = ts
					}
				}
			}
			isNS(spc, []string{spc.Name.Name})
			return
		}

		// --- Given ---
		args := []string{"-test.run=^Test_isNS$"}
		cmd := exec.CommandContext(t.Context(), os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GOMAKE_ISNS_CYCLE=1")

		// --- When ---
		out, err := cmd.CombinedOutput()

		// --- Then ---
		assert.NoError(t, err, string(out))
	})
}

func Test_isNSRoot(t *testing.T) {
	t.Run("is ns root", func(t *testing.T) {
		// --- Given ---
		ts := &ast.TypeSpec{
			Comment: &ast.CommentGroup{
				List: []*ast.Comment{
					{Text: "//gomake:ns_root"},
				},
			},
		}

		// --- When ---
		have := isNSRoot(ts)

		// --- Then ---
		assert.True(t, have)
	})

	t.Run("not root", func(t *testing.T) {
		// --- Given ---
		ts := &ast.TypeSpec{
			Comment: &ast.CommentGroup{
				List: []*ast.Comment{
					{Text: "// comment"},
				},
			},
		}

		// --- When ---
		have := isNSRoot(ts)

		// --- Then ---
		assert.False(t, have)
	})

	t.Run("empty comment", func(t *testing.T) {
		// --- Given ---
		ts := &ast.TypeSpec{
			Comment: &ast.CommentGroup{
				List: []*ast.Comment{
					{Text: "//"},
				},
			},
		}

		// --- When ---
		have := isNSRoot(ts)

		// --- Then ---
		assert.False(t, have)
	})

	t.Run("nils handled", func(t *testing.T) {
		assert.False(t, isNSRoot(nil))
		assert.False(t, isNSRoot(&ast.TypeSpec{}))
		assert.False(t, isNSRoot(&ast.TypeSpec{Comment: nil}))
		assert.False(t, isNSRoot(&ast.TypeSpec{Comment: &ast.CommentGroup{}}))
	})

	t.Run("doc comment", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		src := "" +
			"package p\n" +
			"\n" +
			"// Foo groups targets.\n" +
			"//gomake:ns_root\n" +
			"type Foo struct{}\n"
		pth := oskit.Write(t, src, dir, "p.go")
		_, fls := must.Values(astFiles(dir, []string{pth}))
		spc := fls[pth].Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)

		// --- When ---
		have := isNSRoot(spc)

		// --- Then ---
		assert.True(t, have)
	})
}

func Test_attachDeclDoc(t *testing.T) {
	t.Run("single spec", func(t *testing.T) {
		// --- Given ---
		src := "" +
			"package p\n" +
			"\n" +
			"// Doc on import.\n" +
			"import _ \"fmt\"\n" +
			"\n" +
			"//gomake:ns_root\n" +
			"type Foo struct{}\n"
		fil := must.Value(goparser.ParseFile(
			token.NewFileSet(), "a.go", src, goparser.ParseComments,
		))

		// --- When ---
		attachDeclDoc(fil)

		// --- Then ---
		typ := fil.Decls[1].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
		assert.Equal(t, []string{"gomake:ns_root"}, commentBodies(typ.Doc))
		imp := fil.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ImportSpec)
		assert.Equal(t, []string{"Doc on import."}, commentBodies(imp.Doc))
	})

	t.Run("grouped specs", func(t *testing.T) {
		// --- Given ---
		src := "" +
			"package p\n" +
			"\n" +
			"// Group doc.\n" +
			"type (\n" +
			"\tA struct{}\n" +
			"\tB struct{}\n" +
			")\n"
		fil := must.Value(goparser.ParseFile(
			token.NewFileSet(), "a.go", src, goparser.ParseComments,
		))

		// --- When ---
		attachDeclDoc(fil)

		// --- Then ---
		typ := fil.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
		assert.Nil(t, typ.Doc)
	})

	t.Run("nil file", func(t *testing.T) {
		// --- When ---
		attachDeclDoc(nil)
	})
}

func Test_commentBodies(t *testing.T) {
	t.Run("comments", func(t *testing.T) {
		// --- Given ---
		grp := &ast.CommentGroup{List: []*ast.Comment{
			{Text: "//gomake:import ns"},
			{Text: "//  spaced text "},
		}}

		// --- When ---
		have := commentBodies(grp)

		// --- Then ---
		assert.Equal(t, []string{"gomake:import ns", "spaced text"}, have)
	})

	t.Run("nil group", func(t *testing.T) {
		// --- When ---
		have := commentBodies(nil)

		// --- Then ---
		assert.Nil(t, have)
	})
}

func Test_toKebabCase_tabular(t *testing.T) {
	tt := []struct {
		in   string
		want string
	}{
		{"TargetName", "target-name"},
		{"AATargetName", "aa-target-name"},
		{"TargetBBName", "target-bb-name"},
		{"AATargetBName", "aa-target-b-name"},
		{"AATargetBBName", "aa-target-bb-name"},
		{"TargetName1", "target-name1"},
		{"Target1Name", "target1-name"},
		{"Target1Name1", "target1-name1"},
		{"TargetName1aa", "target-name1-aa"},
		{"targetName", "target-name"},
		{"aaTargetName", "aa-target-name"},
	}

	for _, tc := range tt {
		t.Run(tc.in, func(t *testing.T) {
			// --- When ---
			have := toKebabCase(tc.in)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_importLocalNames(t *testing.T) {
	// --- Given ---
	src := "" +
		"package main\n" +
		"import (\n" +
		"\ta \"example.com/one/git\"\n" +
		"\t\"example.com/pkg\"\n" +
		"\t_ \"example.com/blank\"\n" +
		")\n"
	fil := must.Value(goparser.ParseFile(
		token.NewFileSet(), "x.go", src, goparser.ImportsOnly,
	))

	// --- When ---
	have := importLocalNames(fil)

	// --- Then ---
	want := map[string]string{
		"a":   "example.com/one/git",
		"pkg": "example.com/pkg",
	}
	assert.Equal(t, want, have)
}

func Test_topLevelNames(t *testing.T) {
	// --- Given ---
	src := "" +
		"package main\n" +
		"\n" +
		"import x \"example.com/x\"\n" +
		"\n" +
		"type T struct{}\n" +
		"\n" +
		"func (T) M() {}\n" +
		"\n" +
		"func F() {}\n" +
		"\n" +
		"var a, b = 1, 2\n" +
		"\n" +
		"const c = 3\n"
	fil := must.Value(goparser.ParseFile(token.NewFileSet(), "a.go", src, 0))

	// --- When ---
	have := topLevelNames(map[string]*ast.File{"a.go": fil})

	// --- Then ---
	assert.Equal(t, []string{"T", "F", "a", "b", "c"}, have)
}

func Test_fileImports(t *testing.T) {
	t.Run("file containing pos", func(t *testing.T) {
		// --- Given ---
		set := token.NewFileSet()
		srcA := "package main\nimport x \"example.com/a\"\n"
		filA := must.Value(goparser.ParseFile(set, "a.go", srcA, 0))
		srcB := "package main\nimport x \"example.com/b\"\n"
		filB := must.Value(goparser.ParseFile(set, "b.go", srcB, 0))
		files := map[string]*ast.File{"a.go": filA, "b.go": filB}

		// --- When ---
		have := fileImports(files, filB.Name.Pos())

		// --- Then ---
		assert.Equal(t, map[string]string{"x": "example.com/b"}, have)
	})

	t.Run("no file contains pos", func(t *testing.T) {
		// --- Given ---
		set := token.NewFileSet()
		src := "package main\nimport x \"example.com/a\"\n"
		fil := must.Value(goparser.ParseFile(set, "a.go", src, 0))
		files := map[string]*ast.File{"a.go": fil}

		// --- When ---
		have := fileImports(files, token.NoPos)

		// --- Then ---
		assert.Nil(t, have)
	})
}

func Test_gmImpSpec(t *testing.T) {
	t.Run("error - two namespace words", func(t *testing.T) {
		// --- Given ---
		src := "" +
			"package p\n" +
			"import _ \"example.com/x\" //gomake:import ns mx\n"
		fil := must.Value(goparser.ParseFile(
			token.NewFileSet(), "a.go", src, goparser.ParseComments,
		))

		// --- When ---
		hNS, hSpec, err := gmImpSpec(fil.Imports[0])

		// --- Then ---
		assert.ErrorIs(t, ErrImportTag, err)
		assert.ErrorContain(t, `"example.com/x"`, err)
		assert.Equal(t, "", hNS)
		assert.Equal(t, "", hSpec)
	})

	t.Run("above the spec", func(t *testing.T) {
		// --- Given ---
		src := "" +
			"package p\n" +
			"import (\n" +
			"\t//gomake:import ns\n" +
			"\t_ \"example.com/bar\"\n" +
			")\n"
		set := token.NewFileSet()
		fil := must.Value(goparser.ParseFile(
			set, "x.go", src, goparser.ParseComments,
		))

		// --- When ---
		hNS, hSpec, err := gmImpSpec(fil.Imports[0])

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "ns", hNS)
		assert.Equal(t, "example.com/bar", hSpec)
	})

	t.Run("line comment wins", func(t *testing.T) {
		// --- Given ---
		is := &ast.ImportSpec{
			Path: &ast.BasicLit{Value: `"example.com/bar"`},
			Doc: &ast.CommentGroup{List: []*ast.Comment{
				{Text: "//gomake:import doc"},
			}},
			Comment: &ast.CommentGroup{List: []*ast.Comment{
				{Text: "//gomake:import line"},
			}},
		}

		// --- When ---
		hNS, hSpec, err := gmImpSpec(is)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "line", hNS)
		assert.Equal(t, "example.com/bar", hSpec)
	})

	t.Run("above a single import", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		src := "" +
			"package p\n" +
			"\n" +
			"//gomake:import ns\n" +
			"import _ \"example.com/bar\"\n"
		pth := oskit.Write(t, src, dir, "p.go")
		_, fls := must.Values(astFiles(dir, []string{pth}))

		// --- When ---
		hNS, hSpec, err := gmImpSpec(fls[pth].Imports[0])

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "ns", hNS)
		assert.Equal(t, "example.com/bar", hSpec)
	})

	t.Run("nil spec", func(t *testing.T) {
		// --- When ---
		hNS, hSpec, err := gmImpSpec(nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", hNS)
		assert.Equal(t, "", hSpec)
	})
}

func Test_gmImpSpec_tabular(t *testing.T) {
	absPath := modkit.Path("testdata/projects/showcase_imports/project")
	iss := NewTestHelper(t, ring.New(), absPath).
		ImportSpecs(mkf.MakefileMain)

	tt := []struct {
		testN string

		index    int
		wantNS   string
		wantSpec string
	}{
		{
			"not gomake import 1",
			0,
			"",
			"",
		},
		{
			"not gomake import 2",
			1,
			"",
			"",
		},
		{
			"ring",
			2,
			"",
			"",
		},
		{
			"gomake namespaced import 0",
			3,
			"",
			"github.com/ctx42/gomake/testdata/imports/pkg0",
		},
		{
			"gomake namespaced import 1",
			4,
			"ns",
			"github.com/ctx42/gomake/testdata/imports/pkg1",
		},
		{
			"gomake namespaced import 2",
			5,
			"mx",
			"github.com/ctx42/gomake/testdata/imports/pkg2",
		},
		{
			"gomake namespaced import 9",
			6,
			"abc",
			"github.com/ctx42/gomake/testdata/imports/pkg9",
		},
		{
			"invalid empty comment",
			7,
			"",
			"",
		},
		{
			"invalid incomplete gomake import comment 1",
			8,
			"",
			"",
		},
		{
			"invalid incomplete gomake import comment 2",
			9,
			"",
			"",
		},
		{
			"invalid misspelled tag",
			10,
			"",
			"",
		},
		{
			"invalid not gomake import comment",
			11,
			"",
			"",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			hNS, hSpec, err := gmImpSpec(iss[tc.index])

			// --- Then ---
			assert.NoError(t, err)
			assert.Equal(t, tc.wantSpec, hSpec)
			assert.Equal(t, tc.wantNS, hNS)
		})
	}
}

func Test_unquote_tabular(t *testing.T) {
	tt := []struct {
		testN string

		val  string
		want string
	}{
		{"empty", "", ""},
		{"double quotes", `"str"`, "str"},
		{"backticks", "`str`", "str"},
		{"inner backtick", "`s`str`", "s`str"},
		{"inner quote", `"s"str"`, "s\"str"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			bl := &ast.BasicLit{Value: tc.val}

			// --- When ---
			have := unquote(bl)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_gmImpPackages(t *testing.T) {
	t.Run("imports", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		relPath := "testdata/projects/showcase_imports/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		astFil := NewTestHelper(t, rng, prj.Root()).File(mkf.MakefileMain)

		// --- When ---
		have, err := gmImpPackages(rng, "", astFil.Decls...)

		// --- Then ---
		assert.NoError(t, err)

		prj.ChdirBack()
		pkg := have[0]
		assert.Equal(t, "", pkg.PkgNS)
		relPath = "testdata/imports/pkg0"
		absPath := modkit.Path(relPath)
		impSpec := gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg0", pkg.Name)
		assert.Equal(t, []string{"file0.go"}, pkg.Files)

		pkg = have[1]
		assert.Equal(t, "ns", pkg.PkgNS)
		relPath = "testdata/imports/pkg1"
		absPath = modkit.Path(relPath)
		impSpec = gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg1", pkg.Name)
		assert.Equal(t, []string{"file0.go"}, pkg.Files)

		pkg = have[2]
		assert.Equal(t, "mx", pkg.PkgNS)
		relPath = "testdata/imports/pkg2"
		absPath = modkit.Path(relPath)
		impSpec = gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg2", pkg.Name)
		assert.Equal(t, []string{"file0.go", "file1.go"}, pkg.Files)

		pkg = have[3]
		assert.Equal(t, "abc", pkg.PkgNS)
		relPath = "testdata/imports/pkg9"
		absPath = modkit.Path(relPath)
		impSpec = gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg9", pkg.Name)
		assert.Equal(t, []string{"file0.go"}, pkg.Files)
		assert.Len(t, 4, have)
	})

	t.Run("imports GOOS windows", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("GOOS", "windows")

		relPath := "testdata/projects/showcase_imports/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		astFil := NewTestHelper(t, rng, prj.Root()).File(mkf.MakefileMain)

		// --- When ---
		have, err := gmImpPackages(rng, "", astFil.Decls...)

		// --- Then ---
		assert.NoError(t, err)

		prj.ChdirBack()
		pkg := have[0]
		assert.Equal(t, "", pkg.PkgNS)
		relPath = "testdata/imports/pkg0"
		absPath := modkit.Path(relPath)
		impSpec := gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg0", pkg.Name)
		assert.Equal(t, []string{"file0.go"}, pkg.Files)

		pkg = have[1]
		assert.Equal(t, "ns", pkg.PkgNS)
		relPath = "testdata/imports/pkg1"
		absPath = modkit.Path(relPath)
		impSpec = gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg1", pkg.Name)
		assert.Equal(t, []string{"file0.go"}, pkg.Files)

		pkg = have[2]
		assert.Equal(t, "mx", pkg.PkgNS)
		relPath = "testdata/imports/pkg2"
		absPath = modkit.Path(relPath)
		impSpec = gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg2", pkg.Name)
		assert.Equal(t, []string{"file0.go", "file1.go"}, pkg.Files)

		pkg = have[3]
		assert.Equal(t, "abc", pkg.PkgNS)
		relPath = "testdata/imports/pkg9"
		absPath = modkit.Path(relPath)
		impSpec = gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg9", pkg.Name)
		assert.Equal(t, []string{"file0.go", "file1_windows.go"}, pkg.Files)
		assert.Len(t, 4, have)
	})

	t.Run("same import twice", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.Write(t, "module example.com/mk\n\ngo 1.26\n", dir, "go.mod")
		lib := oskit.MkdirAll(t, dir, "lib")
		oskit.Write(t, "package lib\n", lib, "lib.go")

		src := "" +
			"package main\n" +
			"\n" +
			"import \"example.com/mk/lib\" //gomake:import\n"
		set := token.NewFileSet()
		mode := goparser.ParseComments
		filA := must.Value(goparser.ParseFile(set, "a.go", src, mode))
		filB := must.Value(goparser.ParseFile(set, "b.go", src, mode))
		decls := append(filA.Decls, filB.Decls...)

		// --- When ---
		have, err := gmImpPackages(ring.New(), dir, decls...)

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 1, have)
		assert.Equal(t, "example.com/mk/lib", have[0].ImpSpec)
	})
}

func Test_findDefault(t *testing.T) {
	t.Run("no default target", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/showcase_imports/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		vls := NewTestHelper(t, ring.New(), prj.Root()).docPkg.Vars

		// --- When ---
		have, hPos := findDefault(vls...)

		// --- Then ---
		assert.Nil(t, have)
		assert.Equal(t, token.NoPos, hPos)
	})

	t.Run("default target from local package", func(t *testing.T) {
		// --- Given ---
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path("testdata/projects/default_local/project"))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		vls := NewTestHelper(t, ring.New(), prj.Root()).docPkg.Vars

		// --- When ---
		have, hPos := findDefault(vls...)

		// --- Then ---
		assert.Equal(t, []string{"Hello"}, have)
		assert.True(t, hPos.IsValid())
	})

	t.Run("target from imported package", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/default_from_imp/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		vls := NewTestHelper(t, ring.New(), prj.Root()).docPkg.Vars

		// --- When ---
		have, hPos := findDefault(vls...)

		// --- Then ---
		assert.Equal(t, []string{"pkg1", "Pkg1"}, have)
		assert.True(t, hPos.IsValid())
	})

	t.Run("target from local namespace", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/default_from_ns/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		vls := NewTestHelper(t, ring.New(), prj.Root()).docPkg.Vars

		// --- When ---
		have, hPos := findDefault(vls...)

		// --- Then ---
		assert.Equal(t, []string{"NS", "Hello"}, have)
		assert.True(t, hPos.IsValid())
	})

	t.Run("default declared without a value", func(t *testing.T) {
		// --- Given ---
		// Represents "var Default Target" with no initializer.
		spec := &ast.ValueSpec{
			Names: []*ast.Ident{{Name: "Default"}},
			Type:  &ast.Ident{Name: "Target"},
		}
		val := &doc.Value{
			Names: []string{"Default"},
			Decl:  &ast.GenDecl{Specs: []ast.Spec{spec}},
		}

		// --- When ---
		have, hPos := findDefault(val)

		// --- Then ---
		assert.Nil(t, have)
		assert.Equal(t, token.NoPos, hPos)
	})
}

func Test_codeRef_tabular(t *testing.T) {
	absPath := modkit.Path("testdata/projects/expressions/project")
	tst := NewTestHelper(t, ring.New(), absPath)

	specs := make([]*ast.ValueSpec, 3)
	for i := range specs {
		decl := tst.docPkg.Vars[i].Decl.Specs[0]
		specs[i] = decl.(*ast.ValueSpec) //nolint:forcetypeassert
	}
	tt := []struct {
		testN string

		expr ast.Expr
		want []string
	}{
		{
			"identifier",
			specs[0].Values[0],
			[]string{"Hello"},
		},
		{
			"package selector",
			specs[1].Values[0],
			[]string{"pkg0", "Pkg0"},
		},
		{
			"package namespace method",
			specs[2].Values[0],
			[]string{"pkg4", "NS", "M0"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := codeRef(tc.expr, nil)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_qIdent(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		// --- Given ---
		expr := &ast.Ident{
			NamePos: 0,
			Name:    "string",
			Obj:     nil,
		}

		// --- When ---
		have := qIdent(expr)

		// --- Then ---
		assert.Equal(t, "string", have)
	})

	t.Run("string array", func(t *testing.T) {
		// --- Given ---
		expr := &ast.ArrayType{
			Elt: &ast.Ident{
				NamePos: 0,
				Name:    "string",
				Obj:     nil,
			},
		}

		// --- When ---
		have := qIdent(expr)

		// --- Then ---
		assert.Equal(t, "[]string", have)
	})

	t.Run("time Duration", func(t *testing.T) {
		// --- Given ---
		expr := &ast.SelectorExpr{
			X:   ast.NewIdent("time"),
			Sel: ast.NewIdent("Duration"),
		}

		// --- When ---
		have := qIdent(expr)

		// --- Then ---
		assert.Equal(t, "time.Duration", have)
	})

	t.Run("unknown", func(t *testing.T) {
		// --- Given ---
		expr := &ast.SelectorExpr{
			X: &ast.SelectorExpr{
				X:   ast.NewIdent("a"),
				Sel: ast.NewIdent("b"),
			},
			Sel: ast.NewIdent("c"),
		}

		// --- When ---
		have := qIdent(expr)

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_CreateFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		dst := filepath.Join(t.TempDir(), "src.go")

		// --- When ---
		err := CreateFile(dst, []byte("content"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "content", oskit.ReadFileStr(t, dst))
	})

	t.Run("error - parent directory missing", func(t *testing.T) {
		// --- Given ---
		dst := filepath.Join(t.TempDir(), "not_existing", "src.go")

		// --- When ---
		err := CreateFile(dst, []byte("content"))

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.NoFileExist(t, dst)
	})
}
