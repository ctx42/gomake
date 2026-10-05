// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package builtin

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/goldy"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"

	"github.com/ctx42/gomake/internal/builtin/builtintest"
	"github.com/ctx42/gomake/internal/cli/clitest"
	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/internal/parser"
)

func Test_Empty(t *testing.T) {
	// --- When ---
	have := Empty()

	// --- Then ---
	assert.Nil(t, have.Targets())
	assert.Equal(t, tgsMainEmptySrc, have.Source())
}

func Test_Generated(t *testing.T) {
	// --- When ---
	have := Generated()

	// --- Then ---
	assert.Equal(t, targetsBuiltIn(), have.Targets())
}

func Test_newTargets(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		want := builtintest.TestTargets()
		src := builtintest.TestTargetsSrc()
		pre := []mkf.PreRunFn{builtintest.TestPreRun}

		// --- When ---
		have := newTargets(want, src, pre)

		// --- Then ---
		assert.Equal(t, want, have.Targets())
		assert.NotSame(t, want, have.Targets())
		assert.Equal(t, string(src), string(have.Source()))
		assert.Len(t, 1, have.PreRuns())
		assert.Same(t, pre[0], have.PreRuns()[0])
		assert.NotSame(t, pre, have.pre)
	})

	t.Run("no targets", func(t *testing.T) {
		// --- Given ---
		src := builtintest.TestTargetsSrc()

		// --- When ---
		have := newTargets(make([]*mkf.Target, 0), src, nil)

		// --- Then ---
		assert.Len(t, 0, have.Targets())
		assert.Equal(t, string(tgsMainEmptySrc), string(have.Source()))
	})

	t.Run("all arguments nil", func(t *testing.T) {
		// --- When ---
		have := newTargets(nil, nil, nil)

		// --- Then ---
		assert.Len(t, 0, have.Targets())
		assert.Equal(t, string(tgsMainEmptySrc), string(have.Source()))
	})
}

func Test_targets_Targets(t *testing.T) {
	// --- Given ---
	want := builtintest.TestTargets()
	src := builtintest.TestTargetsSrc()
	tgs := newTargets(want, src, nil)

	// --- When ---
	have := tgs.Targets()

	// --- Then ---
	assert.Equal(t, want, have)
	assert.NotSame(t, want, have)
}

func Test_targets_PreRuns(t *testing.T) {
	// --- Given ---
	pre := []mkf.PreRunFn{builtintest.TestPreRun}
	tgs := newTargets(nil, nil, pre)

	// --- When ---
	have := tgs.PreRuns()

	// --- Then ---
	assert.Equal(t, pre, have)
	assert.NotSame(t, tgs.pre, have)
}

func Test_targets_Source(t *testing.T) {
	t.Run("returns the source", func(t *testing.T) {
		// --- Given ---
		want := builtintest.TestTargets()
		src := builtintest.TestTargetsSrc()
		tgs := newTargets(want, src, nil)

		// --- When ---
		have := tgs.Source()

		// --- Then ---
		assert.Equal(t, string(src), string(have))
	})

	t.Run("clone", func(t *testing.T) {
		// --- Given ---
		want := builtintest.TestTargets()
		src := builtintest.TestTargetsSrc()
		tgs := newTargets(want, src, nil)

		// --- When ---
		have := tgs.Source()

		// --- Then ---
		assert.NotSame(t, tgs.src, have)
	})
}

func Test_WithGenName(t *testing.T) {
	// --- Given ---
	opts := &genOpts{}

	// --- When ---
	WithGenName("name")(opts)

	// --- Then ---
	assert.Equal(t, "name", opts.name)
}

func Test_WithGenEnv(t *testing.T) {
	// --- Given ---
	rng := ring.New()
	opts := &genOpts{}

	// --- When ---
	WithGenEnv(rng)(opts)

	// --- Then ---
	assert.Same(t, rng, opts.rng)
}

func Test_WithGenDst(t *testing.T) {
	// --- Given ---
	opts := &genOpts{}

	// --- When ---
	WithGenDst("/path/to")(opts)

	// --- Then ---
	assert.Equal(t, "/path/to", opts.dst)
}

func Test_WithGenWorkDir(t *testing.T) {
	// --- Given ---
	opts := &genOpts{}

	// --- When ---
	WithGenWorkDir("/path/to")(opts)

	// --- Then ---
	assert.Equal(t, "/path/to", opts.dir)
}

func Test_WithGenImpPathRoot(t *testing.T) {
	// --- Given ---
	opts := &genOpts{}

	// --- When ---
	WithGenImpPathRoot("/path/to")(opts)

	// --- Then ---
	assert.Equal(t, "/path/to", opts.root)
}

