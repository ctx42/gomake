// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"go/build/constraint"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/exekit"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"

	"github.com/ctx42/gomake/internal/builtin"
	gmt "github.com/ctx42/gomake/internal/cli/clitest"
	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/internal/parser"
	"github.com/ctx42/gomake/pkg/gomake"
)

func Test_expandHome_tabular(t *testing.T) {
	tt := []struct {
		testN string

		pth  string
		home string
		want string
	}{
		{"bare tilde", "~", "/home/u", "/home/u"},
		{"tilde slash", "~/a/b", "/home/u", "/home/u/a/b"},
		{"tilde only prefix not slash", "~foo", "/home/u", "~foo"},
		{"no tilde", "a/b", "/home/u", "a/b"},
		{"absolute path", "/etc/x", "/home/u", "/etc/x"},
		{"empty", "", "/home/u", ""},
		{"tilde mid path", "a/~/b", "/home/u", "a/~/b"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			assert.Equal(t, tc.want, expandHome(tc.pth, tc.home))
		})
	}
}

func Test_homeDir(t *testing.T) {
	t.Run("ring value", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("HOME", "/ring/home")
		rng.EnvSet("USERPROFILE", "/ring/home")
		rng.EnvSet("home", "/ring/home")

		// --- When ---
		have, err := homeDir(rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/ring/home", have)
	})

	t.Run("error - unset", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("HOME", "")
		rng.EnvSet("USERPROFILE", "")
		rng.EnvSet("home", "")

		// --- When ---
		_, err := homeDir(rng)

		// --- Then ---
		assert.ErrorContain(t, "is not defined", err)
	})
}

func Test_errCompile_Unwrap(t *testing.T) {
	// --- When ---
	err := &errCompile{error: ErrTest}

	// --- Then ---
	assert.Same(t, ErrTest, err.Unwrap())
}

