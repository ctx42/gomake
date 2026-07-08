// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/goldy"
	"github.com/ctx42/testkit/pkg/exekit"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"

	"github.com/ctx42/gomake/internal/builtin"
	"github.com/ctx42/gomake/internal/builtin/builtintest"
	gmt "github.com/ctx42/gomake/internal/cli/clitest"
	"github.com/ctx42/gomake/internal/mkf"
)

func mainNoMakefileInWD(t *testing.T) {
	t.Run("no target name provided", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/no_makefiles/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--tmp", prj.TempDir(),
			"--src", prj.Root(),
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodePickTarget, code)
		assert.Equal(t, "gomake: "+mkf.ErrPickTarget.Error()+"\n", tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("no go module", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		prj := gmt.NewProject(t)
		prj.Close()

		rng := tst.Ring(
			"--tmp", prj.TempDir(),
			"--src", prj.Root(),
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodePickTarget, code)
		assert.Equal(t, "gomake: "+mkf.ErrPickTarget.Error()+"\n", tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("target name provided", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/no_makefiles/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"target",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeUnkTarget, code)
		assert.Equal(t, "gomake: "+mkf.ErrUnkTarget.Error()+"\n", tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("print version", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/no_makefiles/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--version",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeOK, code)
		assert.Equal(t, "1.2.3\n", tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("built-in target name provided", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		relPath := "testdata/projects/no_makefiles/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			":print",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "message", tst.Stdout())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("not existing built-in target name provided", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/no_makefiles/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			":tgt-x",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeUnkTarget, code)
		assert.Equal(t, "gomake: unknown target\n", tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("custom bin path", func(t *testing.T) {
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/no_makefiles/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		bin := filepath.Join(t.TempDir(), "my-bin")
		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--bin", bin,
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 1, code)
		assert.Contain(t, "no makefile found in", tst.Stderr())
		assert.Contain(t, prj.Root(), tst.Stderr())
		assert.NoFileExist(t, bin)
	})

	t.Run("show help", func(t *testing.T) {
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/no_makefiles/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--help",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		gfp := filepath.Join(absPath, "help_with_builtin.no_trim.gld")
		gld := goldy.Open(t, gfp)
		assert.Equal(t, gld.String(), tst.Stderr())
	})
}

func mainMakefileWithNoTargets(t *testing.T) {
	t.Run("no target name provided", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/no_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodePickTarget, code)
		assert.Equal(t, mkf.ErrPickTarget.Error()+"\n", tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("target name provided", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/no_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"target",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeUnkTarget, code)
		assert.Equal(t, mkf.ErrUnkTarget.Error()+"\n", tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("built-in target name provided", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		relPath := "testdata/projects/no_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			":print",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "message", tst.Stdout())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("not existing built-in target name provided", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/no_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			":tgt-x",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeUnkTarget, code)
		assert.Equal(t, "unknown target\n", tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("custom bin path", func(t *testing.T) {
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/no_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		bin := filepath.Join(t.TempDir(), "my-bin")
		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--bin", bin,
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeErr, code)
		assert.Equal(t, "gomake: no makefile found\n", tst.Stderr())
		assert.NoFileExist(t, bin)
	})

	t.Run("show help", func(t *testing.T) {
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/no_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--help",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		gfp := filepath.Join(absPath, "help_with_builtin.no_trim.gld")
		gld := goldy.Open(t, gfp)
		assert.Equal(t, gld.String(), tst.Stderr())
	})
}

func mainMakefileNoDefaultTarget(t *testing.T) {
	t.Run("no target name provided", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/showcase_imports/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 126, code)
		assert.Equal(t, "pick a target to execute\n", tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("built-in target name provided", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		relPath := "testdata/projects/showcase_imports/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			":print",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "message", tst.Stdout())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("not existing built-in target name provided", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/showcase_imports/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			":tgt-x",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeUnkTarget, code)
		assert.Equal(t, "unknown target\n", tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("custom bin path - call built-n", func(t *testing.T) {
		ctx := t.Context()
		tst := ringtest.New(t)

		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		bin := filepath.Join(t.TempDir(), "my-bin")
		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--bin", bin,
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "Basic", exekit.New(t).ExeStdout(bin, "basic"))
	})

}

func Test_main_scenarios(t *testing.T) {
	t.Run("no makefile in wd", mainNoMakefileInWD)
	t.Run("makefile with no targets", mainMakefileWithNoTargets)
	t.Run("makefile no default target", mainMakefileNoDefaultTarget)
	t.Run("default targets", mainDefaultTargets)
	t.Run("target timeouts", mainTargetTimeouts)
	t.Run("pre run", mainPreRun)
}

func Test_main(t *testing.T) {
	t.Run("unknown target name provided", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"unknown",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 127, code)
		assert.Equal(t, "unknown target\n", tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("exec local target", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		relPath := "testdata/projects/showcase_imports/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"local",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "local target says hello", tst.Stdout())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("exec local target using imported function", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		relPath := "testdata/projects/showcase_imports/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"imported",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "pkg8.Pkg8F1", tst.Stdout())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("exec imported target", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		relPath := "testdata/projects/showcase_imports/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"pkg0",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "pkg0.Pkg0", tst.Stdout())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("exec target from namespaced import", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		relPath := "testdata/projects/showcase_imports/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"ns:pkg1",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "pkg1.Pkg1", tst.Stdout())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("arguments passed to target", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"print-args", "--arg0", "arg1",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "[--arg0 arg1]", tst.Stdout())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("panicking target", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"panic-string",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 1, code)
		assert.Equal(t, "target panicked with: panic string\n", tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("panicking built-in target", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/showcase_imports/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			":panic-string",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 1, code)
		want := "gomake: target panicked with: panic string\n"
		assert.Equal(t, want, tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("duplicated targets", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/dup_imported/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 1, code)
		want := "gomake: duplicated target: PKG1, pkg1.Pkg1\n"
		assert.Equal(t, want, tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("target shows its arg help", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"say-hello", "--unknown",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 1, code)
		exp := "" +
			"flag provided but not defined: -unknown\n" +
			"Usage of say-hello:\n" +
			"  -to string\n" +
			"    \ttarget argument (default \"the World\")\n" +
			"flag provided but not defined: -unknown\n"
		assert.Equal(t, exp, tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("built-in target shows its help", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"-h",
			":print",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		exp := ":print\tis a test target with help message.\n"
		assert.Equal(t, exp, tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("custom bin path", func(t *testing.T) {
		ctx := t.Context()
		tst := ringtest.New(t)

		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		bin := filepath.Join(t.TempDir(), "my-bin")
		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--bin", bin,
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "Basic", exekit.New(t).ExeStdout(bin, "basic"))
	})

	t.Run("show version", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/showcase_imports/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--version",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "1.2.3\n", tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("list targets", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--list",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		gfp := filepath.Join(absPath, "list_with_builtin.no_trim.gld")
		gld := goldy.Open(t, gfp)
		assert.Equal(t, gld.String(), tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("show help", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--help",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		gfp := filepath.Join(absPath, "help_with_builtin.no_trim.gld")
		gld := goldy.Open(t, gfp)
		assert.Equal(t, gld.String(), tst.Stderr())
	})
}

func mainDefaultTargets(t *testing.T) {
	t.Run("namespaced", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		relPath := "testdata/projects/default_from_ns/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "NS says hello", tst.Stdout())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("custom bin path", func(t *testing.T) {
		ctx := t.Context()
		tst := ringtest.New(t)

		relPath := "testdata/projects/default_from_ns/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		bin := filepath.Join(t.TempDir(), "my-bin")
		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--bin", bin,
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "NS says hello", exekit.New(t).ExeStdout(bin))
	})
}

func mainTargetTimeouts(t *testing.T) {
	t.Run("long-running target finished", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"long",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "done after 100ms", tst.Stdout())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("long-running target finished because timeout is longer",
		func(t *testing.T) {
			// --- Given ---
			ctx := t.Context()
			tst := ringtest.New(t).WetStdout()

			relPath := "testdata/projects/showcase_targets/project"
			absPath := modkit.Path(relPath)
			prj := gmt.NewProject(t)
			prj.GoModInit()
			prj.MakefilesFrom(absPath)
			prj.UseGomakeSrc(modkit.Root())
			prj.GoModTidy()
			prj.Close()

			rng := tst.Ring(
				"--src", prj.Root(),
				"--tmp", prj.TempDir(),
				"--timeout", "150ms",
				"long",
			)
			ver := "1.2.3"
			bip := builtintest.NewTstProvider()

			// --- When ---
			code := Main(ctx, rng, ver, bip)

			// --- Then ---
			assert.Equal(t, 0, code)
			assert.Equal(t, "done after 100ms", tst.Stdout())
			assert.Len(t, 0, oskit.List(t, prj.TempDir()))
		})

	t.Run("long-running target canceled because timeout shorter",
		func(t *testing.T) {
			// --- Given ---
			ctx := t.Context()
			tst := ringtest.New(t).WetStderr()

			relPath := "testdata/projects/showcase_targets/project"
			absPath := modkit.Path(relPath)
			prj := gmt.NewProject(t)
			prj.GoModInit()
			prj.MakefilesFrom(absPath)
			prj.UseGomakeSrc(modkit.Root())
			prj.GoModTidy()
			prj.Close()

			rng := tst.Ring(
				"--src", prj.Root(),
				"--tmp", prj.TempDir(),
				"--timeout", "50ms",
				"long",
			)
			ver := "1.2.3"
			bip := builtintest.NewTstProvider()

			// --- When ---
			code := Main(ctx, rng, ver, bip)

			// --- Then ---
			assert.Equal(t, 125, code)
			assert.Equal(t, "context deadline exceeded\n", tst.Stderr())
			assert.Len(t, 0, oskit.List(t, prj.TempDir()))
		})
}

func Test_RunWithoutCompile(t *testing.T) {
	t.Run("call target with arguments", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring(":print", "abc")

		ver := "1.2.3"
		tgs := builtintest.NewTstProvider().Targets()

		// --- When ---
		rc := runWithoutCompile(ctx, rng, ver, tgs)

		// --- Then ---
		assert.Equal(t, 0, rc)
		assert.Equal(t, "[abc]", tst.Stdout())
	})

	t.Run("makefile error", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--unknown", ":tgt-a")

		ver := "1.2.3"
		tgs := builtintest.NewTstProvider().Targets()

		// --- When ---
		rc := runWithoutCompile(ctx, rng, ver, tgs)

		// --- Then ---
		assert.Equal(t, 1, rc)
		want := "" +
			"gomake: parsing flags: flag provided but " +
			"not defined: -unknown\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("execute error", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring(":panic-string")

		ver := "1.2.3"
		bip := builtintest.NewTstProvider().Targets()

		// --- When ---
		rc := runWithoutCompile(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 1, rc)
		want := "gomake: target panicked with: panic string\n"
		assert.Equal(t, want, tst.Stderr())
	})
}

func Test_main_completion(t *testing.T) {
	t.Run("gomake :<TAB>", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"gomake", ":", "gomake",
		)
		rng.EnvSet("COMP_CWORD", "1")
		rng.EnvSet("COMP_LINE", "gomake :")
		rng.EnvSet("COMP_POINT", "8")
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		want := ":panic-string :print"
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("gomake :pa<TAB>", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"gomake", ":pa", "gomake",
		)
		rng.EnvSet("COMP_CWORD", "1")
		rng.EnvSet("COMP_LINE", "gomake :")
		rng.EnvSet("COMP_POINT", "9")
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, ":panic-string", tst.Stdout())
	})

	t.Run("gomake :print <TAB>", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t)

		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"gomake", "", ":print",
		)
		rng.EnvSet("COMP_CWORD", "2")
		rng.EnvSet("COMP_LINE", "gomake :print")
		rng.EnvSet("COMP_POINT", "15")
		ver := "1.2.3"
		bip := builtintest.NewTstProvider()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
	})

	t.Run("not enough arguments", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t)

		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"gomake", ":",
		)
		rng.EnvSet("COMP_CWORD", "1")
		rng.EnvSet("COMP_LINE", "gomake :")
		rng.EnvSet("COMP_POINT", "9")
		ver := "1.2.3"
		bip := builtin.Empty()

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
	})
}

func mainPreRun(t *testing.T) {
	t.Run("run pre runs", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		relPath := "testdata/projects/pre_run/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"env-key",
			"GM_TEST",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider(preAddEnv, preAddEnv)

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "GM_TEST (true): `aa`", tst.Stdout())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("pre run error does not run target", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/pre_run/project"
		absPath := modkit.Path(relPath)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPath)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"env-key",
			"GM_TEST",
		)
		ver := "1.2.3"
		bip := builtintest.NewTstProvider(preAddEnv, preErr, preAddEnv)

		// --- When ---
		code := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 1, code)
		assert.Equal(t, "gomake: test error: a\n", tst.Stderr())
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})
}

func Test_complete_tabular(t *testing.T) {
	tt := []struct {
		testN string

		args []string
		tgs  []*mkf.Target
		want string
	}{
		{
			"wrong arg count",
			[]string{"gomake"},
			nil,
			"",
		},
		{
			"prefix match",
			[]string{"gomake", ":p", "gomake"},
			[]*mkf.Target{
				{Name: ":print"},
			},
			":print",
		},
		{
			"already completed",
			[]string{"gomake", "", ":print"},
			[]*mkf.Target{{Name: ":print"}},
			"",
		},
		{
			"skip hidden",
			[]string{"gomake", "", "gomake"},
			[]*mkf.Target{
				{Name: ":visible"},
				{Name: ":hidden", Hidden: true},
			},
			":visible",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := complete(tc.args, tc.tgs)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_main_error(t *testing.T) {
	t.Run("NewConfig error", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--bin", "/tmp/out", "--help")

		// --- When ---
		code := Main(ctx, rng, "1.0", builtin.Empty())

		// --- Then ---
		assert.Equal(t, 1, code)
		want := "gomake: -h, --help cannot be used with --bin\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("tmp path stat fails", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		dir := t.TempDir()
		assert.NoError(t, os.Chmod(dir, 0))
		t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
		tmp := filepath.Join(dir, "sub")
		relPath := "testdata/projects/no_makefiles/project"
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(modkit.Path(relPath))
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		rng := tst.Ring("--src", prj.Root(), "--tmp", tmp)

		// --- When ---
		code := Main(context.Background(), rng, "1.0", builtin.Empty())

		// --- Then ---
		assert.Equal(t, 1, code)
		have := tst.Stderr()
		assert.Contain(t, "gomake: ", have)
		assert.Contain(t, tmp, have)
	})

	t.Run("tmp mkdir fails", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		base := t.TempDir()
		parent := filepath.Join(base, "file")
		oskit.Write(t, "x", parent)
		tmpPath := filepath.Join(parent, "child", "tmp")
		relPath := "testdata/projects/no_makefiles/project"
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(modkit.Path(relPath))
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		rng := tst.Ring("--src", prj.Root(), "--tmp", tmpPath)

		// --- When ---
		code := Main(context.Background(), rng, "1.0", builtin.Empty())

		// --- Then ---
		assert.Equal(t, 1, code)
		have := tst.Stderr()
		assert.Contain(t, "gomake: ", have)
		assert.Contain(t, tmpPath, have)
	})

	t.Run("tmp path is a file", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		tmpFile := filepath.Join(t.TempDir(), "not-a-dir")
		oskit.Write(t, "x", tmpFile)
		relPath := "testdata/projects/no_makefiles/project"
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(modkit.Path(relPath))
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		rng := tst.Ring("--src", prj.Root(), "--tmp", tmpFile)

		// --- When ---
		code := Main(context.Background(), rng, "1.0", builtin.Empty())

		// --- Then ---
		assert.Equal(t, 1, code)
		assert.Contain(t, "must be a directory", tst.Stderr())
	})

	t.Run("tmp path created when missing", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		relPath := "testdata/projects/no_makefiles/project"
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(modkit.Path(relPath))
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		tmp := filepath.Join(t.TempDir(), "gomake-tmp")
		rng := tst.Ring("--src", prj.Root(), "--tmp", tmp)

		// --- When ---
		code := Main(context.Background(), rng, "1.0", builtin.Empty())

		// --- Then ---
		assert.Equal(t, mkf.ExitCodePickTarget, code)
		assert.Equal(t, "gomake: "+mkf.ErrPickTarget.Error()+"\n", tst.Stderr())
	})

	t.Run("--help AllTargets error", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		relPath := "testdata/projects/dup_imported/project"
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(modkit.Path(relPath))
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--help",
		)

		// --- When ---
		code := Main(context.Background(), rng, "1.0", builtin.Empty())

		// --- Then ---
		assert.Equal(t, 1, code)
		assert.Contain(t, "duplicated target", tst.Stderr())
	})

	t.Run("--list AllTargets error", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		relPath := "testdata/projects/dup_imported/project"
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(modkit.Path(relPath))
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--list",
		)

		// --- When ---
		code := Main(context.Background(), rng, "1.0", builtin.Empty())

		// --- Then ---
		assert.Equal(t, 1, code)
		assert.Contain(t, "duplicated target", tst.Stderr())
	})

	t.Run("--help unknown target", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		relPath := "testdata/projects/showcase_targets/project"
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(modkit.Path(relPath))
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--help", ":no-such-target",
		)

		// --- When ---
		code := Main(context.Background(), rng, "1.0", builtin.Empty())

		// --- Then ---
		assert.Equal(t, 1, code)
		assert.Contain(t, mkf.ErrUnkTarget.Error(), tst.Stderr())
	})

	t.Run("execute compile error", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		relPath := "testdata/projects/simple_untagged/project"
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(modkit.Path(relPath))
		prj.UseGomakeSrc(modkit.Root())
		oskit.Write(t, `
func BrokenCompile(ctx context.Context, rng *ring.Ring) error {
	_ = undefinedSymbol
	return nil
}
`, filepath.Join(prj.Root(), "makefile.go"))
		prj.GoModTidy()
		prj.Close()
		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"broken-compile",
		)

		// --- When ---
		code := Main(context.Background(), rng, "1.0", builtin.Empty())

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeCompile, code)
		assert.Contain(t, "undefinedSymbol", tst.Stderr())
	})

	t.Run("--bin compile error", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		relPath := "testdata/projects/simple_untagged/project"
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(modkit.Path(relPath))
		prj.UseGomakeSrc(modkit.Root())
		oskit.Write(t, `
func BrokenCompile(ctx context.Context, rng *ring.Ring) error {
	_ = undefinedSymbol
	return nil
}
`, filepath.Join(prj.Root(), "makefile.go"))
		prj.GoModTidy()
		prj.Close()
		bin := filepath.Join(t.TempDir(), "out")
		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"--bin", bin,
		)

		// --- When ---
		code := Main(context.Background(), rng, "1.0", builtin.Empty())

		// --- Then ---
		assert.Equal(t, 1, code)
		assert.Contain(t, "undefinedSymbol", tst.Stderr())
	})

	t.Run("pre-run panic recovered", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		relPath := "testdata/projects/simple_untagged/project"
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(modkit.Path(relPath))
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			"hello",
		)

		// --- When ---
		code := Main(
			context.Background(),
			rng,
			"1.0",
			builtintest.NewTstProvider(prePanic),
		)

		// --- Then ---
		assert.Equal(t, 1, code)
		assert.Contain(t, "panicked with:", tst.Stderr())
	})
}