func Test_WithoutGenEmptySrc(t *testing.T) {
	// --- Given ---
	opts := &genOpts{
		empty: true,
	}

	// --- When ---
	WithoutGenEmptySrc(opts)

	// --- Then ---
	assert.False(t, opts.empty)
}

func Test_GenMain(t *testing.T) {
	t.Run("generate all", func(t *testing.T) {
		// --- Given ---
		prj := clitest.NewProject(t)
		prj.Close()

		impSpecs := []string{
			"github.com/ctx42/gomake/testdata/imports/pkg0",
		}

		data := goldy.WithData(map[string]any{"prj_root": modkit.Root()})
		gldB := goldy.Open(t, "testdata/pkg0_builtin_targets.gld", data)
		gldM := goldy.Open(t, "testdata/pkg0_main_targets.gld", data)
		gldE := goldy.Open(t, "testdata/pkg0_main_targets_empty.gld", data)

		opts := []GenOption{
			WithGenDst(prj.Root()),
		}

		// --- When ---
		err := GenMain(impSpecs, opts...)

		// --- Then ---
		assert.NoError(t, err)
		text := oskit.ReadFileStr(t, prj.Path(targetsFN))
		assert.Equal(t, gldB.String(), text)

		text = oskit.ReadFileStr(t, prj.Path("data", mainFN))
		assert.Equal(t, gldM.String(), text)

		text = oskit.ReadFileStr(t, prj.Path("data", mainEmptyFN))
		assert.Equal(t, gldE.String(), text)
	})

	t.Run("generate without mainEmptyFN", func(t *testing.T) {
		// --- Given ---
		prj := clitest.NewProject(t)
		prj.Close()

		impSpecs := []string{
			"github.com/ctx42/gomake/testdata/imports/pkg0",
		}
		opts := []GenOption{
			WithoutGenEmptySrc,
			WithGenDst(prj.Root()),
		}

		// --- When ---
		err := GenMain(impSpecs, opts...)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, prj.Path(targetsFN))
		assert.FileExist(t, prj.Path("data", mainFN))
		assert.NoFileExist(t, prj.Path("data", mainEmptyFN))
	})

	t.Run("generate with given package name", func(t *testing.T) {
		// --- Given ---
		prj := clitest.NewProject(t)
		prj.Close()

		impSpecs := []string{
			"github.com/ctx42/gomake/testdata/imports/pkg0",
		}
		opts := []GenOption{
			WithGenDst(prj.Root()),
			WithGenName("abc"),
		}

		// --- When ---
		err := GenMain(impSpecs, opts...)

		// --- Then ---
		assert.NoError(t, err)
		text := oskit.ReadFileStr(t, prj.Path(targetsFN))
		assert.Contain(t, "\npackage abc\n", text)

		populated := oskit.ReadFileStr(t, prj.Path("data", mainFN))
		assert.Contain(t, "pkg0.Pkg0", populated)

		empty := oskit.ReadFileStr(t, prj.Path("data", mainEmptyFN))
		assert.Contain(t, "make([]*Target, 0)\n", empty)
		assert.NotContain(t, "pkg0.Pkg0", empty)
	})

	t.Run("error - invalid import", func(t *testing.T) {
		// --- Given ---
		prj := clitest.NewProject(t)
		prj.Close()

		impSpecs := []string{
			"github.com/ctx42/gomake/testdata/projects/empty",
		}
		opts := []GenOption{
			WithGenDst(prj.Root()),
		}

		// --- When ---
		err := GenMain(impSpecs, opts...)

		// --- Then ---
		assert.ErrorIs(t, parser.ErrGoList, err)
		assert.NoFileExist(t, prj.Path(targetsFN))
		assert.NoFileExist(t, prj.Path("data", mainFN))
		assert.NoFileExist(t, prj.Path("data", mainEmptyFN))
	})
}

