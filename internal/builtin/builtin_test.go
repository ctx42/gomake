// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package builtin

import (
	"os"
	"strings"
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
	assert.Len(t, 0, have.Targets())
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
		assert.NotSame(t, pre, have.PreRuns())
	})

	t.Run("no targets", func(t *testing.T) {
		// --- Given ---
		src := builtintest.TestTargetsSrc()

		items := []*mkf.Target{}

		// --- When ---
		have := newTargets(items, src, nil)

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
	opts := &genOpts{}

	// --- When ---
	WithGenName("name")(opts)

	// --- Then ---
	assert.Equal(t, "name", opts.name)
}

func Test_validGoIdent_tabular(t *testing.T) {
	tt := []struct {
		testN string
		name  string
		want  bool
	}{
		{"plain", "builtin", true},
		{"digits", "go1", true},
		{"empty", "", false},
		{"blank", "_", false},
		{"dash", "not-a-name", false},
		{"keyword", "package", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := validGoIdent(tc.name)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_WithGenEnv(t *testing.T) {
	// --- Given ---
	rng := ring.New()

	opts := &genOpts{}

	// --- When ---
	WithGenEnv(rng)(opts)

	// --- Then ---
	assert.Equal(t, rng, opts.rng)
}

func Test_WithGenDst(t *testing.T) {
	// --- Given ---
	opts := &genOpts{}

	// --- When ---
	WithGenDst("/path/to")(opts)

	// --- Then ---
	assert.Equal(t, "/path/to", opts.dst)
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
		assert.True(t, strings.HasPrefix(text, "package abc\n"))

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
	t.Run("namespace prefixes generated target names", func(t *testing.T) {
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

	t.Run("error - package name is not an identifier", func(t *testing.T) {
		// --- Given ---
		prj := clitest.NewProject(t)
		prj.Close()

		name := "not-a-name"

		withGenName := WithGenName(name)

		withGenDst := WithGenDst(prj.Root())

		// --- When ---
		err := GenImports(nil, withGenName, withGenDst)

		// --- Then ---
		assert.ErrorContain(t, name, err)

		assert.NoFileExist(t, prj.Path(targetsFN))
	})

	t.Run("write failure leaves existing files", func(t *testing.T) {
		// --- Given ---
		prj := clitest.NewProject(t)
		prj.Close()

		original := "package original\n"
		oskit.Write(t, original, prj.Root(), targetsFN)

		data := oskit.MkdirAll(t, prj.Root(), "data")
		must.Nil(os.Chmod(data, 0o555))
		t.Cleanup(func() { _ = os.Chmod(data, 0o755) })

		imports := []parser.Import{
			{Path: "github.com/ctx42/gomake/testdata/imports/pkg0"},
		}

		withGenDst := WithGenDst(prj.Root())

		// --- When ---
		err := GenImports(imports, withGenDst)

		// --- Then ---
		assert.ErrorContain(t, mainFN, err)

		text := oskit.ReadFileStr(t, prj.Path(targetsFN))
		assert.Equal(t, original, text)
	})
}
