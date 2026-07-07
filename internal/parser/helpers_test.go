// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"go/ast"
	"go/doc"
	goparser "go/parser"
	"go/token"
	"os"
	"os/exec"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/goldy"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"

	gmt "github.com/ctx42/gomake/internal/cli/clitest"
	"github.com/ctx42/gomake/internal/mkf"
)

func Test_GenMakefileUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/showcase_imports/project"

		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		rng := SetBuildTag(ring.New())

		// --- When ---
		code, tgs, err := genMakefileUser(rng, prj.Root())

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
		assert.Equal(t, wantNames, tgs.Names())

		prj.ChdirBack()
		gfp := "testdata/mkf_user_main.gld"
		gfd := map[string]any{"gmk_root": modkit.Root(), "prj_root": prj.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))
		assert.Equal(t, gld.String(), string(code))
	})

	t.Run("duplicated targets error", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/dup_imported/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		rng := SetBuildTag(ring.New())

		// --- When ---
		code, tgs, err := genMakefileUser(rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.Nil(t, tgs)
		assert.Nil(t, code)
	})
}

func Test_GenMakefileUserAndSave(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/no_targets/project"
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(modkit.Path(relPath))
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := SetBuildTag(ring.New())
		dst := prj.Path(mkf.MakefileUser)

		// --- When ---
		tgs, err := GenMakefileUserAndSave(rng, prj.Root(), dst)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 0, tgs.Len())

		gfp := "testdata/gen_no_targets_main_init.gld"
		gfd := map[string]any{"prj_root": modkit.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))
		assert.Equal(t, gld.String(), oskit.ReadFileStr(t, dst))
	})

	t.Run("error generating code", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/dup_imported/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := SetBuildTag(ring.New())
		dst := prj.Path(mkf.MakefileUser)

		// --- When ---
		tgs, err := GenMakefileUserAndSave(rng, prj.Root(), dst)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.Nil(t, tgs)
	})

	t.Run("error writing file", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/no_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := SetBuildTag(ring.New())
		dst := t.TempDir()

		// --- When ---
		tgs, err := GenMakefileUserAndSave(rng, prj.Root(), dst)

		// --- Then ---
		var e *os.PathError
		assert.ErrorAs(t, &e, err)
		assert.Equal(t, dst, e.Path)
		assert.Nil(t, tgs)
	})
}

func Test_SetBuildTag_GetBuildTag_RemBuildTag(t *testing.T) {
	// --- Given ---
	rng0 := ring.New()

	// --- Then ---
	assert.Empty(t, GetBuildTag(rng0))

	// --- When ---
	rng1 := SetBuildTag(rng0)

	// --- Then ---
	assert.Equal(t, rng0.MetaAll(), rng1.MetaAll())

	// --- Then ---
	assert.Equal(t, BuildTag, GetBuildTag(rng1))

	// --- When ---
	rng2 := RemBuildTag(rng1)

	// --- Then ---
	assert.Empty(t, GetBuildTag(rng1))
	assert.Empty(t, GetBuildTag(rng2))
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

func Test_removeNoLintComments_tabular(t *testing.T) {
	t.Run("doc string with nolint", func(t *testing.T) {
		// --- Given ---
		doc := "abc\ndef\nnolint:xxx\nghi"

		// --- When ---
		have := removeNoLintComments(doc)

		// --- Then ---
		assert.Equal(t, "abc\ndef\nghi", have)
	})

	t.Run("doc string without nolint", func(t *testing.T) {
		// --- Given ---
		doc := "abc\ndef\nghi"

		// --- When ---
		have := removeNoLintComments(doc)

		// --- Then ---
		assert.Equal(t, "abc\ndef\nghi", have)
	})
}

func Test_isHidden(t *testing.T) {
	t.Run("hidden target", func(t *testing.T) {
		// --- Given ---
		doc := "abc\n\ngomake:hidden\ndef"

		// --- When ---
		haveDoc, haveHidden := isHidden(doc)

		// --- Then ---
		assert.Equal(t, "abc\n\ndef", haveDoc)
		assert.True(t, haveHidden)
	})

	t.Run("not hidden target", func(t *testing.T) {
		// --- Given ---
		doc := "abc\ndef\nghi"

		// --- When ---
		haveDoc, haveHidden := isHidden(doc)

		// --- Then ---
		assert.Equal(t, "abc\ndef\nghi", haveDoc)
		assert.False(t, haveHidden)
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
			have, n := removeLines(tc.in, tc.remove)

			// --- Then ---
			assert.Equal(t, tc.wantStr, have)
			assert.Equal(t, tc.wantNum, n)
		})
	}
}

