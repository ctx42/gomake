// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/check"
	"github.com/ctx42/testing/pkg/goldy"
	"github.com/ctx42/testkit/pkg/exekit"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"

	"github.com/ctx42/gomake/internal/builtin"
	"github.com/ctx42/gomake/internal/builtin/builtintest"
	gmt "github.com/ctx42/gomake/internal/cli/clitest"
	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/pkg/gomake"
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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodePickTarget, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodePickTarget, have)

		assert.Equal(t, "gomake: "+mkf.ErrPickTarget.Error()+"\n", tst.Stderr())

		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("no go module with target name", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()

		tst := ringtest.New(t).WetStderr()

		prj := gmt.NewProject(t)
		prj.Close()

		rng := tst.Ring(
			"--tmp", prj.TempDir(),
			"--src", prj.Root(),
			"hello",
		)

		ver := "1.2.3"

		bip := builtintest.NewTstProvider()

		// --- When ---
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeErr, have)

		assert.Contain(t, gomake.ErrNoGoMod.Error(), tst.Stderr())
		assert.NotContain(t, mkf.ErrUnkTarget.Error(), tst.Stderr())

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeUnkTarget, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeOK, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeUnkTarget, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 1, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodePickTarget, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeUnkTarget, have)

		assert.Equal(t, "unknown target: target\n", tst.Stderr())

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeUnkTarget, have)

		assert.Equal(t, "unknown target: :tgt-x\n", tst.Stderr())

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeErr, have)

		assert.Equal(t, "gomake: makefile has no targets\n", tst.Stderr())

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 126, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeUnkTarget, have)

		assert.Equal(t, "unknown target: :tgt-x\n", tst.Stderr())

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

		assert.Equal(t, "Basic", exekit.New(t).ExeStdout(bin, "basic"))
	})

}

func mainScenarios(t *testing.T) {
	t.Run("no makefile in wd", mainNoMakefileInWD)
	t.Run("makefile with no targets", mainMakefileWithNoTargets)
	t.Run("makefile no default target", mainMakefileNoDefaultTarget)
	t.Run("default targets", mainDefaultTargets)
	t.Run("target timeouts", mainTargetTimeouts)
	t.Run("pre run", mainPreRun)
}

func mainCases(t *testing.T) {
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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 127, have)

		assert.Equal(t, "unknown target: unknown\n", tst.Stderr())

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 1, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 1, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 1, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 1, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
			have := Main(ctx, rng, ver, bip)

			// --- Then ---
			assert.Equal(t, 0, have)

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
			have := Main(ctx, rng, ver, bip)

			// --- Then ---
			assert.Equal(t, 125, have)

			assert.Equal(t, "context deadline exceeded\n", tst.Stderr())

			assert.Len(t, 0, oskit.List(t, prj.TempDir()))
		})
}

func mainCompletion(t *testing.T) {
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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)
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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)
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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 0, have)

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
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 1, have)

		assert.Equal(t, "gomake: test error: a\n", tst.Stderr())

		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("pre runs fire for built-in target", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()

		tst := ringtest.New(t).WetStderr()

		relPath := "testdata/projects/no_makefiles/project"

		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(modkit.Path(relPath))
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--src", prj.Root(),
			"--tmp", prj.TempDir(),
			":print",
		)

		bip := builtintest.NewTstProvider(preErr)

		ver := "1.0"

		// --- When ---
		have := Main(ctx, rng, ver, bip)

		// --- Then ---
		assert.Equal(t, 1, have)

		assert.Contain(t, "test error", tst.Stderr())
	})
}