func Test_errCompile_Error(t *testing.T) {
	t.Run("standard error set", func(t *testing.T) {
		// --- Given ---
		err := &errCompile{eout: "eout", error: ErrTest}

		// --- When ---
		have := err.Error()

		// --- Then ---
		assert.Equal(t, "eout\ntest error", have)
	})

	t.Run("standard output set", func(t *testing.T) {
		// --- Given ---
		err := &errCompile{sout: "sout", error: ErrTest}

		// --- When ---
		have := err.Error()

		// --- Then ---
		assert.Equal(t, "sout\ntest error", have)
	})

	t.Run("standard output and error set", func(t *testing.T) {
		// --- Given ---
		err := &errCompile{sout: "sout", eout: "eout", error: ErrTest}

		// --- When ---
		have := err.Error()

		// --- Then ---
		assert.Equal(t, "eout\ntest error", have)
	})

	t.Run("outputs empty", func(t *testing.T) {
		// --- Given ---
		err := &errCompile{error: ErrTest}

		// --- When ---
		have := err.Error()

		// --- Then ---
		assert.Equal(t, "test error", have)
	})

	t.Run("zero value is safe", func(t *testing.T) {
		// --- Given ---
		err := &errCompile{}

		// --- When ---
		have := err.Error()

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_ignoreWarning(t *testing.T) {
	// --- When ---
	have := ignoreWarning("makefile_x.go")

	// --- Then ---
	assert.Contain(t, `gomake: ignoring "makefile_x.go": `, have)
}

func Test_validMakefileName_tabular(t *testing.T) {
	tt := []struct {
		testN string

		name string
		want bool
	}{
		{"main", "makefile.go", true},
		{"goos", "makefile_linux.go", true},
		{"goarch", "makefile_amd64.go", true},
		{"goos and goarch", "makefile_linux_amd64.go", true},
		{"another goos", "makefile_plan9.go", true},
		{"custom suffix", "makefile_db.go", false},
		{"reversed order", "makefile_amd64_linux.go", false},
		{"dot form", "makefile.linux.go", false},
		{"extra token", "makefile_linux_amd64_x.go", false},
		{"wrong case", "makefile_Linux.go", false},
		{"unknown token", "makefile_nope.go", false},
		{"empty token", "makefile_.go", false},
		{"not a go file", "makefile_linux.txt", false},
		{"reserved gen", "makefile_gen.go", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := validMakefileName(tc.name)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_selectMakefiles(t *testing.T) {
	t.Run("keeps valid names and collects ignored", func(t *testing.T) {
		// --- Given ---
		files := []string{
			"/dir/main.go",
			"/dir/makefile.go",
			"/dir/makefile_linux.go",
			"/dir/makefile_db.go",
			"/dir/makefile.extra.go",
		}

		// --- When ---
		hKeep, hIgnored := selectMakefiles(files)

		// --- Then ---
		wKeep := []string{"/dir/makefile.go", "/dir/makefile_linux.go"}
		assert.Equal(t, wKeep, hKeep)
		wIgnored := []string{"makefile_db.go", "makefile.extra.go"}
		assert.Equal(t, wIgnored, hIgnored)
	})

	t.Run("no makefile-looking files", func(t *testing.T) {
		// --- Given ---
		items := []string{"/dir/main.go"}

		// --- When ---
		hKeep, hIgnored := selectMakefiles(items)

		// --- Then ---
		assert.Nil(t, hKeep)
		assert.Nil(t, hIgnored)
	})
}

func Test_prepare(t *testing.T) {
	t.Run("cleans build dir on error after mkdir", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os_build_tag/project"
		srcPrj := prjkit.New(t, modkit.Path(relPath))
		srcPrj.Close()

		dstPth := oskit.MkdirTemp(t, "", "project")
		dstPrj := prjkit.New(t, dstPth)
		dstPrj.Close()

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "386")
		rng = parser.SetBuildTag(rng)

		unreadable := filepath.Join(srcPrj.Root(), "makefile.go")
		assert.NoError(t, os.Chmod(unreadable, 0))
		t.Cleanup(func() { _ = os.Chmod(unreadable, 0644) })

		// --- When ---
		have, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		assert.ErrorIs(t, os.ErrPermission, err)
		assert.Nil(t, have)
		entries, rerr := os.ReadDir(dstPrj.Root())
		assert.NoError(t, rerr)
		for _, e := range entries {
			assert.False(t, strings.HasPrefix(e.Name(), "gomake-"))
		}
	})

	t.Run("success", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os_build_tag/project"
		srcPrj := prjkit.New(t, modkit.Path(relPath))
		srcPrj.Close()

		dstPth := oskit.MkdirTemp(t, "", "project")
		dstPrj := prjkit.New(t, dstPth)
		dstPrj.Close()

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "386")
		rng = parser.SetBuildTag(rng)

		// --- When ---
		have, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, dstPrj.Root(), filepath.Dir(have.BuildDir))
		assert.DirExist(t, have.BuildDir)
		assert.True(
			t,
			strings.HasPrefix(filepath.Base(have.BuildDir), "gomake-"),
		)
		bd := have.BuildDir
		assert.Equal(t, filepath.Join(bd, mkf.MakefileGen), have.MainGen)
		assert.Equal(t, filepath.Join(bd, mkf.MakefileBin), have.MainBin)
		assert.Equal(t, filepath.Join(bd, mkf.MakefileUser), have.MainUser)
		want := []string{
			mkf.MakefileMain,
			"makefile_386.go",
			"makefile_windows.go",
			"makefile_windows_386.go",
		}
		assert.Equal(t, want, have.MkfNames)

		assert.NotContain(t, parser.BuildTagLine,
			oskit.ReadFileStr(t, filepath.Join(bd, mkf.MakefileMain)))
		assert.NotContain(t, parser.BuildTagLine,
			oskit.ReadFileStr(t, filepath.Join(bd, "makefile_386.go")))
		assert.NotContain(t, parser.BuildTagLine,
			oskit.ReadFileStr(t, filepath.Join(bd, "makefile_windows.go")))
		assert.NotContain(t, parser.BuildTagLine,
			oskit.ReadFileStr(t, filepath.Join(bd, "makefile_windows_386.go")))
		assert.NoFileExist(t, filepath.Join(bd, mkf.MakefileGen))
		assert.NotContain(t, parser.BuildTagLine,
			oskit.ReadFileStr(t, filepath.Join(bd, mkf.MakefileUser)))
		assert.FileExist(t, filepath.Join(bd, "go.mod"))
		assert.FileExist(t, filepath.Join(bd, "go.sum"))
	})

	t.Run("no makefiles in src", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/no_makefiles/project"
		srcPrj := prjkit.New(t, modkit.Path(relPath))
		srcPrj.Close()

		dstPth := oskit.MkdirTemp(t, "", "project")
		dstPrj := prjkit.New(t, dstPth)
		dstPrj.Close()

		// --- When ---
		have, err := prepare(ring.New(), dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		assert.ErrorIs(t, errNoMakefile, err)
		assert.ErrorContain(t, srcPrj.Root(), err)
		assert.Nil(t, have)
	})

	t.Run("no makefile in src GOOS and build tag set", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/no_makefile/project"
		srcPrj := prjkit.New(t, modkit.Path(relPath))
		srcPrj.Close()

		dstPth := oskit.MkdirTemp(t, "", "project")
		dstPrj := prjkit.New(t, dstPth)
		dstPrj.Close()

		rng := ring.New()
		rng.EnvSet("GOOS", "linux")
		rng = parser.SetBuildTag(rng)

		// --- When ---
		have, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		assert.ErrorIs(t, errNoMakefile, err)
		assert.ErrorContain(t, srcPrj.Root(), err)
		assert.Nil(t, have)
	})

	t.Run("no destination directory error", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os_build_tag/project"
		srcPrj := prjkit.New(t, modkit.Path(relPath))
		srcPrj.Close()

		dstPth := oskit.MkdirTemp(t, "", "project")
		dstPrj := prjkit.New(t, dstPth)
		dstPrj.Close()

		dstPath := dstPrj.Path("not_existing")
		rng := parser.SetBuildTag(ring.New())

		// --- When ---
		have, err := prepare(rng, dstPath, srcPrj.Root())

		// --- Then ---
		var e *fs.PathError
		assert.ErrorAs(t, &e, err)
		assert.Contain(t, dstPath, e.Path)
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.Nil(t, have)
	})

	t.Run("mkf.MakefileGen cannot be in sources", func(t *testing.T) {
		// --- Given ---
		srcPth := oskit.MkdirTemp(t, "", "project")
		srcPrj := prjkit.New(t, srcPth)
		srcPrj.GoModInit()
		srcPrj.CreateFileWith("package makefile", mkf.MakefileMain)
		srcPrj.CreateFileWith("package makefile", mkf.MakefileGen)
		srcPrj.Close()

		dstPth := oskit.MkdirTemp(t, "", "project")
		dstPrj := prjkit.New(t, dstPth)
		dstPrj.CreateDir("rnd")
		dstPrj.Close()

		rng := parser.SetBuildTag(ring.New())

		// --- When ---
		have, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		wMsg := "source directory must not contain \"makefile_gen.go\" file"
		assert.ErrorEqual(t, wMsg, err)
		assert.Nil(t, have)
	})

	t.Run("mkf.MakefileBin cannot be in sources", func(t *testing.T) {
		// --- Given ---
		srcPth := oskit.MkdirTemp(t, "", "project")
		srcPrj := prjkit.New(t, srcPth)
		srcPrj.GoModInit()
		srcPrj.CreateFileWith("package makefile", mkf.MakefileMain)
		srcPrj.CreateFileWith("package makefile", mkf.MakefileBin)
		srcPrj.Close()

		dstPth := oskit.MkdirTemp(t, "", "project")
		dstPrj := prjkit.New(t, dstPth)
		dstPrj.Close()

		rng := parser.SetBuildTag(ring.New())

		// --- When ---
		have, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		wMsg := "source directory must not contain \"makefile\" file"
		assert.ErrorEqual(t, wMsg, err)
		assert.Nil(t, have)
	})

	t.Run("mkf.MakefileUser cannot be in sources", func(t *testing.T) {
		// --- Given ---
		srcPth := oskit.MkdirTemp(t, "", "project")
		srcPrj := prjkit.New(t, srcPth)
		srcPrj.GoModInit()
		srcPrj.CreateFileWith("package makefile", mkf.MakefileMain)
		srcPrj.CreateFileWith("package makefile", mkf.MakefileUser)
		srcPrj.Close()

		dstPth := oskit.MkdirTemp(t, "", "project")
		dstPrj := prjkit.New(t, dstPth)
		dstPrj.Close()

		rng := parser.SetBuildTag(ring.New())

		// --- When ---
		have, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		wMsg := "source directory must not contain \"makefile_user.go\" file"
		assert.ErrorEqual(t, wMsg, err)
		assert.Nil(t, have)
	})

	t.Run("src must be absolute path", func(t *testing.T) {
		// --- Given ---
		srcPth := "../../testdata/projects/arch_os_build_tag/project"

		dstPth := oskit.MkdirTemp(t, "", "project")
		dstPrj := prjkit.New(t, dstPth)
		dstPrj.Close()

		rng := parser.SetBuildTag(ring.New())

		// --- When ---
		have, err := prepare(rng, dstPrj.Root(), srcPth)

		// --- Then ---
		assert.ErrorIs(t, parser.ErrAbsPath, err)
		assert.Nil(t, have)
	})

	t.Run("no go.mod file in source", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/simple_tagged/project"
		srcPth := oskit.MkdirTemp(t, "", "project")
		srcPrj := prjkit.New(t, srcPth)
		srcPrj.FileFrom(modkit.Path(relPath, mkf.MakefileMain))
		srcPrj.Close()

		dstPth := oskit.MkdirTemp(t, "", "project")
		dstPrj := prjkit.New(t, dstPth)
		dstPrj.Close()

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "386")
		rng = parser.SetBuildTag(rng)

		// --- When ---
		have, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		assert.ErrorIs(t, gomake.ErrNoGoMod, err)
		assert.ErrorContain(t, srcPrj.Root(), err)
		assert.Nil(t, have)
	})

	t.Run("no go.sum file in source", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/simple_tagged/project"
		srcPth := oskit.MkdirTemp(t, "", "project")
		srcPrj := prjkit.New(t, srcPth)
		srcPrj.FileFrom(modkit.Path(relPath, mkf.MakefileMain))
		srcPrj.GoModInit()
		srcPrj.Close()

		dstPth := oskit.MkdirTemp(t, "", "project")
		dstPrj := prjkit.New(t, dstPth)
		dstPrj.Close()

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "386")
		rng = parser.SetBuildTag(rng)

		// --- When ---
		have, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, dstPrj.Root(), filepath.Dir(have.BuildDir))
		assert.DirExist(t, have.BuildDir)
		bd := have.BuildDir
		assert.Equal(t, filepath.Join(bd, mkf.MakefileGen), have.MainGen)
		assert.Equal(t, filepath.Join(bd, mkf.MakefileBin), have.MainBin)
		assert.Equal(t, filepath.Join(bd, mkf.MakefileUser), have.MainUser)
		assert.Equal(t, []string{mkf.MakefileMain}, have.MkfNames)
		assert.FileExist(t, filepath.Join(bd, mkf.MakefileMain))
		assert.FileExist(t, filepath.Join(bd, "go.mod"))

		// The source has no go.sum, but gomake records the xflag checksum
		// (the generated makefile imports xflag), so one is created.
		goSum := oskit.ReadFileStr(t, filepath.Join(bd, "go.sum"))
		assert.Contain(t, "github.com/ctx42/xflag", goSum)
	})
}

