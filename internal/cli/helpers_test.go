// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func Test_isCoreCmd_tabular(t *testing.T) {
	tt := []struct {
		testN string

		tgtName string
		want    bool
	}{
		{"1", "", false},
		{"2", "abc", false},
		{"3", ":abc", true},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			assert.Equal(t, tc.want, isCoreCmd(tc.tgtName))
		})
	}
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

func Test_makefiles(t *testing.T) {
	t.Run("files and env with build tags", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		relPath := "testdata/projects/arch_os_build_tag/project"
		absPath := modkit.Path(relPath)

		rng := tst.Ring()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "amd64")
		rng = parser.SetBuildTag(rng)

		// --- When ---
		fls, err := makefiles(rng, absPath)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			filepath.Join(absPath, mkf.MakefileMain),
			filepath.Join(absPath, "makefile_amd64.go"),
			filepath.Join(absPath, "makefile_windows.go"),
			filepath.Join(absPath, "makefile_windows_amd64.go"),
		}
		assert.Equal(t, want, fls)
	})

	t.Run("files with build tags env without", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os_build_tag/project"
		absPath := modkit.Path(relPath)

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "amd64")

		// --- When ---
		fls, err := makefiles(rng, absPath)

		// --- Then ---
		assert.ErrorIs(t, errNoMakefile, err)
		assert.ErrorContain(t, absPath, err)
		assert.Empty(t, fls)
	})

	t.Run("files without build tags env with", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os/project"
		absPath := modkit.Path(relPath)

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "amd64")
		rng = parser.SetBuildTag(rng)

		// --- When ---
		fls, err := makefiles(rng, absPath)

		// --- Then ---
		assert.ErrorIs(t, errNoMakefile, err)
		assert.ErrorContain(t, absPath, err)
		assert.Empty(t, fls)
	})

	t.Run("files and env without build tags", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os/project"
		absPath := modkit.Path(relPath)

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "amd64")

		// --- When ---
		fls, err := makefiles(rng, absPath)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			filepath.Join(absPath, mkf.MakefileMain),
			filepath.Join(absPath, "makefile_amd64.go"),
			filepath.Join(absPath, "makefile_windows.go"),
			filepath.Join(absPath, "makefile_windows_amd64.go"),
		}
		assert.Equal(t, want, fls)
	})

	t.Run("not existing directory error", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/not_existing"
		absPath := modkit.Path(relPath)

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "amd64")
		rng = parser.SetBuildTag(rng)

		// --- When ---
		fls, err := makefiles(rng, absPath)

		// --- Then ---
		assert.ErrorContain(t, "listing Go source files", err)
		assert.Nil(t, fls)
	})
}