func mainErrors(t *testing.T) {
	t.Run("NewConfig error", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()

		tst := ringtest.New(t).WetStderr()

		rng := tst.Ring("--bin", "/tmp/out", "--help")

		ver := "1.0"

		empty := builtin.Empty()

		// --- When ---
		have := Main(ctx, rng, ver, empty)

		// --- Then ---
		assert.Equal(t, 1, have)

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

		ctx := t.Context()

		ver := "1.0"

		empty := builtin.Empty()

		// --- When ---
		hCode := Main(ctx, rng, ver, empty)

		// --- Then ---
		assert.Equal(t, 1, hCode)

		stderr := tst.Stderr()
		assert.Contain(t, "gomake: ", stderr)
		assert.Contain(t, tmp, stderr)
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

		ctx := t.Context()

		ver := "1.0"

		empty := builtin.Empty()

		// --- When ---
		hCode := Main(ctx, rng, ver, empty)

		// --- Then ---
		assert.Equal(t, 1, hCode)

		stderr := tst.Stderr()
		assert.Contain(t, "gomake: ", stderr)
		assert.Contain(t, tmpPath, stderr)
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

		ctx := t.Context()

		ver := "1.0"

		empty := builtin.Empty()

		// --- When ---
		have := Main(ctx, rng, ver, empty)

		// --- Then ---
		assert.Equal(t, 1, have)

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

		ctx := t.Context()

		ver := "1.0"

		empty := builtin.Empty()

		// --- When ---
		have := Main(ctx, rng, ver, empty)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodePickTarget, have)

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

		ctx := t.Context()

		ver := "1.0"

		empty := builtin.Empty()

		// --- When ---
		have := Main(ctx, rng, ver, empty)

		// --- Then ---
		assert.Equal(t, 1, have)

		want := "gomake: duplicated target: PKG1, pkg1.Pkg1\n"
		assert.Equal(t, want, tst.Stderr())
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

		ctx := t.Context()

		ver := "1.0"

		empty := builtin.Empty()

		// --- When ---
		have := Main(ctx, rng, ver, empty)

		// --- Then ---
		assert.Equal(t, 1, have)

		want := "gomake: duplicated target: PKG1, pkg1.Pkg1\n"
		assert.Equal(t, want, tst.Stderr())
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

		ctx := t.Context()

		ver := "1.0"

		empty := builtin.Empty()

		// --- When ---
		have := Main(ctx, rng, ver, empty)

		// --- Then ---
		assert.Equal(t, 1, have)

		assert.Contain(t, mkf.ErrUnkTarget.Error(), tst.Stderr())
	})

	t.Run("--help target with timeout", func(t *testing.T) {
		// --- Given ---
		// Non-default --timeout rebuilds cfg.args with makefile flags before
		// the target name; help must still resolve the target, not the duration.
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
			"--timeout", "1s",
			"--help", "say-hello",
		)

		ctx := t.Context()

		ver := "1.0"

		empty := builtin.Empty()

		// --- When ---
		have := Main(ctx, rng, ver, empty)

		// --- Then ---
		assert.Equal(t, 0, have)

		assert.Contain(t, "say-hello", tst.Stderr())
		assert.NotContain(t, mkf.ErrUnkTarget.Error(), tst.Stderr())
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

		ctx := t.Context()

		ver := "1.0"

		empty := builtin.Empty()

		// --- When ---
		have := Main(ctx, rng, ver, empty)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeCompile, have)

		assert.Contain(t, "undefined: undefinedSymbol", tst.Stderr())
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

		ctx := t.Context()

		ver := "1.0"

		empty := builtin.Empty()

		// --- When ---
		have := Main(ctx, rng, ver, empty)

		// --- Then ---
		assert.Equal(t, mkf.ExitCodeCompile, have)

		assert.Contain(t, "undefined: undefinedSymbol", tst.Stderr())

		assert.NoFileExist(t, bin)
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

		ctx := t.Context()

		ver := "1.0"

		newTstProvider := builtintest.NewTstProvider(prePanic)

		// --- When ---
		have := Main(
			ctx,
			rng,
			ver,
			newTstProvider,
		)

		// --- Then ---
		assert.Equal(t, 1, have)

		assert.Contain(t, "panicked with:", tst.Stderr())
	})
}

// Test_Main_targetConfig verifies that a target's gomake.yaml configuration
// block is delivered to the running target through the compiled-makefile
// subprocess bridge.