func Test_isGomakeConstraintLine_tabular(t *testing.T) {
	tt := []struct {
		testN string

		line string
		want bool
	}{
		{"go build", "//go:build gomake", true},
		{"compound", "//go:build gomake && linux", true},
		{"negated", "//go:build !gomake", true},
		{"second of or", "//go:build linux || gomake", true},
		{"legacy", "// +build gomake", true},
		{"legacy list", "// +build linux,gomake", true},
		{"tag prefix", "//go:build gomakeci", false},
		{"other tag", "//go:build linux", false},
		{"comment", "// gomake rules", false},
		{"code", "x := gomake", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := isGomakeConstraintLine([]byte(tc.line))

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_hasTag_tabular(t *testing.T) {
	tt := []struct {
		testN string

		expr string
		want bool
	}{
		{"tag", "gomake", true},
		{"not", "!gomake", true},
		{"and", "linux && gomake", true},
		{"or", "linux || gomake", true},
		{"absent", "linux && !windows", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			expr := must.Value(constraint.Parse("//go:build " + tc.expr))

			// --- When ---
			have := hasTag(expr, "gomake")

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_stripBuildTag_tabular(t *testing.T) {
	tt := []struct {
		test string
		in   string
		want string
	}{
		{
			"lf at start",
			"//go:build gomake\n\npackage main\n",
			"package main\n",
		},
		{
			"crlf at start",
			"//go:build gomake\r\n\r\npackage main\r\n",
			"package main\n",
		},
		{
			"crlf after header",
			"// copyright\r\n//go:build gomake\r\n\r\npackage main\r\n",
			"// copyright\npackage main\n",
		},
		{
			"no tag",
			"package main\n",
			"package main\n",
		},
	}

	for _, tc := range tt {
		t.Run(tc.test, func(t *testing.T) {
			// --- When ---
			have := stripBuildTag([]byte(tc.in))

			// --- Then ---
			assert.Equal(t, tc.want, string(have))
		})
	}
}

func Test_findGoWork(t *testing.T) {
	t.Run("at module root", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "go 1.22\nuse .\n", root, "go.work")

		// --- When ---
		have, err := findGoWork(ring.New(), root)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(root, "go.work"), have)
	})

	t.Run("in parent directory", func(t *testing.T) {
		// --- Given ---
		base := t.TempDir()
		oskit.Write(t, "go 1.22\nuse .\n", base, "go.work")
		mod := oskit.MkdirAll(t, base, "mod")

		// --- When ---
		have, err := findGoWork(ring.New(), mod)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(base, "go.work"), have)
	})

	t.Run("GOWORK absolute", func(t *testing.T) {
		// --- Given ---
		base := t.TempDir()
		work := oskit.Write(t, "go 1.22\nuse .\n", base, "custom.work")
		mod := oskit.MkdirAll(t, base, "mod")
		env := ring.New()
		env.EnvSet("GOWORK", work)

		// --- When ---
		have, err := findGoWork(env, mod)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, work, have)
	})

	t.Run("GOWORK relative uses process cwd", func(t *testing.T) {
		// --- Given ---
		base := t.TempDir()
		oskit.Write(t, "go 1.22\nuse .\n", base, "go.work")
		mod := oskit.MkdirAll(t, base, "mod")

		// Relative GOWORK is resolved against cwd, not modRoot.
		cwd := must.Value(os.Getwd())
		t.Cleanup(func() { _ = os.Chdir(cwd) })
		must.Nil(os.Chdir(base))

		env := ring.New()
		env.EnvSet("GOWORK", "go.work")

		// --- When ---
		have, err := findGoWork(env, mod)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(base, "go.work"), have)
	})

	t.Run("GOWORK off", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "go 1.22\nuse .\n", root, "go.work")

		env := ring.New()
		env.EnvSet("GOWORK", "off")

		// --- When ---
		have, err := findGoWork(env, root)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", have)
	})

	t.Run("error - GOWORK path missing", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		env := ring.New()
		env.EnvSet("GOWORK", filepath.Join(root, "nope.work"))

		// --- When ---
		have, err := findGoWork(env, root)

		// --- Then ---
		assert.ErrorContain(t, "GOWORK", err)
		assert.Equal(t, "", have)
	})

	t.Run("missing", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()

		// --- When ---
		have, err := findGoWork(ring.New(), root)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", have)
	})
}