func Test_filterMkf(t *testing.T) {
	t.Run("filters", func(t *testing.T) {
		// --- Given ---
		files := []string{
			"/dir/file.go",
			"/dir/makefile.go",
			"/dir/makefiles.go",
			"/dir/makefile_linux.go",
		}

		// --- When ---
		have := filterMkf(files)

		// --- Then ---
		want := []string{
			"/dir/makefile.go",
			"/dir/makefile_linux.go",
		}
		assert.Equal(t, want, have)
	})

	t.Run("empty", func(t *testing.T) {
		// --- When ---
		have := filterMkf(make([]string, 0))

		// --- Then ---
		assert.Empty(t, have)
		assert.Nil(t, have)
	})

	t.Run("nil", func(t *testing.T) {
		// --- When ---
		have := filterMkf(nil)

		// --- Then ---
		assert.Empty(t, have)
		assert.Nil(t, have)
	})
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
		keep, ignored := selectMakefiles(files)

		// --- Then ---
		wKeep := []string{"/dir/makefile.go", "/dir/makefile_linux.go"}
		assert.Equal(t, wKeep, keep)

		wIgnored := []string{"makefile_db.go", "makefile.extra.go"}
		assert.Equal(t, wIgnored, ignored)
	})

	t.Run("no makefile-looking files", func(t *testing.T) {
		// --- When ---
		keep, ignored := selectMakefiles([]string{"/dir/main.go"})

		// --- Then ---
		assert.Nil(t, keep)
		assert.Nil(t, ignored)
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
		cu, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		assert.Error(t, err)
		assert.Nil(t, cu)
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
		cu, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, srcPrj.Root(), cu.SourceDir)
		assert.Equal(t, dstPrj.Root(), cu.BuildRootDir)
		assert.DirExist(t, cu.BuildDir)
		assert.True(t, strings.HasPrefix(filepath.Base(cu.BuildDir), "gomake-"))
		bd := cu.BuildDir
		assert.Equal(t, filepath.Join(bd, mkf.MakefileGen), cu.MainGen)
		assert.Equal(t, filepath.Join(bd, mkf.MakefileBin), cu.MainBin)
		assert.Equal(t, filepath.Join(bd, mkf.MakefileUser), cu.MainUser)
		want := []string{
			filepath.Join(bd, mkf.MakefileMain),          // 0
			filepath.Join(bd, "makefile_386.go"),         // 1
			filepath.Join(bd, "makefile_windows.go"),     // 2
			filepath.Join(bd, "makefile_windows_386.go"), // 3
			filepath.Join(bd, mkf.MakefileGen),           // 4
			filepath.Join(bd, mkf.MakefileUser),          // 5
		}
		assert.Equal(t, want, cu.Files)
		assert.NotContain(t, buildTagLine,
			oskit.ReadFileStr(t, filepath.Join(bd, mkf.MakefileMain)))
		assert.NotContain(t, buildTagLine,
			oskit.ReadFileStr(t, filepath.Join(bd, "makefile_386.go")))
		assert.NotContain(t, buildTagLine,
			oskit.ReadFileStr(t, filepath.Join(bd, "makefile_windows.go")))
		assert.NotContain(t, buildTagLine,
			oskit.ReadFileStr(t, filepath.Join(bd, "makefile_windows_386.go")))
		assert.NoFileExist(t, filepath.Join(bd, mkf.MakefileGen))
		assert.NotContain(t, buildTagLine,
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

		rng := ring.New()

		// --- When ---
		cu, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		assert.ErrorIs(t, errNoMakefile, err)
		assert.ErrorContain(t, srcPrj.Root(), err)
		assert.Nil(t, cu)
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
		cu, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		assert.ErrorIs(t, errNoMakefile, err)
		assert.ErrorContain(t, srcPrj.Root(), err)
		assert.Nil(t, cu)
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
		cu, err := prepare(rng, dstPath, srcPrj.Root())

		// --- Then ---
		var e *fs.PathError
		assert.ErrorAs(t, &e, err)
		assert.Contain(t, dstPath, e.Path)
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.Nil(t, cu)
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
		cu, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		wMsg := "source directory must not contain \"makefile_gen.go\" file"
		assert.ErrorEqual(t, wMsg, err)
		assert.Nil(t, cu)
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
		cu, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		wMsg := "source directory must not contain \"makefile\" file"
		assert.ErrorEqual(t, wMsg, err)
		assert.Nil(t, cu)
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
		cu, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		wMsg := "source directory must not contain \"makefile_user.go\" file"
		assert.ErrorEqual(t, wMsg, err)
		assert.Nil(t, cu)
	})

	t.Run("src must be absolute path", func(t *testing.T) {
		// --- Given ---
		srcPth := "../../testdata/projects/arch_os_build_tag/project"

		dstPth := oskit.MkdirTemp(t, "", "project")
		dstPrj := prjkit.New(t, dstPth)
		dstPrj.Close()

		rng := parser.SetBuildTag(ring.New())

		// --- When ---
		cu, err := prepare(rng, dstPrj.Root(), srcPth)

		// --- Then ---
		assert.ErrorIs(t, parser.ErrAbsPath, err)
		assert.Nil(t, cu)
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
		cu, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		assert.ErrorIs(t, gomake.ErrNoGoMod, err)
		assert.ErrorContain(t, srcPrj.Root(), err)
		assert.Nil(t, cu)
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
		cu, err := prepare(rng, dstPrj.Root(), srcPrj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, srcPrj.Root(), cu.SourceDir)
		assert.Equal(t, dstPrj.Root(), cu.BuildRootDir)
		assert.DirExist(t, cu.BuildDir)
		bd := cu.BuildDir
		assert.Equal(t, filepath.Join(bd, mkf.MakefileGen), cu.MainGen)
		assert.Equal(t, filepath.Join(bd, mkf.MakefileBin), cu.MainBin)
		assert.Equal(t, filepath.Join(bd, mkf.MakefileUser), cu.MainUser)
		want := []string{
			filepath.Join(bd, mkf.MakefileMain), // 0
			filepath.Join(bd, mkf.MakefileGen),  // 1
			filepath.Join(bd, mkf.MakefileUser), // 2
		}
		assert.Equal(t, want, cu.Files)
		assert.FileExist(t, filepath.Join(bd, "go.mod"))

		// The source has no go.sum, but gomake records the xflag checksum
		// (the generated makefile imports xflag), so one is created.
		goSum := oskit.ReadFileStr(t, filepath.Join(bd, "go.sum"))
		assert.Contain(t, "github.com/ctx42/xflag", goSum)
	})
}

