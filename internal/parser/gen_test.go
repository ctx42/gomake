// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/goldy"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"
)

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

func Test_WithGenNames(t *testing.T) {
	// --- Given ---
	opts := &genOpts{}

	// --- When ---
	WithGenNames("pkg", "method")(opts)

	// --- Then ---
	assert.Equal(t, "pkg", opts.pkg)
	assert.Equal(t, "method", opts.suffix)
}

func Test_WithGenReg(t *testing.T) {
	// --- Given ---
	opts := &genOpts{}

	// --- When ---
	WithGenReg(opts)

	// --- Then ---
	assert.True(t, opts.register)
}

func Test_defGenOpts(t *testing.T) {
	// --- When ---
	def := defGenOpts()

	// --- Then ---
	assert.Equal(t, "main", def.pkg)
	assert.Equal(t, "Main", def.suffix)
	assert.False(t, def.register)
}

func Test_NewGenerator(t *testing.T) {
	// --- Given ---
	tgs := NewTargets()

	// --- When ---
	have := NewGenerator(tgs)

	// --- Then ---
	assert.Same(t, tgs, have.tgs)
}

func Test_Generator_Generate(t *testing.T) {
	t.Run("builtin targets for abc package", func(t *testing.T) {
		// --- Given ---
		impSpecs := []string{
			"github.com/ctx42/gomake/testdata/imports/pkg0",
			"github.com/ctx42/gomake/testdata/imports/pkg1",
			"github.com/ctx42/gomake/testdata/imports/pkg7",
		}
		gfp := "testdata/gen_builtin_targets_abc.gld"
		gfd := map[string]any{"prj_root": modkit.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))

		rng := ring.New()
		tgs := must.Value(TargetsFromSpecs(rng, impSpecs, BuiltInCB))

		// --- When ---
		have, err := NewGenerator(tgs).Generate(WithGenNames("abc", "Abc"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, gld.String(), string(have))
	})

	t.Run("error - empty suffix", func(t *testing.T) {
		// --- When ---
		have, err := NewGenerator(nil).Generate(WithGenNames("abc", ""))

		// --- Then ---
		assert.ErrorEqual(t, "generated method suffix cannot be empty", err)
		assert.Nil(t, have)
	})

	t.Run("builtin targets for the main package", func(t *testing.T) {
		// --- Given ---
		impSpecs := []string{
			"github.com/ctx42/gomake/testdata/imports/pkg0",
			"github.com/ctx42/gomake/testdata/imports/pkg1",
			"github.com/ctx42/gomake/testdata/imports/pkg7",
		}
		gfp := "testdata/gen_builtin_targets_main.gld"
		gfd := map[string]any{"prj_root": modkit.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))

		rng := ring.New()
		tgs := must.Value(TargetsFromSpecs(rng, impSpecs, BuiltInCB))

		// --- When ---
		have, err := NewGenerator(tgs).Generate()

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, gld.String(), string(have))
	})

	t.Run("register targets", func(t *testing.T) {
		// --- Given ---
		impSpecs := []string{
			"github.com/ctx42/gomake/testdata/imports/pkg0",
			"github.com/ctx42/gomake/testdata/imports/pkg1",
			"github.com/ctx42/gomake/testdata/imports/pkg7",
		}
		gfp := "testdata/gen_builtin_targets_main_registered.gld"
		gfd := map[string]any{"prj_root": modkit.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))

		rng := ring.New()
		tgs := must.Value(TargetsFromSpecs(rng, impSpecs, BuiltInCB))

		// --- When ---
		have, err := NewGenerator(tgs).Generate(WithGenReg)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, gld.String(), string(have))
	})

	t.Run("no targets abc", func(t *testing.T) {
		// --- Given ---
		gfp := "testdata/gen_no_targets_abc.gld"
		gfd := map[string]any{"prj_root": modkit.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))

		rng := ring.New()
		tgs := must.Value(TargetsFromSpecs(rng, nil))

		// --- When ---
		have, err := NewGenerator(tgs).Generate(WithGenNames("abc", "Abc"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, gld.String(), string(have))
	})

	t.Run("no targets main", func(t *testing.T) {
		// --- Given ---
		gfp := "testdata/gen_no_targets_main.gld"
		gfd := map[string]any{"prj_root": modkit.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))

		rng := ring.New()
		tgs := must.Value(TargetsFromSpecs(rng, nil))

		// --- When ---
		have, err := NewGenerator(tgs).Generate()

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, gld.String(), string(have))
	})

	t.Run("import without targets", func(t *testing.T) {
		// --- Given ---
		impSpecs := []string{
			"github.com/ctx42/gomake/testdata/projects/no_targets/project",
		}

		gfp := "testdata/gen_no_targets_abc.gld"
		gfd := map[string]any{"prj_root": modkit.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))

		rng := ring.New()
		tgs := must.Value(TargetsFromSpecs(rng, impSpecs, BuiltInCB))

		// --- When ---
		have, err := NewGenerator(tgs).Generate(WithGenNames("abc", "Abc"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, gld.String(), string(have))
	})
}
