// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/randkit"

	gmt "github.com/ctx42/gomake/internal/cli/clitest"
	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/internal/parser"
)

func Test_newGoMake(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)

		relPth := "testdata/projects/showcase_imports/project"
		absPth := modkit.Path(relPth)

		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPth)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--tmp", prj.TempDir(),
			"--src", prj.Root(),
		)
		cfg := must.Value(newConfig("1.2.3", rng))
		rng = rng.SetArgs(cfg.args)

		// --- When ---
		gmk, err := newGoMake(rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		assert.Same(t, cfg, gmk.cfg)
		assert.Equal(t, cfg.src, gmk.cu.SourceDir)
		assert.Equal(t, cfg.tmp, gmk.cu.BuildRootDir)
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
		assert.Equal(t, wantNames, gmk.targets.Names())
	})

	t.Run("prepare error", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)

		relPth := "testdata/projects/no_makefile/project"
		absPth := modkit.Path(relPth)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPth)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--tmp", prj.TempDir(),
			"--src", prj.Root(),
		)
		cfg := must.Value(newConfig("1.2.3", rng))
		rng = rng.SetArgs(cfg.args)

		// --- When ---
		gmk, err := newGoMake(rng, cfg)

		// --- Then ---
		assert.ErrorIs(t, errNoMakefile, err)
		assert.Nil(t, gmk)
	})

	t.Run("parsing error", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)

		relPth := "testdata/projects/dup_imported/project"
		absPth := modkit.Path(relPth)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPth)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--tmp", prj.TempDir(),
			"--src", prj.Root(),
		)
		cfg := must.Value(newConfig("1.2.3", rng))
		rng = rng.SetArgs(cfg.args)

		// --- When ---
		gmk, err := newGoMake(rng, cfg)

		// --- Then ---
		assert.ErrorIs(t, parser.ErrDupTarget, err)
		assert.Nil(t, gmk)
	})

	t.Run("on error defer removes the build directory", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)

		relPth := "testdata/projects/dup_imported/project"
		absPth := modkit.Path(relPth)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPth)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--tmp", prj.TempDir(),
			"--src", prj.Root(),
		)
		cfg := must.Value(newConfig("1.2.3", rng))
		rng = rng.SetArgs(cfg.args)

		// --- When ---
		gmk, err := newGoMake(rng, cfg)

		// --- Then ---
		assert.Error(t, err)
		assert.Nil(t, gmk)
		assert.Len(t, 0, oskit.List(t, prj.TempDir()))
	})

	t.Run("GOOS GOARCH", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)

		relPth := "testdata/projects/arch_os_build_tag/project"
		absPth := modkit.Path(relPth)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPth)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--tmp", prj.TempDir(),
			"--src", prj.Root(),
		)
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "386")
		cfg := must.Value(newConfig("1.2.3", rng))
		rng = rng.SetArgs(cfg.args)

		// --- When ---
		gmk, err := newGoMake(rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		wantNames := []string{
			"bye",
			"target-arch",
			"target-main",
			"target-os",
			"target-os-arch",
		}
		assert.Equal(t, wantNames, gmk.targets.Names())
	})

	t.Run("--bin does not change the compilation unit MainBin path",
		func(t *testing.T) {
			// --- Given ---
			tst := ringtest.New(t)

			relPth := "testdata/projects/showcase_targets/project"
			absPth := modkit.Path(relPth)
			prj := gmt.NewProject(t)
			prj.GoModInit()
			prj.MakefilesFrom(absPth)
			prj.UseGomakeSrc(modkit.Root())
			prj.GoModTidy()
			prj.Close()

			bin := filepath.Join(t.TempDir(), "my-bin")
			rng := tst.Ring(
				"--tmp", prj.TempDir(),
				"--src", prj.Root(),
				"--bin", bin,
			)
			cfg := must.Value(newConfig("1.2.3", rng))
			rng = rng.SetArgs(cfg.args)

			// --- When ---
			gmk, err := newGoMake(rng, cfg)

			// --- Then ---
			assert.NoError(t, err)
			assert.Equal(t, mkf.MakefileBin, filepath.Base(gmk.cu.MainBin))
			assert.Equal(t, gmk.cu.BuildDir, filepath.Dir(gmk.cu.MainBin))
		})
}

func Test_goMake_Compile(t *testing.T) {
	// --- Given ---
	ctx := t.Context()
	tst := ringtest.New(t)

	absPth := modkit.Path("testdata/projects/showcase_targets/project")
	prj := gmt.NewProject(t)
	prj.GoModInit()
	prj.MakefilesFrom(absPth)
	prj.UseGomakeSrc(modkit.Root())
	prj.GoModTidy()
	prj.Close()

	bin := filepath.Join(t.TempDir(), "my-bin")
	rng := tst.Ring("--tmp", prj.TempDir(), "--src", prj.Root())
	cfg := must.Value(newConfig("1.2.3", rng))
	rng = rng.SetArgs(cfg.args)
	gmk := must.Value(newGoMake(rng, cfg))

	// --- When ---
	have, err := gmk.Compile(ctx, rng.EnvAll(), bin)

	// --- Then ---
	assert.NoError(t, err)
	assert.Equal(t, bin, have)
	assert.FileExist(t, bin)
}

func Test_goMake_Execute(t *testing.T) {
	t.Run("show version", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPth := "testdata/projects/showcase_imports/project"
		absPth := modkit.Path(relPth)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPth)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--tmp", prj.TempDir(),
			"--src", prj.Root(),
			"--version",
		)
		cfg := must.Value(newConfig("1.2.3", rng))
		rng = rng.SetArgs(cfg.args)
		gmk := must.Value(newGoMake(rng, cfg))

		// --- When ---
		err := gmk.Execute(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "1.2.3\n", tst.Stderr())
	})

	t.Run("show env", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		relPth := "testdata/projects/showcase_targets/project"
		absPth := modkit.Path(relPth)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPth)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		kv := randkit.Str()
		rng := tst.Ring(
			"--tmp", prj.TempDir(),
			"--src", prj.Root(),
			"basic-env", kv,
		)
		rng.EnvSet(kv, kv)
		cfg := must.Value(newConfig("1.2.3", rng))
		rng = rng.SetArgs(cfg.args)
		gmk := must.Value(newGoMake(rng, cfg))

		// --- When ---
		err := gmk.Execute(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := fmt.Sprintf("BasicEnv %s=%s", kv, kv)
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("run error", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		relPth := "testdata/projects/no_targets/project"
		absPth := modkit.Path(relPth)
		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(absPth)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := tst.Ring(
			"--tmp", prj.TempDir(),
			"--src", prj.Root(),
		)
		cfg := must.Value(newConfig("1.2.3", rng))
		rng = rng.SetArgs(cfg.args)
		gmk := must.Value(newGoMake(rng, cfg))

		// --- When ---
		err := gmk.Execute(ctx, rng)

		// --- Then ---
		assert.ExitCode(t, mkf.ExitCodePickTarget, err)
		assert.Equal(t, mkf.ErrPickTarget.Error()+"\n", tst.Stderr())
	})
}
