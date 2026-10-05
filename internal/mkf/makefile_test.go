// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mkf

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
)

func Test_interruptedError_Error(t *testing.T) {
	// --- When ---
	err := interruptedError{sig: syscall.SIGINT}

	// --- Then ---
	assert.Equal(t, "target interrupted", err.Error())
}

func Test_interruptedError_ExitCode(t *testing.T) {
	t.Run("signal number", func(t *testing.T) {
		// --- Given ---
		err := interruptedError{sig: syscall.SIGINT}

		// --- When ---
		have := err.ExitCode()

		// --- Then ---
		assert.Equal(t, 130, have)
	})

	t.Run("no signal number", func(t *testing.T) {
		// --- Given ---
		err := interruptedError{sig: tstSignal{}}

		// --- When ---
		have := err.ExitCode()

		// --- Then ---
		assert.Equal(t, ExitCodeErr, have)
	})
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
		cmf := &Makefile{}

		// --- When ---
		WithMakefileArgs()(cmf)

		// --- Then ---
		assert.NotNil(t, cmf.args)
		assert.Empty(t, cmf.args)
	})

	t.Run("args", func(t *testing.T) {
		// --- Given ---
		cmf := &Makefile{}

		// --- When ---
		WithMakefileArgs("a", "b", "c")(cmf)

		// --- Then ---
		assert.Equal(t, []string{"a", "b", "c"}, cmf.args)
	})
}

func Test_WithMakefileRing(t *testing.T) {
	t.Run("ring", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		cmf := &Makefile{}

		// --- When ---
		WithMakefileRing(rng)(cmf)

		// --- Then ---
		assert.Same(t, rng, cmf.rng)
	})

	t.Run("nil", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		cmf := &Makefile{rng: rng}

		// --- When ---
		WithMakefileRing(nil)(cmf)

		// --- Then ---
		assert.Same(t, rng, cmf.rng)
	})
}

func Test_NewMakefile(t *testing.T) {
	t.Run("no arguments", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring())

		// Option function designed to extract a slice of arguments
		// before it is overwritten by WithMakefileArgs option.
		var origArgs []string
		extractArgs := func(cmf *Makefile) { origArgs = cmf.rng.Args() }

		// --- When ---
		have, err := NewMakefile(tgs, extractArgs, rngOF, WithMakefileArgs())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, must.Value(os.Getwd()), have.wd)
		assert.Equal(t, time.Duration(0), have.timeout)
		assert.NotNil(t, have.targets)
		assert.NotNil(t, have.fs)
		assert.Equal(t, os.Args[1:], origArgs)
		assert.Equal(t, "unknown version", have.version)
		assert.False(t, have.showHelp)
		assert.Nil(t, have.optionTgt)
	})

	t.Run("args before ring", func(t *testing.T) {
		// --- Given ---
		argsOF := WithMakefileArgs("tgt-a")
		rngOF := WithMakefileRing(ring.New())

		// --- When ---
		have, err := NewMakefile([]*Target{TgtA()}, argsOF, rngOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"tgt-a"}, have.rng.Args())
	})

	t.Run("nil ring", func(t *testing.T) {
		// --- Given ---
		rngOF := WithMakefileRing(nil)

		// --- When ---
		have, err := NewMakefile(nil, rngOF, WithMakefileArgs("tgt-a"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"tgt-a"}, have.rng.Args())
	})

	t.Run("set a working directory", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("--wd", "/wd/path", "target"))

		// --- When ---
		have, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/wd/path", have.wd)
		assert.Equal(t, []string{"target"}, have.rng.Args())
	})

	t.Run("set timeout", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("--timeout", "7s", "target"))

		// --- When ---
		have, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 7*time.Second, have.timeout)
		assert.Equal(t, []string{"target"}, have.rng.Args())
	})

	t.Run("error - negative timeout", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("--timeout", "-1s", "target"))

		// --- When ---
		have, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.ErrorIs(t, ErrInvTimeout, err)
		assert.Nil(t, have)
	})

	t.Run("error - invalid timeout argument", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("--timeout", "abc", "target"))

		// --- When ---
		have, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.ErrorIs(t, ErrInvTimeout, err)
		assert.ErrorContain(t, "abc", err)
		assert.Nil(t, have)
	})

	t.Run("short help option", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("-h", "target"))

		// --- When ---
		have, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have.showHelp)
		assert.Equal(t, []string{"target"}, have.rng.Args())
		assert.Nil(t, have.optionTgt)
	})

	t.Run("help long", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("--help", "target"))

		// --- When ---
		have, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have.showHelp)
		assert.Equal(t, []string{"target"}, have.rng.Args())
		assert.Nil(t, have.optionTgt)
	})

	t.Run("both short and long help options", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		argsOF := WithMakefileArgs("-h", "--help", "target")

		// --- When ---
		have, err := NewMakefile(tgs, argsOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have.showHelp)
		assert.Equal(t, []string{"target"}, have.rng.Args())
		assert.Nil(t, have.optionTgt)
	})

	t.Run("version option", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t).WetStdout()
		rngOF := WithMakefileRing(tst.Ring("--version"))
		verOF := WithMakefileVersion("1.2.3")

		// --- When ---
		have, err := NewMakefile(tgs, rngOF, verOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "1.2.3", have.version)
		assert.Equal(t, []string{}, have.rng.Args())

		assert.NoError(t, have.Execute(t.Context()))
		assert.Equal(t, "1.2.3\n", tst.Stdout())
	})

	t.Run("list option", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{
			{Name: "first", Synopsis: "doc for first", Default: false},
			{Name: "second", Synopsis: "doc for second", Default: true},
		}
		tst := ringtest.New(t).WetStdout()
		rngOF := WithMakefileRing(tst.Ring("--list"))

		// --- When ---
		have, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{}, have.rng.Args())

		assert.NoError(t, have.Execute(t.Context()))
		want := "" +
			"first      doc for first\n" +
			"second*    doc for second\n"
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("error - unknown option", func(t *testing.T) {
		// --- Given ---
		tgs := make([]*Target, 0)
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("--unknown", "target"))

		// --- When ---
		have, err := NewMakefile(tgs, rngOF)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		assert.Nil(t, have)
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
		must.Nil(os.Chdir(dir))
		must.Nil(os.Remove(dir))

		// --- When ---
		have, err := NewMakefile(tgs, rngOF, argsOF)

		// --- Then ---
		want := "working directory: getwd: no such file or directory"
		assert.ErrorEqual(t, want, err)
		assert.Nil(t, have)
	})
}