func Test_stripBuildTag_tabular(t *testing.T) {
	tag := []byte(buildTagLine)
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
			have := stripBuildTag([]byte(tc.in), tag)

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
		env := ring.New()

		// --- When ---
		have := findGoWork(env, root)

		// --- Then ---
		assert.Equal(t, filepath.Join(root, "go.work"), have)
	})

	t.Run("in parent directory", func(t *testing.T) {
		// --- Given ---
		base := t.TempDir()
		oskit.Write(t, "go 1.22\nuse .\n", base, "go.work")
		mod := oskit.MkdirAll(t, base, "mod")
		env := ring.New()

		// --- When ---
		have := findGoWork(env, mod)

		// --- Then ---
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
		have := findGoWork(env, mod)

		// --- Then ---
		assert.Equal(t, work, have)
	})

	t.Run("GOWORK off", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "go 1.22\nuse .\n", root, "go.work")
		env := ring.New()
		env.EnvSet("GOWORK", "off")

		// --- When ---
		have := findGoWork(env, root)

		// --- Then ---
		assert.Equal(t, "", have)
	})

	t.Run("missing", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		env := ring.New()

		// --- When ---
		have := findGoWork(env, root)

		// --- Then ---
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
		err := editGoWork(env, filepath.Join(prjRoot, "go.work"), outPth, prjRoot)

		// --- Then ---
		assert.NoError(t, err)
		have := exekit.New(t).ExeStderr(outPrj.Compile())
		assert.Equal(t, "project called other\n", have)
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
		env := ring.New()
		srcPth := oskit.MkdirTemp(t, "", "project")
		dstPth := oskit.MkdirTemp(t, "", "project")

		// --- When ---
		err := editGoWork(env, filepath.Join(srcPth, "go.work"), dstPth, srcPth)

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
		srcPrj.Exe("go", "work", "init", ".")
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
		err := editGoWork(env, filepath.Join(prjRoot, "go.work"), outPth, prjRoot)

		// --- Then ---
		assert.NoError(t, err)
		have := exekit.New(t).ExeStderr(outPrj.Compile())
		assert.Equal(t, "project called other\n", have)
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
		err := editGoWork(env, filepath.Join(prjRoot, "go.work"), dstPth, prjRoot)

		// --- Then ---
		assert.ErrorIs(t, errGoWorkEdit, err)
		assert.ErrorContain(t, dstPth, err)
		assert.ErrorContain(t, "go:", err) // Underlying toolchain diagnostic.
	})
}

