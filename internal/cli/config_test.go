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
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, must.Value(os.Getwd()), have.src)
		assert.Equal(t, must.Value(os.Getwd()), have.wd)
		assert.Empty(t, have.bin)
		assert.True(t, filepath.IsAbs(have.tmp))
		assert.Equal(t, filepath.Join(os.TempDir(), binName), have.tmp)
		assert.Equal(t, time.Duration(0), have.timeout)
		assert.False(t, have.showHelp)
		assert.False(t, have.showList)
		assert.False(t, have.showVersion)
		assert.False(t, have.showComplete)
		assert.False(t, have.showCheckConfig)
		assert.Equal(t, runtime.GOOS, have.goos)
		assert.Equal(t, runtime.GOARCH, have.goarch)
		assert.Equal(t, "1.2.3", have.version)
		assert.NotNil(t, have.fs)
		assert.Nil(t, have.args)
		assert.Equal(t, -1, have.targetIdx)
		assert.Empty(t, have.target)
		assert.Equal(t, 0, len(have.userTargets))
		assert.Equal(t, 0, len(have.projectTargets))
	})

	t.Run("option work dir absolute", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--wd", "/dir/path"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/dir/path", have.wd)
		assert.Equal(t, []string{"--wd", "/dir/path"}, have.args)
	})

	t.Run("option work dir relative", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--wd", "dir"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(must.Value(os.Getwd()), "dir"), have.wd)
		wantWD := filepath.Join(must.Value(os.Getwd()), "dir")
		assert.Equal(t, []string{"--wd", wantWD}, have.args)
	})

	t.Run("option work dir equal to current working dir", func(t *testing.T) {
		// --- Given ---
		wd := must.Value(os.Getwd())
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--wd", wd, "--timeout", "1s"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, wd, have.wd)
		assert.Equal(t, []string{"--timeout", "1s"}, have.args)
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
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorContain(t, "config: getwd: no such file or directory", err)
		assert.Nil(t, have)
	})

	t.Run("tmp dir set from environment", func(t *testing.T) {
		// --- Given ---
		env := []string{envKeyTmpDir + "=/dir"}
		tst := ringtest.New(t, ring.WithEnv(env))
		var args []string

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/dir", have.tmp)
	})

	t.Run("tmp dir set from environment must be absolute path",
		func(t *testing.T) {
			// --- Given ---
			env := []string{envKeyTmpDir + "=dir"}
			tst := ringtest.New(t, ring.WithEnv(env))
			var args []string

			// --- When ---
			have, err := newConfig("1.2.3", tst.Ring(args...))

			// --- Then ---
			assert.ErrorIs(t, parser.ErrAbsPath, err)
			assert.ErrorContain(t, ": dir", err)
			assert.Nil(t, have)
		})

	t.Run("empty tmp dir from environment not considered", func(t *testing.T) {
		// --- Given ---
		env := []string{envKeyTmpDir + "="}
		tst := ringtest.New(t, ring.WithEnv(env))
		var args []string

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(os.TempDir(), binName), have.tmp)
	})

	t.Run("tmp dir set from option", func(t *testing.T) {
		// --- Given ---
		env := make([]string, 0)
		tst := ringtest.New(t, ring.WithEnv(env))
		args := []string{"--tmp", "/dir"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/dir", have.tmp)
	})

	t.Run("tmp dir set from option must be absolute path", func(t *testing.T) {
		// --- Given ---
		env := make([]string, 0, 1)
		tst := ringtest.New(t, ring.WithEnv(env))
		args := []string{"--tmp", "dir"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorIs(t, parser.ErrAbsPath, err)
		assert.ErrorContain(t, ": dir", err)
		assert.Nil(t, have)
	})

	t.Run("tmp dir set from option and environment", func(t *testing.T) {
		// --- Given ---
		env := []string{envKeyTmpDir + "=/dir-env"}
		tst := ringtest.New(t, ring.WithEnv(env))
		args := []string{"--tmp", "/dir-arg"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/dir-arg", have.tmp)
	})

	t.Run("option src absolute", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--src", "/dir/path"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/dir/path", have.src)
	})

	t.Run("option src must be absolute", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--src", "dir"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(must.Value(os.Getwd()), "dir"), have.src)
	})

	t.Run("set option bin", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--bin", "/dir/makefile"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/dir/makefile", have.bin)
	})

	t.Run("option bin relative changed to absolute at current work dir",
		func(t *testing.T) {
			// --- Given ---
			tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
			args := []string{"--bin", "testdata/my-gomake-makefile"}

			// --- When ---
			have, err := newConfig("1.2.3", tst.Ring(args...))

			// --- Then ---
			assert.NoError(t, err)
			want := pathkit.AbsPath(t, "testdata/my-gomake-makefile")
			assert.Equal(t, want, have.bin)
		})

	t.Run("option wd does not impact option bin ", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		pth := "/dir/my-gomake-makefile"
		args := []string{"--wd", "/tmp", "--bin", pth}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, pth, have.bin)
	})

	t.Run("option bin must point to not existing file", func(t *testing.T) {
		// --- Given ---
		pth := pathkit.AbsPath(t, "testdata/makefile")
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--bin", pth}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorIs(t, errBinExists, err)
		assert.ErrorContain(t, pth, err)
		assert.Nil(t, have)
	})

	t.Run("option bin cannot be used with -h", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(os.TempDir(), "my-gomake-makefile")
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"-h", "--bin", pth}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorEqual(t, "-h, --help cannot be used with --bin", err)
		assert.Nil(t, have)
	})

	t.Run("option bin cannot be used with --help", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(os.TempDir(), "my-gomake-makefile")
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--help", "--bin", pth}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorEqual(t, "-h, --help cannot be used with --bin", err)
		assert.Nil(t, have)
	})

	t.Run("option bin cannot be used with --version", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(os.TempDir(), "my-gomake-makefile")
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--version", "--bin", pth}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorEqual(t, "--version cannot be used with --bin", err)
		assert.Nil(t, have)
	})

	t.Run("option bin cannot be used with --list", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(os.TempDir(), "my-gomake-makefile")
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--list", "--bin", pth}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorEqual(t, "--list cannot be used with --bin", err)
		assert.Nil(t, have)
	})

	t.Run("option bin cannot be used with target name", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(os.TempDir(), "my-gomake-makefile")
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--bin", pth, "target"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorEqual(t, "--bin cannot be used with targets", err)
		assert.Nil(t, have)
	})

	t.Run("option timeout", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--timeout", "1m"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Duration(t, "1m", have.timeout)
	})

	t.Run("option timeout invalid", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--timeout", "abc"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorIs(t, mkf.ErrInvTimeout, err)
		assert.Nil(t, have)
	})

	t.Run("option help long", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--help"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have.showHelp)
	})

	t.Run("option help short", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"-h"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have.showHelp)
	})

	t.Run("option version", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--version"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have.showVersion)
	})

	t.Run("option list", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--list"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have.showList)
	})

	t.Run("gomake options and target name with options", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--timeout", "1s", "target", "--arg0", "--arg1"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		wantArgs := []string{
			"--timeout",
			"1s",
			"target",
			"--arg0",
			"--arg1",
		}
		assert.Equal(t, wantArgs, have.args)
		assert.Equal(t, "target", have.target)
		assert.Equal(t, 2, have.targetIdx)
	})

	t.Run("unknown option before target name", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--unknown", "target", "--arg0"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		assert.Nil(t, have)
	})

	t.Run("target field set after options", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--help", "target"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"--help", "target"}, have.args)
		assert.Equal(t, "target", have.target)
		assert.Equal(t, 1, have.targetIdx)
	})

	t.Run("default target with options", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		args := []string{"--wd", "/dir"}

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", have.target)
		assert.Equal(t, -1, have.targetIdx)
	})

	t.Run("default target without options", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv(gmt.TestEnv(t)))
		var args []string

		// --- When ---
		have, err := newConfig("1.2.3", tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", have.target)
		assert.Equal(t, -1, have.targetIdx)
	})
}
