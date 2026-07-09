// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/pathkit"
	"github.com/ctx42/testkit/pkg/subkit"

	gmt "github.com/ctx42/gomake/internal/cli/clitest"
	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/internal/parser"
)

func Test_newConfig(t *testing.T) {
	// Tripwire: a new config field must gain an assertion below, not just a
	// bumped count.
	assert.Fields(t, 19, config{})

	t.Run("no args", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		var args []string

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, must.Value(os.Getwd()), cfg.src)
		assert.Equal(t, must.Value(os.Getwd()), cfg.wd)
		assert.Empty(t, cfg.bin)
		assert.True(t, filepath.IsAbs(cfg.tmp))
		assert.Equal(t, filepath.Join(os.TempDir(), binName), cfg.tmp)
		assert.Equal(t, time.Duration(0), cfg.timeout)
		assert.False(t, cfg.showHelp)
		assert.False(t, cfg.showList)
		assert.False(t, cfg.showVersion)
		assert.False(t, cfg.showComplete)
		assert.False(t, cfg.showCheckConfig)
		assert.Equal(t, runtime.GOOS, cfg.goos)
		assert.Equal(t, runtime.GOARCH, cfg.goarch)
		assert.Equal(t, "1.2.3", cfg.version)
		assert.NotNil(t, cfg.fs)
		assert.Nil(t, cfg.args)
		assert.Equal(t, -1, cfg.targetIdx)
		assert.Empty(t, cfg.target)
		assert.Equal(t, 0, len(cfg.userTargets))
		assert.Equal(t, 0, len(cfg.projectTargets))
	})

	t.Run("option work dir absolute", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--wd", "/dir/path"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/dir/path", cfg.wd)
		assert.Equal(t, []string{"--wd", "/dir/path"}, cfg.args)
	})

	t.Run("option work dir relative", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--wd", "dir"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(must.Value(os.Getwd()), "dir"), cfg.wd)

		wantWD := filepath.Join(must.Value(os.Getwd()), "dir")
		assert.Equal(t, []string{"--wd", wantWD}, cfg.args)
	})

	t.Run("option work dir equal to current working dir", func(t *testing.T) {
		// --- Given ---
		wd := must.Value(os.Getwd())
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--wd", wd, "--timeout", "1s"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, wd, cfg.wd)
		assert.Equal(t, []string{"--timeout", "1s"}, cfg.args)
	})

	t.Run("error getting working dir", func(t *testing.T) {
		if runtime.GOOS == "darwin" {
			t.Skip("skipping test on darwin")
		}

		// --- SUBPROCESS SETUP ---
		sub := subkit.New(t.Name())
		if sub.InMainProcess() {
			// --- TEST SUBPROCESS OUTPUT AND ERROR ---
			sout, eout, err := sub.Run()
			if !assert.NoError(t, err) {
				t.Log(err.Error())
				t.Log(sout, eout)
			}
			return
		}
		// --- IN SUBPROCESS ---

		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		var args []string

		dir := t.TempDir()
		assert.NoError(t, os.Chdir(dir))
		assert.NoError(t, os.Remove(dir))

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorContain(t, "config: getwd: no such file or directory", err)
		assert.Nil(t, cfg)
	})

	t.Run("tmp dir set from environment", func(t *testing.T) {
		// --- Given ---
		env := []string{envKeyTmpDir + "=/dir"}
		tst := ringtest.New(t, ring.WithEnv(env))
		var args []string

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/dir", cfg.tmp)
	})

	t.Run("tmp dir set from environment must be absolute path",
		func(t *testing.T) {
			// --- Given ---
			env := []string{envKeyTmpDir + "=dir"}
			tst := ringtest.New(t, ring.WithEnv(env))
			var args []string

			// --- When ---
			cfg, err := newConfig("1.2.3", tst.Ring(args...))

			// --- Then ---
			assert.ErrorIs(t, parser.ErrAbsPath, err)
			assert.ErrorContain(t, ": dir", err)
			assert.Nil(t, cfg)
		})

	t.Run("empty tmp dir from environment not considered", func(t *testing.T) {
		// --- Given ---
		env := []string{envKeyTmpDir + "="}
		tst := ringtest.New(t, ring.WithEnv(env))
		var args []string

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(os.TempDir(), binName), cfg.tmp)
	})

	t.Run("tmp dir set from option", func(t *testing.T) {
		// --- Given ---
		env := make([]string, 0)
		tst := ringtest.New(t, ring.WithEnv(env))
		args := []string{"--tmp", "/dir"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/dir", cfg.tmp)
	})

	t.Run("tmp dir set from option must be absolute path", func(t *testing.T) {
		// --- Given ---
		env := make([]string, 0, 1)
		tst := ringtest.New(t, ring.WithEnv(env))
		args := []string{"--tmp", "dir"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorIs(t, parser.ErrAbsPath, err)
		assert.ErrorContain(t, ": dir", err)
		assert.Nil(t, cfg)
	})

	t.Run("tmp dir set from option and environment", func(t *testing.T) {
		// --- Given ---
		env := []string{envKeyTmpDir + "=/dir-env"}
		tst := ringtest.New(t, ring.WithEnv(env))
		args := []string{"--tmp", "/dir-arg"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/dir-arg", cfg.tmp)
	})

	t.Run("option src absolute", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--src", "/dir/path"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/dir/path", cfg.src)
	})

	t.Run("option src must be absolute", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--src", "dir"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)

		expSrc := filepath.Join(must.Value(os.Getwd()), "dir")
		assert.Equal(t, expSrc, cfg.src)
	})

	t.Run("set option bin", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--bin", "/dir/makefile"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/dir/makefile", cfg.bin)
	})

	t.Run("option bin relative changed to absolute at current work dir",
		func(t *testing.T) {
			// --- Given ---
			tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
			args := []string{"--bin", "testdata/my-gomake-makefile"}

			// --- When ---
			cfg, err := newConfig("1.2.3", tst.Ring(args...))

			// --- Then ---
			assert.NoError(t, err)
			want := pathkit.AbsPath(t, "testdata/my-gomake-makefile")
			assert.Equal(t, want, cfg.bin)
		})

	t.Run("option wd does not impact option bin ", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		pth := "/dir/my-gomake-makefile"
		args := []string{"--wd", "/tmp", "--bin", pth}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, pth, cfg.bin)
	})

	t.Run("option bin must point to not existing file", func(t *testing.T) {
		// --- Given ---
		pth := pathkit.AbsPath(t, "testdata/makefile")
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--bin", pth}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorIs(t, errBinExists, err)
		assert.ErrorContain(t, pth, err)
		assert.Nil(t, cfg)
	})

	t.Run("option bin cannot be used with -h", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(os.TempDir(), "my-gomake-makefile")
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"-h", "--bin", pth}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorEqual(t, "-h, --help cannot be used with --bin", err)
		assert.Nil(t, cfg)
	})

	t.Run("option bin cannot be used with --help", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(os.TempDir(), "my-gomake-makefile")
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--help", "--bin", pth}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorEqual(t, "-h, --help cannot be used with --bin", err)
		assert.Nil(t, cfg)
	})

	t.Run("option bin cannot be used with --version", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(os.TempDir(), "my-gomake-makefile")
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--version", "--bin", pth}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorEqual(t, "--version cannot be used with --bin", err)
		assert.Nil(t, cfg)
	})

	t.Run("option bin cannot be used with --list", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(os.TempDir(), "my-gomake-makefile")
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--list", "--bin", pth}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorEqual(t, "--list cannot be used with --bin", err)
		assert.Nil(t, cfg)
	})

	t.Run("option bin cannot be used with target name", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(os.TempDir(), "my-gomake-makefile")
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--bin", pth, "target"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorEqual(t, "--bin cannot be used with targets", err)
		assert.Nil(t, cfg)
	})

	t.Run("option timeout", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--timeout", "1m"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Duration(t, "1m", cfg.timeout)
	})

	t.Run("option timeout invalid", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--timeout", "abc"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorIs(t, mkf.ErrInvTimeout, err)
		assert.Nil(t, cfg)
	})

	t.Run("option help long", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--help"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, cfg.showHelp)
	})

	t.Run("option help short", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"-h"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, cfg.showHelp)
	})

	t.Run("option version", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--version"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, cfg.showVersion)
	})

	t.Run("option list", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--list"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, cfg.showList)
	})

	t.Run("gomake options and target name with options", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--timeout", "1s", "target", "--arg0", "--arg1"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		wantArgs := []string{
			"--timeout",
			"1s",
			"target",
			"--arg0",
			"--arg1",
		}
		assert.Equal(t, wantArgs, cfg.args)
		assert.Equal(t, "target", cfg.target)
		assert.Equal(t, 2, cfg.targetIdx)
	})

	t.Run("unknown option before target name", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--unknown", "target", "--arg0"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		assert.Nil(t, cfg)
	})

	t.Run("target field set after options", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--help", "target"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"--help", "target"}, cfg.args)
		assert.Equal(t, "target", cfg.target)
		assert.Equal(t, 1, cfg.targetIdx)
	})

	t.Run("default target with options", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--wd", "/dir"}

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", cfg.target)
		assert.Equal(t, -1, cfg.targetIdx)
	})

	t.Run("default target without options", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		var args []string

		// --- When ---
		cfg, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", cfg.target)
		assert.Equal(t, -1, cfg.targetIdx)
	})
}