func Test_editGoMod(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		env := ring.New()
		inData := oskit.ReadFile(t, "testdata/go.mod_in")
		pth := oskit.Write(t, inData, t.TempDir(), "go.mod")
		wantData := oskit.ReadFileStr(t, "testdata/go.mod_want")

		// --- When ---
		err := editGoMod(env, pth, "example.com/user/repo", "/module/path")

		// --- Then ---
		assert.NoError(t, err)
		haveData := oskit.ReadFileStr(t, pth)
		assert.Equal(t, wantData, haveData)
	})

	t.Run("no go.mod file", func(t *testing.T) {
		// --- Given ---
		env := ring.New()
		pth := oskit.Write(t, "", t.TempDir(), "not-go.mod")

		// --- When ---
		err := editGoMod(env, pth, "example.com/user/repo", "/module/path")

		// --- Then ---
		assert.ErrorIs(t, errGoModEdit, err)
		assert.ErrorContain(t, pth, err)
		assert.ErrorContain(t, "go:", err) // Underlying toolchain diagnostic.
	})
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

func Test_goEditErr(t *testing.T) {
	t.Run("uses trimmed toolchain output as detail", func(t *testing.T) {
		// --- When ---
		err := goEditErr(errGoModEdit, "/b", "  go: boom\n", ErrTest)

		// --- Then ---
		assert.ErrorIs(t, errGoModEdit, err)
		want := "editing \"go.mod\" file at /b: go: boom"
		assert.ErrorEqual(t, want, err)
	})

	t.Run("empty output falls back to raw error", func(t *testing.T) {
		// --- When ---
		err := goEditErr(errGoWorkEdit, "/b", "   ", ErrTest)

		// --- Then ---
		assert.ErrorIs(t, errGoWorkEdit, err)
		want := "editing \"go.work\" file at /b: test error"
		assert.ErrorEqual(t, want, err)
	})
}

func Test_compile(t *testing.T) {
	t.Run("compile", func(t *testing.T) {
		// --- Given ---
		prj := prjkit.New(t, oskit.MkdirTemp(t, "", "project"))
		prj.ProjectFrom(modkit.Path("testdata/compile/simple"))
		prj.Close()

		// --- When ---
		files := []string{
			"main.go",
			"helpers.go",
		}
		err := compile(
			context.Background(),
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
			context.Background(),
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

func Test_gmFiles(t *testing.T) {
	t.Run("list only files with build tag", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os_build_tag/project"
		prj := prjkit.New(t, modkit.Path(relPath))
		prj.Close()

		rng := ring.New()
		rng.EnvSet("GOOS", "darwin")
		rng.EnvSet("GOARCH", "amd64")
		rng = parser.SetBuildTag(rng)

		// --- When ---
		fls, err := gmFiles(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)

		exp := []string{
			prj.Path(mkf.MakefileMain),
			prj.Path("makefile_amd64.go"),
			prj.Path("makefile_darwin.go"),
			prj.Path("makefile_darwin_amd64.go"),
		}
		assert.Equal(t, exp, fls)
	})

	t.Run("unknown directory error", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/not/existing"
		prj := prjkit.New(t, modkit.Path(relPath))
		prj.Close()
		rng := parser.SetBuildTag(ring.New())

		// --- When ---
		fls, err := gmFiles(rng, prj.Root())

		// --- Then ---
		assert.ErrorContain(t, "listing Go source files", err)
		assert.Nil(t, fls)
	})

	t.Run("files in multiple packages", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/packages/multi"
		prj := prjkit.New(t, modkit.Path(relPath))
		prj.Close()
		rng := parser.SetBuildTag(ring.New())

		// --- When ---
		fls, err := gmFiles(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		exp := []string{
			prj.Path("main_file.go"), // The pkg.go is tagged.
		}
		assert.Equal(t, exp, fls)
	})

	t.Run("empty package dir no error", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/empty"
		prj := prjkit.New(t, modkit.Path(relPath))
		prj.Close()
		rng := parser.SetBuildTag(ring.New())

		// --- When ---
		fls, err := gmFiles(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.NotNil(t, fls)
		assert.Empty(t, fls)
	})

	t.Run("buildTag must not be empty", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os_build_tag/project"
		prj := prjkit.New(t, modkit.Path(relPath))
		prj.Close()
		rng := ring.New()

		// --- When ---
		fls, err := gmFiles(rng, prj.Root())

		// --- Then ---
		assert.ErrorEqual(t, "build tag must be provided", err)
		assert.Nil(t, fls)
	})

	t.Run("only tagged files", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/simple_tagged/project"
		prj := prjkit.New(t, modkit.Path(relPath))
		prj.Close()
		rng := parser.SetBuildTag(ring.New())

		// --- When ---
		have, err := gmFiles(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			prj.Path(mkf.MakefileMain),
		}
		assert.Equal(t, want, have)
	})

	t.Run("only not tagged files", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/simple_untagged/project"
		prj := prjkit.New(t, modkit.Path(relPath))
		prj.Close()
		rng := parser.SetBuildTag(ring.New())

		// --- When ---
		have, err := gmFiles(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, have)
	})
}