func Test_editGoWork(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		env := ring.New()
		prjSrc := modkit.Path("testdata/projects/workspace/project")
		othSrc := modkit.Path("testdata/projects/workspace/other")

		// Prepare workspace destination directories. We copy them so we
		// can rename files without changing fixtures.
		wsPth := oskit.MkdirTemp(t, "", "workspace")
		othRoot := oskit.MkdirAll(t, wsPth, "other")
		prjRoot := oskit.MkdirAll(t, wsPth, "project")

		othPrj := prjkit.New(t, othRoot)
		othPrj.ProjectFrom(othSrc)
		othPrj.Rename("go.mod_", "go.mod")
		othPrj.Close()

		prjPrj := prjkit.New(t, prjRoot)
		prjPrj.ProjectFrom(prjSrc)
		prjPrj.Rename("go.mod_", "go.mod")
		prjPrj.Rename("go.work_", "go.work")
		prjPrj.Close()
		prjPrj.Compile()

		// Copy edited project to new location so "../other" path in
		// the `go.work` file is invalid to test it will be fixed by editGoWork.
		outPth := oskit.MkdirTemp(t, "", "project")
		outPrj := prjkit.New(t, outPth)
		outPrj.ProjectFrom(prjRoot)
		outPrj.Close()

		// --- When ---
		err := editGoWork(
			env,
			filepath.Join(prjRoot, "go.work"),
			outPth,
			prjRoot,
		)

		// --- Then ---
		assert.NoError(t, err)
		stderr := exekit.New(t).ExeStderr(outPrj.Compile())
		assert.Equal(t, "project called other\n", stderr)
	})

	t.Run("parent workspace rewrites use dot", func(t *testing.T) {
		// --- Given ---
		// go.work at monorepo root: use . (parent) and use ./project.
		// After copy into buildDir, "." must become the abs parent path.
		wsPth := oskit.MkdirTemp(t, "", "workspace")
		prjRoot := oskit.MkdirAll(t, wsPth, "project")

		oskit.Write(
			t,
			"module example.com/parent\n\ngo 1.23\n",
			wsPth,
			"go.mod",
		)

		oskit.Write(
			t,
			"module example.com/project\n\ngo 1.23\n",
			prjRoot,
			"go.mod",
		)

		srcWork := filepath.Join(wsPth, "go.work")
		oskit.Write(t, "go 1.23\n\nuse .\nuse ./project\n", srcWork)

		outPth := oskit.MkdirTemp(t, "", "project")
		oskit.Write(
			t,
			"module example.com/project\n\ngo 1.23\n",
			outPth,
			"go.mod",
		)
		// Destination starts with a copy of the parent workfile.
		oskit.Write(t, "go 1.23\n\nuse .\nuse ./project\n", outPth, "go.work")

		// --- When ---
		err := editGoWork(ring.New(), srcWork, outPth, prjRoot)

		// --- Then ---
		assert.NoError(t, err)
		text := oskit.ReadFileStr(t, filepath.Join(outPth, "go.work"))
		// go work edit may emit a use ( ... ) block; parent is absolute,
		// project is ".".
		assert.Contain(t, wsPth, text)
		assert.Contain(t, "\t.\n", text)
		assert.NotContain(t, "./project", text)
	})

	t.Run("does not mutate source when GOWORK set", func(t *testing.T) {
		// --- Given ---
		prjSrc := modkit.Path("testdata/projects/workspace/project")
		othSrc := modkit.Path("testdata/projects/workspace/other")

		wsPth := oskit.MkdirTemp(t, "", "workspace")
		othRoot := oskit.MkdirAll(t, wsPth, "other")
		prjRoot := oskit.MkdirAll(t, wsPth, "project")

		othPrj := prjkit.New(t, othRoot)
		othPrj.ProjectFrom(othSrc)
		othPrj.Rename("go.mod_", "go.mod")
		othPrj.Close()

		prjPrj := prjkit.New(t, prjRoot)
		prjPrj.ProjectFrom(prjSrc)
		prjPrj.Rename("go.mod_", "go.mod")
		prjPrj.Rename("go.work_", "go.work")
		prjPrj.Close()

		srcWork := filepath.Join(prjRoot, "go.work")
		before := oskit.ReadFileStr(t, srcWork)

		outPth := oskit.MkdirTemp(t, "", "project")
		outPrj := prjkit.New(t, outPth)
		outPrj.ProjectFrom(prjRoot)
		outPrj.Close()

		// Ambient GOWORK points at the source workfile — edits must not
		// follow it and rewrite the caller's workspace.
		env := ring.New()
		env.EnvSet("GOWORK", srcWork)

		// --- When ---
		err := editGoWork(env, srcWork, outPth, prjRoot)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, before, oskit.ReadFileStr(t, srcWork))
		// Destination was rewritten (absolute sibling path).
		dstWork := oskit.ReadFileStr(t, filepath.Join(outPth, "go.work"))
		assert.NotEqual(t, before, dstWork)
		assert.Contain(t, "use ", dstWork)
	})

	t.Run("no go.work in source", func(t *testing.T) {
		// --- Given ---
		srcPth := oskit.MkdirTemp(t, "", "project")
		dstPth := oskit.MkdirTemp(t, "", "project")

		// --- When ---
		err := editGoWork(
			ring.New(),
			filepath.Join(srcPth, "go.work"),
			dstPth,
			srcPth,
		)

		// --- Then ---
		assert.ErrorIs(t, errGoWorkEdit, err)
		assert.ErrorContain(t, srcPth, err)
	})

	t.Run("no relative paths", func(t *testing.T) {
		// --- Given ---
		env := ring.New()

		srcPth := oskit.MkdirTemp(t, "", "project")
		srcPrj := prjkit.New(t, srcPth)
		srcPrj.GoModInit()
		_, _ = srcPrj.Exe("go", "work", "init", ".")
		srcPrj.Close()

		dstPth := oskit.MkdirTemp(t, "", "project")

		// --- When ---
		err := editGoWork(env, filepath.Join(srcPth, "go.work"), dstPth, srcPth)

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("absolutizes dot-slash use paths", func(t *testing.T) {
		// --- Given ---
		env := ring.New()
		wsPth := oskit.MkdirTemp(t, "", "workspace")
		othRoot := oskit.MkdirAll(t, wsPth, "other")
		prjRoot := oskit.MkdirAll(t, wsPth, "project")

		othPrj := prjkit.New(t, othRoot)
		othPrj.ProjectFrom(modkit.Path("testdata/projects/workspace/other"))
		othPrj.Rename("go.mod_", "go.mod")
		othPrj.Close()

		prjPrj := prjkit.New(t, prjRoot)
		prjPrj.ProjectFrom(modkit.Path("testdata/projects/workspace/project"))
		prjPrj.Rename("go.mod_", "go.mod")

		// use ./../other is unusual; use a path with ./ prefix that still
		// points at the sibling via a cleaned relative form.
		oskit.Write(t, "go 1.23\n\nuse .\nuse ./../other\n", prjRoot, "go.work")
		prjPrj.Close()

		outPth := oskit.MkdirTemp(t, "", "project")
		outPrj := prjkit.New(t, outPth)
		outPrj.ProjectFrom(prjRoot)
		outPrj.Close()

		// --- When ---
		err := editGoWork(
			env,
			filepath.Join(prjRoot, "go.work"),
			outPth,
			prjRoot,
		)

		// --- Then ---
		assert.NoError(t, err)
		stderr := exekit.New(t).ExeStderr(outPrj.Compile())
		assert.Equal(t, "project called other\n", stderr)
	})

	t.Run("no go.work in destination", func(t *testing.T) {
		// --- Given ---
		env := ring.New()
		prjSrc := modkit.Path("testdata/projects/workspace/project")
		othSrc := modkit.Path("testdata/projects/workspace/other")

		wsPth := oskit.MkdirTemp(t, "", "workspace")
		othRoot := oskit.MkdirAll(t, wsPth, "other")
		prjRoot := oskit.MkdirAll(t, wsPth, "project")

		othPrj := prjkit.New(t, othRoot)
		othPrj.ProjectFrom(othSrc)
		othPrj.Rename("go.mod_", "go.mod")
		othPrj.Close()

		prjPrj := prjkit.New(t, prjRoot)
		prjPrj.ProjectFrom(prjSrc)
		prjPrj.Rename("go.mod_", "go.mod")
		prjPrj.Rename("go.work_", "go.work")
		prjPrj.Close()

		dstPth := oskit.MkdirTemp(t, "", "project")

		// --- When ---
		err := editGoWork(
			env,
			filepath.Join(prjRoot, "go.work"),
			dstPth,
			prjRoot,
		)

		// --- Then ---
		assert.ErrorIs(t, errGoWorkEdit, err)
		assert.ErrorContain(t, dstPth, err)
		assert.ErrorContain(t, "go:", err) // Underlying toolchain diagnostic.
	})
}

func Test_editGoMod(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		inData := oskit.ReadFile(t, "testdata/go.mod_in")
		pth := oskit.Write(t, inData, t.TempDir(), "go.mod")
		wantData := oskit.ReadFileStr(t, "testdata/go.mod_want")

		// --- When ---
		err := editGoMod(
			ring.New(),
			pth,
			"example.com/user/repo",
			"/module/path",
			filepath.Dir(pth),
		)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, wantData, oskit.ReadFileStr(t, pth))
	})

	t.Run("no go.mod file", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Write(t, "", t.TempDir(), "not-go.mod")

		// --- When ---
		err := editGoMod(
			ring.New(),
			pth,
			"example.com/user/repo",
			"/module/path",
			filepath.Dir(pth),
		)

		// --- Then ---
		assert.ErrorIs(t, errGoModEdit, err)
		assert.ErrorContain(t, pth, err)
		assert.ErrorContain(t, "go:", err) // Underlying toolchain diagnostic.
	})
}