func Test_Makefile_Execute(t *testing.T) {
	t.Run("show target help", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtA()}
		tst := ringtest.New(t).WetStderr()
		rngOF := WithMakefileRing(tst.Ring("-h", "tgt-a"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(t.Context())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "tgt-a\tdoc tgt-a\n", tst.Stderr())
	})

	t.Run("error - help for unknown target", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtA()}
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("-h", "unknown"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(t.Context())

		// --- Then ---
		assert.ErrorIs(t, ErrUnkTarget, err)
	})

	t.Run("show the version", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtA()}
		tst := ringtest.New(t).WetStdout()
		rngOF := WithMakefileRing(tst.Ring("--version"))
		verOF := WithMakefileVersion("1.2.3")
		cmf := must.Value(NewMakefile(tgs, rngOF, verOF))

		// --- When ---
		err := cmf.Execute(t.Context())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "1.2.3\n", tst.Stdout())
	})

	t.Run("execute target", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtPrintArgs()}
		tst := ringtest.New(t).WetStdout()
		rngOF := WithMakefileRing(tst.Ring("tgt-print-args", "arg0"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(t.Context())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "[arg0]", tst.Stdout())
	})

	t.Run("error - unknown target", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtB()}
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("unknown"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(t.Context())

		// --- Then ---
		assert.ErrorIs(t, ErrUnkTarget, err)
	})

	t.Run("error - target returning", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtError()}
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("tgt-error"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(t.Context())

		// --- Then ---
		assert.ErrorEqual(t, "always error", err)
	})

	t.Run("error - target execution timeout applied", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtLong()}
		tst := ringtest.New(t)
		rngOF := WithMakefileRing(tst.Ring("--timeout", "50ms", "tgt-long"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(t.Context())

		// --- Then ---
		assert.ErrorIs(t, context.DeadlineExceeded, err)
		assert.Equal(t, 125, ExitCode(err))
	})

	t.Run("error - target deadline without timeout", func(t *testing.T) {
		// --- Given ---
		tgt := &Target{
			Name: "tgt-deadline",
			Run: func(_ context.Context, _ *ring.Ring) error {
				return fmt.Errorf("fetch: %w", context.DeadlineExceeded)
			},
		}
		rngOF := WithMakefileRing(ringtest.New(t).Ring("tgt-deadline"))
		cmf := must.Value(NewMakefile([]*Target{tgt}, rngOF))

		// --- When ---
		err := cmf.Execute(t.Context())

		// --- Then ---
		assert.ErrorIs(t, context.DeadlineExceeded, err)
		assert.Equal(t, ExitCodeErr, ExitCode(err))
	})

	t.Run("target execution time not limited", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtLong()}
		tst := ringtest.New(t).WetStdout()
		rngOF := WithMakefileRing(tst.Ring("tgt-long"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(t.Context())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "done after 100ms", tst.Stdout())
	})

	t.Run("target knows its name", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtPrintName()}
		tst := ringtest.New(t).WetStdout()
		rngOF := WithMakefileRing(tst.Ring("tgt-print-name"))
		cmf := must.Value(NewMakefile(tgs, rngOF))

		// --- When ---
		err := cmf.Execute(t.Context())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "tgt-print-name", tst.Stdout())
	})
}

func Test_fTgtVersion(t *testing.T) {
	t.Run("no empty version", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()

		// --- When ---
		err := fTgtVersion("1.2.3")(t.Context(), tst.Ring())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "1.2.3\n", tst.Stdout())
	})

	t.Run("empty version", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()

		// --- When ---
		err := fTgtVersion("")(t.Context(), tst.Ring())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "\n", tst.Stdout())
	})
}

func Test_fTgtList(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtCore(), TgtA(), TgtB()}
		tst := ringtest.New(t).WetStdout()

		// --- When ---
		err := fTgtList(tgs)(t.Context(), tst.Ring())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			":tgt-core    syn tgt-core\n" +
			"\n" +
			"tgt-a        syn tgt-a\n" +
			"tgt-b*       syn tgt-b\n"
		assert.Equal(t, want, tst.Stdout())
	})
}