func Test_targetSynopsis_tabular(t *testing.T) {
	tt := []struct {
		testN string

		name string
		doc  string
		want string
	}{
		{"1", "FnName", "FnName does stuff.", "does stuff"},
		{"2", "FnName", "FnName does stuff", "does stuff"},
		{"3", "OtherName", "FnName does stuff.", "FnName does stuff"},
		{"4", "", "FnName does stuff.", "FnName does stuff"},
		{"5", "FnName", "", ""},
		{"6", "", "FnName", "FnName"},
		{"7", "FnName", "FnName does stuff.\nA lot of stuff.", "does stuff"},
		{
			"8",
			"FnName",
			"FnName does stuff. Some more stuff.\nA lot of stuff.",
			"does stuff",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := targetSynopsis(tc.name, tc.doc)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_breadcrumbs(t *testing.T) {
	t.Run("invalid type", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		relPath := "testdata/projects/ns_crossfile/project"
		absPath := modkit.Path(relPath)
		tst := NewTestHelper(t, rng, absPath)

		typ := tst.Types()[0]
		typ.Decl.Specs = typ.Decl.Specs[:0] // Break the type.

		// --- When ---
		have := breadcrumbs(typ)

		// --- Then ---
		assert.Nil(t, have)
	})
}

func Test_breadcrumbs_tabular(t *testing.T) {
	rng := ring.New()
	relPath := "testdata/projects/ns_crossfile/project"
	absPath := modkit.Path(relPath)
	tst := NewTestHelper(t, rng, absPath)

	tt := []struct {
		testN string

		ns   string
		want []string
	}{
		{"1", "NS0", []string{rootCrumb, "NS0"}},
		{"2", "NS1", []string{rootCrumb, "NS0", "NS1"}},
		{"3", "NS2", []string{rootCrumb, "NS0", "NS1", "NS2"}},
		{"4", "NS3", []string{partCrumb, "NS0", "NS3"}},
		{"5", "NS4", []string{partCrumb, "NS0", "NS3", "NS4"}},
		{"6", "NOT0", nil},
		{"7", "NOT1", nil},
		{"8", "NOT2", nil},
		{"9", "NOT3", nil},
		{"10", "NOT4", nil},
		{"11", "NOT5", []string{partCrumb, "NOT2", "NOT5"}},
		{"12", "NOT6", nil},
		{"13", "KebabCase", []string{rootCrumb, "KebabCase"}},
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
	t.Run("cyclic type chain does not overflow", func(t *testing.T) {
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
		exe := os.Args[0]
		env := append(os.Environ(), "GOMAKE_ISNS_CYCLE=1")

		// --- When ---
		cmd := exec.Command(exe, "-test.run=^Test_isNS$")
		cmd.Env = env
		out, err := cmd.CombinedOutput()

		// --- Then ---
		if err != nil {
			t.Log(string(out))
		}
		assert.NoError(t, err)
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
			assert.Equal(t, tc.want, toKebabCase(tc.in))
		})
	}
}

func Test_gmImpSpec_tabular(t *testing.T) {
	rng := ring.New()
	relPath := "testdata/projects/showcase_imports/project"
	absPath := modkit.Path(relPath)
	iss := NewTestHelper(t, rng, absPath).
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
			"invalid more than one gomake namespace",
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
			haveNS, haveSpec := gmImpSpec(iss[tc.index])

			// --- Then ---
			assert.Equal(t, tc.wantSpec, haveSpec)
			assert.Equal(t, tc.wantNS, haveNS)
		})
	}
}

