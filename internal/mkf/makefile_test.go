// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mkf

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
)

func Test_InterruptedError_Error(t *testing.T) {
	// --- When ---
	err := interruptedError(123)

	// --- Then ---
	assert.Equal(t, "target interrupted", err.Error())
}

func Test_InterruptedError_Signal(t *testing.T) {
	// --- When ---
	err := interruptedError(123)

	// --- Then ---
	assert.Equal(t, 123, err.Signal())
}

func Test_WithMakefileVersion(t *testing.T) {
	// --- Given ---
	cmf := &Makefile{}

	// --- When ---
	WithMakefileVersion("1.2.3")(cmf)

	// --- Then ---
	assert.Equal(t, "1.2.3", cmf.version)
}

func Test_WithMakefileArgs(t *testing.T) {
	t.Run("no args", func(t *testing.T) {
		// --- Given ---
		cmf := &Makefile{rng: ring.New()}

		// --- When ---
		WithMakefileArgs()(cmf)

		// --- Then ---
		assert.NotNil(t, cmf.rng.Args())
		assert.Empty(t, cmf.rng.Args())
	})

	t.Run("args", func(t *testing.T) {
		// --- Given ---
		args := []string{"a", "b", "c"}
		cmf := &Makefile{rng: ring.New()}

		// --- When ---
		WithMakefileArgs(args...)(cmf)

		// --- Then ---
		assert.Equal(t, []string{"a", "b", "c"}, cmf.rng.Args())
	})
}

func Test_WithMakefileRing(t *testing.T) {
	// --- Given ---
	rng := ring.New()
	cmf := &Makefile{}

	// --- When ---
	WithMakefileRing(rng)(cmf)

	// --- Then ---
	assert.Equal(t, rng, cmf.rng)
}

func Test_NewMakefile(t *testing.T) {
	t.Run("no arguments", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring())

		// Option function which designed to extract a slice of arguments
		// before it is overwritten by WithMakefileArgs option.
		var origArgs []string
		extractArgs := func(cmf *Makefile) { origArgs = cmf.rng.Args() }

		// --- When ---
		cmf, err := NewMakefile(tgs, extractArgs, rngOF, WithMakefileArgs())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, must.Value(os.Getwd()), cmf.wd)
		assert.Equal(t, time.Duration(0), cmf.timeout)
		assert.NotNil(t, cmf.targets)
		assert.NotNil(t, cmf.fs)
		assert.Equal(t, os.Args[1:], origArgs)
		assert.Equal(t, "unknown version", cmf.version)
		assert.False(t, cmf.showHelp)
		assert.Nil(t, cmf.optionTgt)
	})

	t.Run("set a working directory", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("--wd", "/wd/path", "target"))

		// --- When ---
		cmf, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/wd/path", cmf.wd)
		assert.Equal(t, []string{"target"}, cmf.rng.Args())
	})

	t.Run("set timeout", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("--timeout", "7s", "target"))

		// --- When ---
		cmf, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 7*time.Second, cmf.timeout)
		assert.Equal(t, []string{"target"}, cmf.rng.Args())
	})

	t.Run("error - negative timeout", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("--timeout", "-1s", "target"))

		// --- When ---
		cmf, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.ErrorIs(t, ErrInvTimeout, err)
		assert.Nil(t, cmf)
	})

	t.Run("error - invalid timeout argument", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("--timeout", "abc", "target"))

		// --- When ---
		cmf, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.ErrorIs(t, ErrInvTimeout, err)
		assert.Nil(t, cmf)
	})

	t.Run("short help option", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("-h", "target"))

		// --- When ---
		cmf, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, cmf.showHelp)
		assert.Equal(t, []string{"target"}, cmf.rng.Args())
		assert.Nil(t, cmf.optionTgt)
	})

	t.Run("help long", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("--help", "target"))

		// --- When ---
		cmf, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, cmf.showHelp)
		assert.Equal(t, []string{"target"}, cmf.rng.Args())
		assert.Nil(t, cmf.optionTgt)
	})

	t.Run("both short and long help options", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		argsOF := WithMakefileArgs("-h", "--help", "target")

		// --- When ---
		cmf, err := NewMakefile(tgs, argsOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, cmf.showHelp)
		assert.Equal(t, []string{"target"}, cmf.rng.Args())
		assert.Nil(t, cmf.optionTgt)
	})

	t.Run("version option", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()
		rngOF := WithMakefileRing(tst.Ring("--version"))
		verOF := WithMakefileVersion("1.2.3")

		// --- When ---
		cmf, err := NewMakefile(tgs, rngOF, verOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "1.2.3", cmf.version)
		assert.Equal(t, []string{}, cmf.rng.Args())

		assert.NoError(t, cmf.Execute(ctx))
		assert.Equal(t, "1.2.3\n", tst.Stderr())
	})

	t.Run("list option", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{
			{Name: "first", Synopsis: "doc for first", Default: false},
			{Name: "second", Synopsis: "doc for second", Default: true},
		}
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()
		rngOF := WithMakefileRing(tst.Ring("--list"))

		// --- When ---
		cmf, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{}, cmf.rng.Args())

		assert.NoError(t, cmf.Execute(ctx))
		want := "" +
			"first      doc for first\n" +
			"second*    doc for second\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - unknown option", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("--unknown", "target"))

		// --- When ---
		cmf, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		assert.Nil(t, cmf)
	})

	t.Run("error - getting working directory", func(t *testing.T) {
		if runtime.GOOS == "darwin" {
			t.Skip("skipping test on darwin")
		}

		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring())
		argsOF := WithMakefileArgs()
		wd := must.Value(os.Getwd())
		t.Cleanup(func() { _ = os.Chdir(wd) })

		dir := t.TempDir()
		assert.NoError(t, os.Chdir(dir))
		assert.NoError(t, os.Remove(dir))

		// --- When ---
		cmf, err := NewMakefile(tgs, rngOF, argsOF)

		// --- Then ---
		assert.ErrorEqual(t, "getwd: no such file or directory", err)
		assert.Nil(t, cmf)
	})
}