func Test_xflagFallbackVer(t *testing.T) {
	// --- Given ---
	modPth := filepath.Join(modkit.Root(), "go.mod")
	want := must.Value(modkit.ModVer(modPth, xflagModPath))

	// --- When ---
	have := xflagFallbackVer

	// --- Then ---
	assert.Equal(t, want, have)
}

func Test_xflagVersion(t *testing.T) {
	// --- Given ---
	modPth := filepath.Join(modkit.Root(), "go.mod")
	want := must.Value(modkit.ModVer(modPth, xflagModPath))

	// --- When ---
	have := xflagVersion()

	// --- Then ---
	assert.Equal(t, want, have)
}

func Test_findGoWorkValue(t *testing.T) {
	t.Run("off", func(t *testing.T) {
		// --- When ---
		have, err := findGoWorkValue("off", t.TempDir())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", have)
	})

	t.Run("explicit file", func(t *testing.T) {
		// --- Given ---
		work := oskit.Write(t, "go 1.26\n", t.TempDir(), "go.work")

		// --- When ---
		have, err := findGoWorkValue(work, t.TempDir())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, work, have)
	})

	t.Run("error - explicit file missing", func(t *testing.T) {
		// --- Given ---
		work := filepath.Join(t.TempDir(), "go.work")

		// --- When ---
		have, err := findGoWorkValue(work, t.TempDir())

		// --- Then ---
		assert.ErrorContain(t, "no such file", err)
		assert.Equal(t, "", have)
	})

	t.Run("found in parent", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		work := oskit.Write(t, "go 1.26\n", root, "go.work")
		mod := oskit.MkdirAll(t, root, "mod")

		// --- When ---
		have, err := findGoWorkValue("", mod)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, work, have)
	})
}