func Test_goFiles(t *testing.T) {
	t.Run("list files without build tag", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os_build_tag/project"
		prj := prjkit.New(t, modkit.Path(relPath))
		prj.Close()
		rng := ring.New()
		rng.EnvSet("GOOS", "darwin")
		rng.EnvSet("GOARCH", "amd64")

		// --- When ---
		fls, err := goFiles(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		exp := []string{
			prj.Path("main.go"),
			prj.Path("main_helpers.go"),
		}
		assert.Equal(t, exp, fls)
	})

	t.Run("list files with and without build tag", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os_build_tag/project"
		prj := prjkit.New(t, modkit.Path(relPath))
		prj.Close()
		rng := ring.New()
		rng.EnvSet("GOOS", "darwin")
		rng.EnvSet("GOARCH", "amd64")
		rng = parser.SetBuildTag(rng)

		// --- When ---
		fls, err := goFiles(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)

		exp := []string{
			prj.Path("main.go"),
			prj.Path("main_helpers.go"),
			prj.Path(mkf.MakefileMain),
			prj.Path("makefile_amd64.go"),
			prj.Path("makefile_darwin.go"),
			prj.Path("makefile_darwin_amd64.go"),
		}
		assert.Equal(t, exp, fls)
	})

	t.Run("unknown directory error", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/not/existing"
		prj := prjkit.New(t, modkit.Path(relPath))
		prj.Close()
		rng := ring.New()

		// --- When ---
		fls, err := goFiles(rng, prj.Root())

		// --- Then ---
		assert.ErrorContain(t, "listing Go source files", err)
		assert.Nil(t, fls)
	})

	t.Run("multi package error", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/packages/multi"
		prj := prjkit.New(t, modkit.Path(relPath))
		prj.Close()
		rng := ring.New()

		// --- When ---
		fls, err := goFiles(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)

		exp := []string{
			filepath.Join(prj.Root(), "multi_file.go"),
		}
		assert.Equal(t, exp, fls)
	})

	t.Run("multi package error with build tag", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/packages/multi"
		prj := prjkit.New(t, modkit.Path(relPath))
		prj.Close()
		rng := parser.SetBuildTag(ring.New())

		// --- When ---
		fls, err := goFiles(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)

		exp := []string{
			prj.Path("main_file.go"),
			prj.Path("multi_file.go"),
		}
		assert.Equal(t, exp, fls)
	})

	t.Run("empty package dir no error", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/empty"
		prj := prjkit.New(t, modkit.Path(relPath))
		prj.Close()
		rng := ring.New()

		// --- When ---
		fls, err := goFiles(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.NotNil(t, fls)
		assert.Empty(t, fls)
	})

	t.Run("current os and arch used", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os/project"
		prj := prjkit.New(t, modkit.Path(relPath))
		prj.Close()
		rng := ring.New()

		// --- When ---
		fls, err := goFiles(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Has(t, prj.Path("main.go"), fls)
		assert.Has(t, prj.Path("main_helpers.go"), fls)
		assert.Has(t, prj.Path(mkf.MakefileMain), fls)
		assert.True(t, len(fls) > 2)
	})
}

func Test_allTargets(t *testing.T) {
	t.Run("bin only", func(t *testing.T) {
		// --- Given ---
		cfg := &config{bin: "/tmp/out"}
		rng := ring.New()

		// --- When ---
		have, err := allTargets(rng, cfg, builtin.Empty().Targets())

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
		all, err := allTargets(tst.Ring(), cfg, gen)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, len(all) > len(gen))
	})
}