func Test_Main(t *testing.T) {
	mainScenarios(t)
	mainCases(t)
	mainCompletion(t)
	mainErrors(t)

	t.Run("version flag", func(t *testing.T) {
		// --- Given ---
		old := append([]string(nil), os.Args...)
		t.Cleanup(func() { os.Args = old })

		os.Args = []string{"gomake", "--version"}

		rng := ring.New()

		ctx := t.Context()

		ver := "9.9.9-test"

		empty := builtin.Empty()

		// --- When ---
		have := Main(ctx, rng, ver, empty)

		// --- Then ---
		assert.Equal(t, 0, have)
	})
}

func Test_Main_nestedTmp(t *testing.T) {
	// --- Given ---
	// Nested --tmp must create missing parents (MkdirAll). --list runs past
	// tmp setup without requiring a makefile.
	tst := ringtest.New(t).WetStderr()

	tmp := filepath.Join(t.TempDir(), "a", "b", "c")

	src := t.TempDir()

	rng := tst.Ring("--list", "--tmp", tmp, "--src", src)

	ctx := t.Context()

	ver := "1.0"

	empty := builtin.Empty()

	// --- When ---
	have := Main(ctx, rng, ver, empty)

	// --- Then ---
	assert.Equal(t, 0, have)

	fi, err := os.Stat(tmp)
	assert.NoError(t, err)

	assert.True(t, fi.IsDir())

	_ = tst.Stderr() // empty or target list; just drain WetStderr
}

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
		"  " + prjkit.GoModName + ":\n" +
		"    show:\n" +
		"      message: hello-from-config\n"
	oskit.Write(t, yaml, prj.Root(), configFileName)

	rng := tst.Ring(
		"--src", prj.Root(),
		"--tmp", prj.TempDir(),
		"show",
	)

	bip := builtintest.NewTstProvider()

	ver := "1.2.3"

	// --- When ---
	have := Main(ctx, rng, ver, bip)

	// --- Then ---
	assert.Equal(t, 0, have)

	assert.Equal(t, "message=hello-from-config", tst.Stdout())
}

func Test_Main_setsContractEnv(t *testing.T) {
	// --- Given ---
	// --version returns early after EnvSet of the public contract keys.
	tst := ringtest.New(t).WetStderr()

	tmp := t.TempDir()

	src := t.TempDir()

	rng := tst.Ring("--version", "--tmp", tmp, "--src", src)

	ver := "9.9.9-env"

	ctx := t.Context()

	empty := builtin.Empty()

	// --- When ---
	have := Main(ctx, rng, ver, empty)

	// --- Then ---
	assert.Equal(t, 0, have)

	assert.Equal(t, ver+"\n", tst.Stderr())

	assert.Equal(t, ver, rng.EnvGet(gomake.VersionEnvKey))

	assert.Equal(t, src, rng.EnvGet(gomake.ProjectDirEnvKey))

	assert.Equal(t, ver, rng.MetaGet(gomake.VersionEnvKey))

	assert.Equal(t, src, rng.MetaGet(gomake.ProjectDirEnvKey))
}

func Test_watchBuildDir(t *testing.T) {
	t.Run("signal removes the directory", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()

		marker := oskit.Write(t, "x", dir, "marker")

		sig := make(chan os.Signal, 1)

		stop := watchBuildDir(dir, sig)
		defer stop()

		// --- When ---
		sig <- syscall.SIGINT

		// --- Then ---
		gone := func() bool {
			_, err := os.Stat(marker)
			return errors.Is(err, fs.ErrNotExist)
		}
		err := check.Wait(
			"1s", gone, check.WithWaitThrottle(10*time.Millisecond),
		)

		assert.NoError(t, err)
	})

	t.Run("stop leaves the directory", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()

		marker := oskit.Write(t, "x", dir, "marker")

		sig := make(chan os.Signal, 1)

		stop := watchBuildDir(dir, sig)

		// --- When ---
		stop()

		// --- Then ---
		_, err := os.Stat(marker)
		assert.NoError(t, err)
	})
}

// prePanic is a pre-run hook that panics (exercises main's recover path).