func Test_Makefile_Execute(t *testing.T) {
	t.Run("show target help", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtA()}
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()
		rngOF := WithMakefileRing(tst.Ring("-h", "tgt-a"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(ctx)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "tgt-a\tdoc tgt-a\n", tst.Stderr())
	})

	t.Run("error - help for unknown target", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtA()}
		ctx := t.Context()
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("-h", "unknown"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(ctx)

		// --- Then ---
		assert.ErrorIs(t, ErrUnkTarget, err)
	})

	t.Run("show the version", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtA()}
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()
		rngOF := WithMakefileRing(tst.Ring("--version"))
		verOF := WithMakefileVersion("1.2.3")
		cmf := must.Value(NewMakefile(tgs, rngOF, verOF))

		// --- When ---
		err := cmf.Execute(ctx)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "1.2.3\n", tst.Stderr())
	})

	t.Run("execute target", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtPrintArgs()}
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()
		rngOF := WithMakefileRing(tst.Ring("tgt-print-args", "arg0"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(ctx)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "[arg0]", tst.Stdout())
	})

	t.Run("error - unknown target", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtB()}
		ctx := t.Context()
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("unknown"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(ctx)

		// --- Then ---
		assert.ErrorIs(t, ErrUnkTarget, err)
	})

	t.Run("error - target returning", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtError()}
		ctx := t.Context()
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("tgt-error"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(ctx)

		// --- Then ---
		assert.ErrorEqual(t, "always error", err)
	})

	t.Run("target execution timeout applied", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtLong()}
		ctx := t.Context()
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("--timeout", "50ms", "tgt-long"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(ctx)

		// --- Then ---
		assert.ErrorIs(t, context.DeadlineExceeded, err)
	})

	t.Run("target execution time not limited", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtLong()}
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()
		rngOF := WithMakefileRing(tst.Ring("tgt-long"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(ctx)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "done after 100ms", tst.Stdout())
	})

	t.Run("target knows its name", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtPrintName()}
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()
		rngOF := WithMakefileRing(tst.Ring("tgt-print-name"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(ctx)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "tgt-print-name", tst.Stdout())
	})
}

func Test_fTgtVersion(t *testing.T) {
	t.Run("no empty version", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		// --- When ---
		err := fTgtVersion("1.2.3")(ctx, tst.Ring())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "1.2.3\n", tst.Stderr())
	})

	t.Run("empty version", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		// --- When ---
		err := fTgtVersion("")(ctx, tst.Ring())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "\n", tst.Stderr())
	})
}

func Test_fTgtList(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtCore(), TgtA(), TgtB()}
		ctx := t.Context()
		tst := ringtest.New(t).WetStderr()

		// --- When ---
		err := fTgtList(tgs)(ctx, tst.Ring())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			":tgt-core    syn tgt-core\n" +
			"\n" +
			"tgt-a        syn tgt-a\n" +
			"tgt-b*       syn tgt-b\n"
		assert.Equal(t, want, tst.Stderr())
	})
}
