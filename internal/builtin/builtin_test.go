// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package builtin

import (
	"strings"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/goldy"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"

	"github.com/ctx42/gomake/internal/builtin/builtintest"
	"github.com/ctx42/gomake/internal/cli/clitest"
	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/internal/parser"
)

func Test_Empty(t *testing.T) {
	// --- When ---
	tgs := Empty()

	// --- Then ---
	assert.Nil(t, tgs.Targets())
	assert.Equal(t, tgsMainEmptySrc, tgs.Source())
}

func Test_Generated(t *testing.T) {
	// --- When ---
	tgs := Generated()

	// --- Then ---
	assert.Len(t, 0, tgs.Targets())
}

func Test_newTargets(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		want := builtintest.TestTargets()
		src := builtintest.TestTargetsSrc()
		pre := []mkf.PreRunFn{builtintest.TestPreRun}

		// --- When ---
		tgs := newTargets(want, src, pre)

		// --- Then ---
		assert.Same(t, want, tgs.Targets())
		assert.Equal(t, string(src), string(tgs.Source()))
		assert.Len(t, 1, tgs.PreRuns())
		assert.Same(t, pre[0], tgs.PreRuns()[0])
	})

	t.Run("when no targets mainEmptyFN always used", func(t *testing.T) {
		// --- Given ---
		src := builtintest.TestTargetsSrc()

		// --- When ---
		tgs := newTargets([]*mkf.Target{}, src, nil)

		// --- Then ---
		assert.Len(t, 0, tgs.Targets())
		assert.Equal(t, string(tgsMainEmptySrc), string(tgs.Source()))
	})

	t.Run("all arguments nil", func(t *testing.T) {
		// --- When ---
		tgs := newTargets(nil, nil, nil)

		// --- Then ---
		assert.Len(t, 0, tgs.Targets())
		assert.Equal(t, string(tgsMainEmptySrc), string(tgs.Source()))
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
	assert.Same(t, want, have)
}

func Test_targets_PreRuns(t *testing.T) {
	// --- Given ---
	pre := []mkf.PreRunFn{builtintest.TestPreRun}
	tgs := newTargets(nil, nil, pre)

	// --- When ---
	have := tgs.PreRuns()

	// --- Then ---
	assert.Equal(t, pre, have)
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

	t.Run("returns a clone the caller cannot mutate", func(t *testing.T) {
		// --- Given ---
		want := builtintest.TestTargets()
		src := builtintest.TestTargetsSrc()
		tgs := newTargets(want, src, nil)

		// --- When ---
		have := tgs.Source()
		have[0]++

		// --- Then ---
		assert.Equal(t, string(src), string(tgs.Source()))
	})
}

func Test_WithGenName(t *testing.T) {
	// --- Given ---
	def := &genOpts{}

	// --- When ---
	WithGenName("name")(def)

	// --- Then ---
	assert.Equal(t, "name", def.name)
}

func Test_WithGenEnv(t *testing.T) {
	// --- Given ---
	def := &genOpts{}
	rng := ring.New()

	// --- When ---
	WithGenEnv(rng)(def)

	// --- Then ---
	assert.Equal(t, rng, def.rng)
}

func Test_WithGenDst(t *testing.T) {
	// --- Given ---
	def := &genOpts{}

	// --- When ---
	WithGenDst("/path/to")(def)

	// --- Then ---
	assert.Equal(t, "/path/to", def.dst)
}

func Test_WithoutGenEmptySrc(t *testing.T) {
	// --- Given ---
	def := &genOpts{
		empty: true,
	}

	// --- When ---
	WithoutGenEmptySrc(def)

	// --- Then ---
	assert.False(t, def.empty)
}

func Test_GenMain(t *testing.T) {
	t.Run("generate all", func(t *testing.T) {
		// --- Given ---

		prj := clitest.NewProject(t)
		prj.Close()

		impSpecs := []string{
			"github.com/ctx42/gomake/testdata/imports/pkg0",
		}

		gfpB := "testdata/pkg0_builtin_targets.gld"
		gfdB := map[string]any{"prj_root": modkit.Root()}
		gldB := goldy.Open(t, gfpB, goldy.WithData(gfdB))

		gfpM := "testdata/pkg0_main_targets.gld"
		gfdM := map[string]any{"prj_root": modkit.Root()}
		gldM := goldy.Open(t, gfpM, goldy.WithData(gfdM))

		gfpE := "testdata/pkg0_main_targets_empty.gld"
		gfdE := map[string]any{"prj_root": modkit.Root()}
		gldE := goldy.Open(t, gfpE, goldy.WithData(gfdE))

		opts := []GenOption{
			WithGenDst(prj.Root()),
		}

		// --- When ---
		err := GenMain(impSpecs, opts...)

		// --- Then ---
		assert.NoError(t, err)

		have := oskit.ReadFileStr(t, prj.Path(targetsFN))
		assert.Equal(t, gldB.String(), have)

		have = oskit.ReadFileStr(t, prj.Path("data", mainFN))
		assert.Equal(t, gldM.String(), have)

		have = oskit.ReadFileStr(t, prj.Path("data", mainEmptyFN))
		assert.Equal(t, gldE.String(), have)
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
		have := oskit.ReadFileStr(t, prj.Path(targetsFN))
		assert.True(t, strings.HasPrefix(have, "package abc\n"))

		have = oskit.ReadFileStr(t, prj.Path("data", mainFN))
		assert.True(t, strings.HasPrefix(have, "package main\n"))

		have = oskit.ReadFileStr(t, prj.Path("data", mainEmptyFN))
		assert.True(t, strings.HasPrefix(have, "package main\n"))
	})

	t.Run("invalid import", func(t *testing.T) {
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