func Test_absolutizeGoModReplaces(t *testing.T) {
	// --- Given ---
	src := t.TempDir()
	build := t.TempDir()
	mod := "module example.com/m\n\nreplace example.com/x => ./x\n"
	pth := oskit.Write(t, mod, build, "go.mod")

	// --- When ---
	err := absolutizeGoModReplaces(os.Environ(), build, src)

	// --- Then ---
	assert.NoError(t, err)
	want := "example.com/x => " + filepath.Join(src, "x")
	assert.FileContain(t, want, pth)
}

func Test_pinBuildGOWORK(t *testing.T) {
	t.Run("workspace file", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		work := oskit.Write(t, "go 1.26\n", dir, "go.work")

		// --- When ---
		have := pinBuildGOWORK(nil, dir)

		// --- Then ---
		assert.Equal(t, []string{"GOWORK=" + work}, have)
	})

	t.Run("no workspace file", func(t *testing.T) {
		// --- When ---
		have := pinBuildGOWORK([]string{"GOWORK=x"}, t.TempDir())

		// --- Then ---
		assert.Equal(t, []string{"GOWORK=off"}, have)
	})
}

func Test_goEditErr(t *testing.T) {
	t.Run("uses trimmed toolchain output as detail", func(t *testing.T) {
		// --- When ---
		err := goEditErr(errGoModEdit, "/b", "  go: boom\n", ErrTest)

		// --- Then ---
		assert.ErrorIs(t, errGoModEdit, err)
		assert.ErrorIs(t, ErrTest, err)
		want := "editing \"go.mod\" file at /b: go: boom: test error"
		assert.ErrorEqual(t, want, err)
	})

	t.Run("empty output falls back to raw error", func(t *testing.T) {
		// --- When ---
		err := goEditErr(errGoWorkEdit, "/b", "   ", ErrTest)

		// --- Then ---
		assert.ErrorIs(t, errGoWorkEdit, err)
		assert.ErrorEqual(t, "editing \"go.work\" file at /b: test error", err)
	})
}