func Test_unquote_tabular(t *testing.T) {
	tt := []struct {
		testN string

		val  string
		want string
	}{
		{"1", "", ""},
		{"2", `"str"`, "str"},
		{"3", "`str`", "str"},
		{"4", "`s`str`", "s`str"},
		{"5", `"s"str"`, "s\"str"},
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
		relPath := "testdata/projects/showcase_imports/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		rng := ring.New()
		astFil := NewTestHelper(t, rng, prj.Root()).File(mkf.MakefileMain)

		// --- When ---
		pks, err := gmImpPackages(rng, astFil.Decls...)

		// --- Then ---
		assert.NoError(t, err)
		prj.ChdirBack()

		pkg := pks[0]
		assert.Equal(t, "", pkg.PkgNS)
		relPath = "testdata/imports/pkg0"
		absPath := modkit.Path(relPath)
		impSpec := gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg0", pkg.Name)
		assert.Equal(t, []string{"file0.go"}, pkg.Files)

		pkg = pks[1]
		assert.Equal(t, "ns", pkg.PkgNS)
		relPath = "testdata/imports/pkg1"
		absPath = modkit.Path(relPath)
		impSpec = gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg1", pkg.Name)
		assert.Equal(t, []string{"file0.go"}, pkg.Files)

		pkg = pks[2]
		assert.Equal(t, "mx", pkg.PkgNS)
		relPath = "testdata/imports/pkg2"
		absPath = modkit.Path(relPath)
		impSpec = gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg2", pkg.Name)
		assert.Equal(t, []string{"file0.go", "file1.go"}, pkg.Files)

		pkg = pks[3]
		assert.Equal(t, "abc", pkg.PkgNS)
		relPath = "testdata/imports/pkg9"
		absPath = modkit.Path(relPath)
		impSpec = gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg9", pkg.Name)
		assert.Equal(t, []string{"file0.go"}, pkg.Files)

		assert.Len(t, 4, pks)
	})

	t.Run("imports GOOS windows", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/showcase_imports/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		astFil := NewTestHelper(t, rng, prj.Root()).File(mkf.MakefileMain)

		// --- When ---
		pks, err := gmImpPackages(rng, astFil.Decls...)

		// --- Then ---
		assert.NoError(t, err)
		prj.ChdirBack()

		pkg := pks[0]
		assert.Equal(t, "", pkg.PkgNS)
		relPath = "testdata/imports/pkg0"
		absPath := modkit.Path(relPath)
		impSpec := gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg0", pkg.Name)
		assert.Equal(t, []string{"file0.go"}, pkg.Files)

		pkg = pks[1]
		assert.Equal(t, "ns", pkg.PkgNS)
		relPath = "testdata/imports/pkg1"
		absPath = modkit.Path(relPath)
		impSpec = gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg1", pkg.Name)
		assert.Equal(t, []string{"file0.go"}, pkg.Files)

		pkg = pks[2]
		assert.Equal(t, "mx", pkg.PkgNS)
		relPath = "testdata/imports/pkg2"
		absPath = modkit.Path(relPath)
		impSpec = gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg2", pkg.Name)
		assert.Equal(t, []string{"file0.go", "file1.go"}, pkg.Files)

		pkg = pks[3]
		assert.Equal(t, "abc", pkg.PkgNS)
		relPath = "testdata/imports/pkg9"
		absPath = modkit.Path(relPath)
		impSpec = gmt.JoinImpSpec(t, gmt.GmModName, relPath)
		assert.Equal(t, absPath, pkg.ImpPath)
		assert.Equal(t, impSpec, pkg.ImpSpec)
		assert.Equal(t, "pkg9", pkg.Name)
		assert.Equal(t, []string{"file0.go", "file1_windows.go"}, pkg.Files)

		assert.Len(t, 4, pks)
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

		rng := ring.New()
		vls := NewTestHelper(t, rng, prj.Root()).Values()

		// --- When ---
		ref := findDefault(vls...)

		// --- Then ---
		assert.Nil(t, ref)
	})

	t.Run("default target from local package", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/default_local/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		rng := ring.New()
		vls := NewTestHelper(t, rng, prj.Root()).Values()

		// --- When ---
		ref := findDefault(vls...)

		// --- Then ---
		assert.Equal(t, []string{"Hello"}, ref)
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

		rng := ring.New()
		vls := NewTestHelper(t, rng, prj.Root()).Values()

		// --- When ---
		ref := findDefault(vls...)

		// --- Then ---
		assert.Equal(t, []string{"pkg1", "Pkg1"}, ref)
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

		rng := ring.New()
		vls := NewTestHelper(t, rng, prj.Root()).Values()

		// --- When ---
		ref := findDefault(vls...)

		// --- Then ---
		assert.Equal(t, []string{"NS", "Hello"}, ref)
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
		ref := findDefault(val)

		// --- Then ---
		assert.Nil(t, ref)
	})
}

func Test_codeRef_tabular(t *testing.T) {
	rng := ring.New()
	relPath := "testdata/projects/expressions/project"
	absPath := modkit.Path(relPath)
	tst := NewTestHelper(t, rng, absPath)

	// False positive lint error.
	//nolint:forcetypeassert
	tt := []struct {
		testN string

		expr ast.Expr
		want []string
	}{
		{
			"var A = Hello",
			tst.Values()[0].Decl.Specs[0].(*ast.ValueSpec).Values[0],
			[]string{"Hello"},
		},
		{
			"var B = pkg0.Pkg0",
			tst.Values()[1].Decl.Specs[0].(*ast.ValueSpec).Values[0],
			[]string{"pkg0", "Pkg0"},
		},
		{
			"var C = pkg4.NS.M0",
			tst.Values()[2].Decl.Specs[0].(*ast.ValueSpec).Values[0],
			[]string{"pkg4", "NS", "M0"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			assert.Equal(t, tc.want, codeRef(tc.expr, nil))
		})
	}
}

func Test_qIdent(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		// --- Given ---
		var expr ast.Expr
		expr = &ast.Ident{
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
		var expr ast.Expr
		expr = &ast.ArrayType{
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

	t.Run("time.Duration", func(t *testing.T) {
		// --- Given ---
		var expr ast.Expr
		expr = &ast.SelectorExpr{
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
		var expr ast.Expr
		expr = &ast.SelectorExpr{
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