func Test_applyExternalTargetMeta(t *testing.T) {
	t.Run("config injected under the namespace key", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		dir := t.TempDir()

		content := "imports:\n" +
			"  - import: a.com/pkg\n" +
			"    namespace: db\n" +
			"    config:\n" +
			"      host: db.internal\n"
		oskit.Write(t, content, dir, TargetsFile)

		// --- When ---
		err := applyExternalTargetMeta(rng, dir)

		// --- Then ---
		assert.NoError(t, err)

		cfg, ok := rng.MetaLookup("db")
		assert.True(t, ok)

		assert.Equal(t, `{"host":"db.internal"}`, cfg)
	})

	t.Run("config injected under the path base key", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		dir := t.TempDir()

		content := "imports:\n" +
			"  - import: a.com/pkg\n" +
			"    config:\n" +
			"      host: db.internal\n"
		oskit.Write(t, content, dir, TargetsFile)

		// --- When ---
		err := applyExternalTargetMeta(rng, dir)

		// --- Then ---
		assert.NoError(t, err)

		cfg, ok := rng.MetaLookup("pkg")
		assert.True(t, ok)

		assert.Equal(t, `{"host":"db.internal"}`, cfg)
	})

	t.Run("import without config sets no meta", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		dir := t.TempDir()

		content := "imports:\n  - import: a.com/pkg\n    namespace: db\n"
		oskit.Write(t, content, dir, TargetsFile)

		// --- When ---
		err := applyExternalTargetMeta(rng, dir)

		// --- Then ---
		assert.NoError(t, err)

		_, ok := rng.MetaLookup("db")
		assert.False(t, ok)
	})

	t.Run("absent targets file is a no-op", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		dir := t.TempDir()

		// --- When ---
		err := applyExternalTargetMeta(rng, dir)

		// --- Then ---
		assert.NoError(t, err)

		_, ok := rng.MetaLookup("pkg")
		assert.False(t, ok)
	})

	t.Run("malformed targets file is a no-op", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		dir := t.TempDir()
		oskit.Write(t, "{bad yaml}", dir, TargetsFile)

		// --- When ---
		err := applyExternalTargetMeta(rng, dir)

		// --- Then ---
		assert.NoError(t, err)

		_, ok := rng.MetaLookup("pkg")
		assert.False(t, ok)
	})

	t.Run("error - two configs share a meta key", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		dir := t.TempDir()

		content := "" +
			"imports:\n" +
			"  - import: a.com/db\n" +
			"    config:\n" +
			"      host: a\n" +
			"  - import: b.com/db\n" +
			"    config:\n" +
			"      host: b\n"
		oskit.Write(t, content, dir, TargetsFile)

		// --- When ---
		err := applyExternalTargetMeta(rng, dir)

		// --- Then ---
		assert.ErrorIs(t, errDupMetaKey, err)

		_, ok := rng.MetaLookup("db")
		assert.False(t, ok)
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
		have := runWithoutCompile(ctx, rng, ver, tgs)

		// --- Then ---
		assert.NoError(t, have)

		assert.Equal(t, "[abc]", tst.Stdout())
	})

	t.Run("makefile error", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()

		rng := ringtest.New(t).Ring("--unknown", ":tgt-a")

		ver := "1.2.3"

		tgs := builtintest.NewTstProvider().Targets()

		// --- When ---
		have := runWithoutCompile(ctx, rng, ver, tgs)

		// --- Then ---
		assert.Error(t, have)
		_, ok := errors.AsType[plainExit](have)
		assert.True(t, ok)
		want := "" +
			"parsing flags: flag provided but " +
			"not defined: -unknown"
		assert.Equal(t, want, have.Error())
	})

	t.Run("execute error", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()

		rng := ringtest.New(t).Ring(":panic-string")

		ver := "1.2.3"

		bip := builtintest.NewTstProvider().Targets()

		// --- When ---
		have := runWithoutCompile(ctx, rng, ver, bip)

		// --- Then ---
		assert.Error(t, have)
		_, ok := errors.AsType[plainExit](have)
		assert.False(t, ok)
		assert.Equal(t, "target panicked with: panic string", have.Error())
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