func Test_compile(t *testing.T) {
	t.Run("compile", func(t *testing.T) {
		// --- Given ---
		prj := prjkit.New(t, oskit.MkdirTemp(t, "", "project"))
		prj.ProjectFrom(modkit.Path("testdata/compile/simple"))
		prj.Close()

		files := []string{
			"main.go",
			"helpers.go",
		}

		// --- When ---
		err := compile(
			t.Context(),
			os.Environ(),
			prj.Root(),
			prj.Path("a.out"),
			files...,
		)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "hello\n", prj.ExeStdout(prj.Path("a.out")))
	})

	t.Run("compile error", func(t *testing.T) {
		// --- Given ---
		prj := prjkit.New(t, oskit.MkdirTemp(t, "", "project"))
		prj.ProjectFrom(modkit.Path("testdata/compile/error"))
		prj.Close()

		// --- When ---
		err := compile(
			t.Context(),
			os.Environ(),
			prj.Root(),
			prj.Path("a.out"),
			parser.MainName,
		)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		var e *errCompile
		assert.ErrorAs(t, &e, err)
		assert.Empty(t, e.sout)
		assert.Contain(t, "package main is not in ", e.eout)
	})

	t.Run("error - deadline during the build", func(t *testing.T) {
		// --- Given ---
		prj := prjkit.New(t, oskit.MkdirTemp(t, "", "project"))
		prj.ProjectFrom(modkit.Path("testdata/compile/simple"))
		prj.Close()

		// A fresh GOCACHE makes the build outlast the deadline.
		env := ring.EnvSet(os.Environ(), "GOCACHE", t.TempDir())
		ctx, cxl := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cxl()

		// --- When ---
		err := compile(
			ctx,
			env,
			prj.Root(),
			prj.Path("a.out"),
			"main.go",
			"helpers.go",
		)

		// --- Then ---
		assert.ErrorIs(t, context.DeadlineExceeded, err)
		assert.Equal(t, mkf.ExitCodeErr, compileExit(err))
	})

	t.Run("error - canceled context stops the build", func(t *testing.T) {
		// --- Given ---
		prj := prjkit.New(t, oskit.MkdirTemp(t, "", "project"))
		prj.ProjectFrom(modkit.Path("testdata/compile/simple"))
		prj.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		files := []string{
			"main.go",
			"helpers.go",
		}

		// --- When ---
		err := compile(
			ctx,
			os.Environ(),
			prj.Root(),
			prj.Path("a.out"),
			files...,
		)

		// --- Then ---
		assert.ErrorIs(t, context.Canceled, err)
	})
}