// Test_Main_targetConfig verifies that a target's gomake.yaml configuration
// block is delivered to the running target through the compiled-makefile
// subprocess bridge.
func Test_Main_targetConfig(t *testing.T) {
	// --- Given ---
	ctx := t.Context()
	tst := ringtest.New(t).WetStdout()

	absPath := modkit.Path("testdata/projects/config_target/project")
	prj := gmt.NewProject(t)
	prj.GoModInit()
	prj.MakefilesFrom(absPath)
	prj.UseGomakeSrc(modkit.Root())
	prj.GoModTidy()
	prj.Close()

	yaml := "version: 1\n" +
		"targets:\n" +
		"  " + prjkit.GoModName + "#Show:\n" +
		"    message: hello-from-config\n"
	oskit.Write(t, yaml, prj.Root(), configFileName)

	rng := tst.Ring(
		"--src", prj.Root(),
		"--tmp", prj.TempDir(),
		"show",
	)
	bip := builtintest.NewTstProvider()

	// --- When ---
	code := Main(ctx, rng, "1.2.3", bip)

	// --- Then ---
	assert.Equal(t, 0, code)
	assert.Equal(t, "message=hello-from-config", tst.Stdout())
}

func Test_Main(t *testing.T) {
	// --- Given ---
	old := append([]string(nil), os.Args...)
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{"gomake", "--version"}
	rng := ring.New()

	// --- When ---
	code := Main(context.Background(), rng, "9.9.9-test", builtin.Empty())

	// --- Then ---
	assert.Equal(t, 0, code)
}

// prePanic is a pre-run hook that panics (exercises main's recover path).
func prePanic(
	ctx context.Context,
	rng *ring.Ring,
) (context.Context, *ring.Ring, error) {

	panic("pre-run panic")
}

// preAddEnv is pre-run function matching [mkf.PreRunFn] signature which
// appends to `GM_TEST` environment variable letter `a`.
func preAddEnv(
	ctx context.Context,
	rng *ring.Ring,
) (context.Context, *ring.Ring, error) {

	rng.EnvSet("GM_TEST", rng.EnvGet("GM_TEST")+"a")
	return ctx, rng, nil
}

// preErr is pre-run function matching [mkf.PreRunFn] signature which always
// returns error.
func preErr(
	ctx context.Context,
	rng *ring.Ring,
) (context.Context, *ring.Ring, error) {

	return ctx, rng, fmt.Errorf("test error: %v", rng.EnvGet("GM_TEST"))
}