func Test_GenImports(t *testing.T) {
	t.Run("namespace", func(t *testing.T) {
		// --- Given ---
		prj := clitest.NewProject(t)
		prj.Close()

		imports := []parser.Import{
			{
				Path:      "github.com/ctx42/gomake/testdata/imports/pkg0",
				Namespace: "myns",
			},
		}
		opts := []GenOption{
			WithoutGenEmptySrc,
			WithGenDst(prj.Root()),
		}

		// --- When ---
		err := GenImports(imports, opts...)

		// --- Then ---
		assert.NoError(t, err)
		text := oskit.ReadFileStr(t, prj.Path(targetsFN))
		assert.Contain(t, `PkgNS:       "myns",`, text)
		assert.Contain(t, `Name:        ":myns:pkg0",`, text)
	})

	t.Run("ImpPath relative to the root", func(t *testing.T) {
		// --- Given ---
		prj := clitest.NewProject(t)
		prj.Close()

		imports := []parser.Import{
			{Path: "github.com/ctx42/gomake/testdata/imports/pkg0"},
		}
		opts := []GenOption{
			WithoutGenEmptySrc,
			WithGenDst(prj.Root()),
			WithGenImpPathRoot(modkit.Root()),
		}

		// --- When ---
		err := GenImports(imports, opts...)

		// --- Then ---
		assert.NoError(t, err)
		text := oskit.ReadFileStr(t, prj.Path(targetsFN))
		assert.Contain(t, `ImpPath:     "testdata/imports/pkg0",`, text)
	})

	t.Run("ImpPath outside root", func(t *testing.T) {
		// --- Given ---
		prj := clitest.NewProject(t)
		prj.Close()

		imports := []parser.Import{
			{Path: "github.com/ctx42/gomake/testdata/imports/pkg0"},
		}
		opts := []GenOption{
			WithoutGenEmptySrc,
			WithGenDst(prj.Root()),
			WithGenImpPathRoot(t.TempDir()),
		}

		// --- When ---
		err := GenImports(imports, opts...)

		// --- Then ---
		assert.NoError(t, err)
		text := oskit.ReadFileStr(t, prj.Path(targetsFN))
		want := `ImpPath:     "` + modkit.Path("testdata/imports/pkg0") + `",`
		assert.Contain(t, want, text)
	})

	t.Run("error - data dir not writable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root ignores file permissions")
		}

		// --- Given ---
		prj := clitest.NewProject(t)
		prj.Close()

		oskit.Write(t, "old", prj.Root(), targetsFN)
		data := oskit.MkdirAll(t, prj.Root(), "data")
		must.Nil(os.Chmod(data, 0o555))
		t.Cleanup(func() { _ = os.Chmod(data, 0o755) })

		// --- When ---
		err := GenImports(nil, WithGenDst(prj.Root()))

		// --- Then ---
		assert.ErrorIs(t, fs.ErrPermission, err)
		assert.Equal(t, "old", oskit.ReadFileStr(t, prj.Path(targetsFN)))
	})

	t.Run("error - package name is not an identifier", func(t *testing.T) {
		// --- Given ---
		prj := clitest.NewProject(t)
		prj.Close()

		name := "not-a-name"

		// --- When ---
		err := GenImports(nil, WithGenName(name), WithGenDst(prj.Root()))

		// --- Then ---
		assert.ErrorContain(t, name, err)
		assert.NoFileExist(t, prj.Path(targetsFN))
	})
}

func Test_writeFiles(t *testing.T) {
	t.Run("writes every file", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		files := []genFile{
			{dst: filepath.Join(dir, "a.go"), code: []byte("a")},
			{dst: filepath.Join(dir, "b.go"), code: []byte("b")},
		}

		// --- When ---
		err := writeFiles(files)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "a", oskit.ReadFileStr(t, dir, "a.go"))
		assert.Equal(t, "b", oskit.ReadFileStr(t, dir, "b.go"))
		info := must.Value(os.Stat(filepath.Join(dir, "a.go")))
		assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
		assert.Len(t, 2, oskit.List(t, dir))
	})

	t.Run("error - missing directory", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		files := []genFile{
			{dst: filepath.Join(dir, "a.go"), code: []byte("a")},
			{dst: filepath.Join(dir, "none", "b.go"), code: []byte("b")},
		}

		// --- When ---
		err := writeFiles(files)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.Len(t, 0, oskit.List(t, dir))
	})
}

func Test_relImpPath(t *testing.T) {
	t.Run("dot dot named dir under root", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		tgt := &mkf.Target{ImpPath: filepath.Join(root, "..cache", "pkg")}

		// --- When ---
		relImpPath(root)(nil, tgt)

		// --- Then ---
		assert.Equal(t, "..cache/pkg", tgt.ImpPath)
	})

	t.Run("outside root", func(t *testing.T) {
		// --- Given ---
		root := filepath.Join(t.TempDir(), "root")
		pth := filepath.Join(filepath.Dir(root), "other", "pkg")
		tgt := &mkf.Target{ImpPath: pth}

		// --- When ---
		relImpPath(root)(nil, tgt)

		// --- Then ---
		assert.Equal(t, pth, tgt.ImpPath)
	})

	t.Run("relative root", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		t.Chdir(dir)
		tgt := &mkf.Target{ImpPath: filepath.Join(dir, "sub", "pkg")}

		// --- When ---
		relImpPath(".")(nil, tgt)

		// --- Then ---
		assert.Equal(t, "sub/pkg", tgt.ImpPath)
	})
}