func Test_allTargets(t *testing.T) {
	t.Run("bin only", func(t *testing.T) {
		// --- Given ---
		cfg := &config{bin: "/tmp/out"}

		// --- When ---
		have, err := allTargets(ring.New(), cfg, builtin.Empty().Targets())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 0, len(have))
	})

	t.Run("includes makefile targets", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/simple_untagged/project"
		absPath := modkit.Path(relPath)

		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		cfg, err := newConfig(
			"1.0",
			tst.Ring("--src", prj.Root(), "--tmp", prj.TempDir(), "--list"),
		)
		assert.NoError(t, err)

		gen := builtin.Generated().Targets()

		// --- When ---
		have, err := allTargets(tst.Ring(), cfg, gen)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, len(have) > len(gen))
	})
}

func Test_fileContainsStr(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "f")
		oskit.Write(t, "foo bar baz", p)
		ok, err := fileContainsStr(p, "bar")
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("absent", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "f")
		oskit.Write(t, "foo baz", p)
		ok, err := fileContainsStr(p, "bar")
		assert.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("file does not exist", func(t *testing.T) {
		ok, err := fileContainsStr("/nonexistent/path", "x")
		assert.False(t, ok)
		assert.ErrorIs(t, os.ErrNotExist, err)
	})
}

func Test_copyFile(t *testing.T) {
	// --- Given ---
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	oskit.Write(t, "data", src)

	// --- When ---
	err := copyFile(src, dst)

	// --- Then ---
	assert.NoError(t, err)
	assert.Equal(t, "data", oskit.ReadFileStr(t, dst))
}

func Test_copyFile_error_tabular(t *testing.T) {
	tt := []struct {
		testN string

		src string
	}{
		{
			"src missing",
			"missing",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			err := copyFile(
				filepath.Join(t.TempDir(), tc.src),
				filepath.Join(t.TempDir(), "dst"),
			)

			// --- Then ---
			assert.ErrorIs(t, os.ErrNotExist, err)
		})
	}
}
