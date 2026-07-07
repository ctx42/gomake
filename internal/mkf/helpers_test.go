// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mkf

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/check"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/pathkit"
)

func Test_ExitCode_tabular(t *testing.T) {
	tt := []struct {
		testN string

		in   error
		want int
	}{
		{"nil err", nil, 0},
		{"deadline exceeded", context.DeadlineExceeded, 125},
		{"pick target", ErrPickTarget, 126},
		{"unknown target", ErrUnkTarget, 127},
		{"interrupted", interruptedError(200), 200},
		{"generic", errors.New("test error"), 1},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			assert.Equal(t, tc.want, ExitCode(tc.in))
		})
	}
}

func Test_RecoverError(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		// --- Given ---
		in := errors.New("test error")

		// --- When ---
		err := RecoverError(in)

		// --- Then ---
		assert.Same(t, in, err)
	})

	t.Run("string", func(t *testing.T) {
		// --- When ---
		err := RecoverError("string err")

		// --- Then ---
		assert.ErrorEqual(t, "string err", err)
	})

	t.Run("other", func(t *testing.T) {
		// --- When ---
		err := RecoverError(123)

		// --- Then ---
		assert.ErrorEqual(t, "123", err)
	})
}

func Test_FindTarget(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		// --- Given ---
		want := TgtB()
		tgs := []*Target{TgtA(), want, TgtC()}

		// --- When ---
		have, err := FindTarget("tgt-b", tgs)

		// --- Then ---
		assert.NoError(t, err)
		assert.Same(t, want, have)
	})

	t.Run("error - not found", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtA(), TgtB(), TgtC()}

		// --- When ---
		have, err := FindTarget("unknown", tgs)

		// --- Then ---
		assert.ErrorIs(t, ErrUnkTarget, err)
		assert.Nil(t, have)
	})
}

func Test_defaultTarget(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		// --- Given ---
		want := TgtB()
		tgs := []*Target{TgtA(), want, TgtC()}

		// --- When ---
		have, err := defaultTarget(tgs)

		// --- Then ---
		assert.NoError(t, err)
		assert.Same(t, want, have)
	})

	t.Run("error - not found", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtA(), TgtC()}

		// --- When ---
		have, err := defaultTarget(tgs)

		// --- Then ---
		assert.ErrorIs(t, ErrPickTarget, err)
		assert.Nil(t, have)
	})
}

func Test_pickTarget(t *testing.T) {
	t.Run("error - empty args no default", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtA(), TgtC()}
		var args []string

		// --- When ---
		haveTgt, err := pickTarget(tgs, &args)

		// --- Then ---
		assert.Nil(t, haveTgt)
		assert.ErrorIs(t, ErrPickTarget, err)
	})

	t.Run("empty args with default", func(t *testing.T) {
		// --- Given ---
		wantTgt := TgtB()
		tgs := []*Target{TgtA(), wantTgt, TgtC()}
		var args []string

		// --- When ---
		haveTgt, err := pickTarget(tgs, &args)

		// --- Then ---
		assert.NoError(t, err)
		assert.Same(t, wantTgt, haveTgt)
		assert.Empty(t, args)
	})

	t.Run("pick target selected in args", func(t *testing.T) {
		// --- Given ---
		wantTgt := TgtC()
		tgs := []*Target{TgtA(), TgtB(), wantTgt}
		args := []string{"tgt-c", "arg0", "arg1"}

		// --- When ---
		haveTgt, err := pickTarget(tgs, &args)

		// --- Then ---
		assert.NoError(t, err)
		assert.Same(t, wantTgt, haveTgt)
		assert.Equal(t, []string{"arg0", "arg1"}, args)
	})

	t.Run("error - unknown target", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtA(), TgtB(), TgtC()}
		args := []string{"unknown"}

		// --- When ---
		haveTgt, err := pickTarget(tgs, &args)

		// --- Then ---
		assert.ErrorIs(t, ErrUnkTarget, err)
		assert.Nil(t, haveTgt)
		assert.Equal(t, []string{"unknown"}, args)
	})
}

func Test_runTarget(t *testing.T) {
	t.Run("execute target", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())

		// --- When ---
		err := runTarget(ctx, sig, TgtA().Run, wd, tst.Ring())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "a", tst.Stdout())
		assert.Empty(t, tst.Stderr())
	})

	t.Run("pass arguments to target", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())
		args := []string{"arg0", "arg1"}

		// --- When ---
		err := runTarget(ctx, sig, TgtPrintArgs().Run, wd, tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "[arg0 arg1]", tst.Stdout())
		assert.Empty(t, tst.Stderr())
	})

	t.Run("context timeout shorter than execution time", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t)
		ctx, cxl := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cxl()

		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())

		// --- When ---
		err := runTarget(ctx, sig, TgtLong().Run, wd, tst.Ring())

		// --- Then ---
		assert.ErrorIs(t, context.DeadlineExceeded, err)
		assert.Empty(t, tst.Stdout())
		assert.Empty(t, tst.Stderr())
	})

	t.Run("context timeout longer than execution time", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()
		ctx, cxl := context.WithTimeout(ctx, 150*time.Millisecond)
		defer cxl()

		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())

		// --- When ---
		err := runTarget(ctx, sig, TgtLong().Run, wd, tst.Ring())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "done after 100ms", tst.Stdout())
		assert.Empty(t, tst.Stderr())
	})

	t.Run("target panicking", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t)
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())

		// --- When ---
		err := runTarget(ctx, sig, TgtPanicString().Run, wd, tst.Ring())

		// --- Then ---
		assert.ErrorEqual(t, "target panicked with: panic string", err)
		assert.Empty(t, tst.Stdout())
		assert.Empty(t, tst.Stderr())
	})

	t.Run("target canceled with signal", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())

		// Wait for "started" and "exited" to appear on stdout.
		started := func() bool {
			return strings.HasPrefix(tst.Stdout(), "started ")
		}
		exited := func() bool {
			return strings.HasSuffix(tst.Stdout(), " exited")
		}

		// --- When ---
		done := make(chan struct{})
		var err error
		go func() {
			err = runTarget(ctx, sig, TgtWaiting().Run, wd, tst.Ring())
			close(done)
		}()

		// --- Then ---
		check.Wait("1s", started, check.WithWaitThrottle(10*time.Millisecond))
		sig <- syscall.SIGINT
		check.Wait("1s", exited, check.WithWaitThrottle(10*time.Millisecond))
		<-done // Goroutine exited.

		var e interruptedError
		assert.ErrorAs(t, &e, err)
		assert.Equal(t, 128+2, e.Signal())
		assert.Equal(t, "started context canceled exited", tst.Stdout())
		assert.Equal(t, "", tst.Stderr())
	})

	t.Run("cwd restored after timeout", func(t *testing.T) {
		// --- Given ---
		ctx, cxl := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cxl()
		tst := ringtest.New(t).WetStdout()
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())
		t.Cleanup(func() { _ = os.Chdir(wd) })
		dir := pathkit.AbsPath(t, "testdata/dir")

		// --- When ---
		err := runTarget(ctx, sig, TgtWaiting().Run, dir, tst.Ring())

		// --- Then ---
		assert.ErrorIs(t, context.DeadlineExceeded, err)
		// The target goroutine outlives RunTarget; once it observes the
		// canceled context it terminates and its deferred os.Chdir restores
		// the working directory. Polling for the restored directory proves the
		// goroutine's send did not block forever (no leak) and that Execute's
		// cwd contract holds on the timeout path.
		restored := func() bool { return must.Value(os.Getwd()) == wd }
		check.Wait("1s", restored, check.WithWaitThrottle(10*time.Millisecond))
		assert.Equal(t, wd, must.Value(os.Getwd()))
		// The directory is restored only after the target's deferred " exited"
		// print, so by now stdout is complete.
		assert.Equal(t, "started context canceled exited", tst.Stdout())
	})

	t.Run("cwd restored after signal", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())
		t.Cleanup(func() { _ = os.Chdir(wd) })
		dir := pathkit.AbsPath(t, "testdata/dir")

		started := func() bool {
			return strings.HasPrefix(tst.Stdout(), "started ")
		}

		// --- When ---
		done := make(chan struct{})
		var err error
		go func() {
			err = runTarget(ctx, sig, TgtWaiting().Run, dir, tst.Ring())
			close(done)
		}()

		// --- Then ---
		check.Wait("1s", started, check.WithWaitThrottle(10*time.Millisecond))
		sig <- syscall.SIGINT
		<-done // RunTarget returned.

		var e interruptedError
		assert.ErrorAs(t, &e, err)
		// The target goroutine outlives RunTarget on the signal path too; its
		// deferred os.Chdir restores the working directory only after the send
		// completes. Polling for it proves the goroutine did not leak.
		restored := func() bool { return must.Value(os.Getwd()) == wd }
		check.Wait("1s", restored, check.WithWaitThrottle(10*time.Millisecond))
		assert.Equal(t, wd, must.Value(os.Getwd()))
	})

	t.Run("change working directory", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout().WetStderr()
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())
		cwd := pathkit.AbsPath(t, "testdata/dir")

		// --- When ---
		err := runTarget(ctx, sig, TgtListFiles().Run, cwd, tst.Ring())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "fil0.txt\nfil1.txt\n", tst.Stdout())
		want := fmt.Sprintf("Listing files in: %s\n", cwd)
		assert.Equal(t, want, tst.Stderr())
		assert.Equal(t, wd, must.Value(os.Getwd()))
	})

	t.Run("error - change working directory", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t)
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())
		cwd := pathkit.AbsPath(t, "testdata/not-existing")

		// --- When ---
		err := runTarget(ctx, sig, TgtListFiles().Run, cwd, tst.Ring())

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.Equal(t, wd, must.Value(os.Getwd()))
		assert.Empty(t, tst.Stdout())
		assert.Empty(t, tst.Stderr())
	})

	t.Run("error - target returning", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t)
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())

		// --- When ---
		err := runTarget(ctx, sig, TgtError().Run, wd, tst.Ring())

		// --- Then ---
		assert.ErrorEqual(t, "always error", err)
		assert.Empty(t, tst.Stdout())
		assert.Empty(t, tst.Stderr())
	})

	t.Run("error - working directory does not exist", func(t *testing.T) {
		if runtime.GOOS == "darwin" {
			t.Skip("skipping test on darwin")
		}

		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t)
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())
		t.Cleanup(func() { _ = os.Chdir(wd) })

		cwd := t.TempDir()
		assert.NoError(t, os.Chdir(cwd))
		assert.NoError(t, os.Remove(cwd))

		// --- When ---
		err := runTarget(ctx, sig, TgtCore().Run, cwd, tst.Ring())

		// --- Then ---
		var e *os.SyscallError
		assert.ErrorAs(t, &e, err)
		assert.Equal(t, e.Syscall, "getwd")
		assert.Equal(t, e.Err, syscall.ENOENT)
		assert.Empty(t, tst.Stdout())
		assert.Empty(t, tst.Stderr())
	})

	t.Run("cwd restored after target changed it", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())
		cwd := pathkit.AbsPath(t, "testdata")
		args := []string{cwd}

		// --- When ---
		err := runTarget(ctx, sig, TgtChdir().Run, wd, tst.Ring(args...))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, wd, must.Value(os.Getwd()))
		assert.Equal(t, fmt.Sprintf("changed to %s", cwd), tst.Stdout())
		assert.Empty(t, tst.Stderr())
	})
}

func Test_HelpTargets(t *testing.T) {
	t.Run("one target", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtA()}

		// --- When ---
		have := HelpTargets(tgs, 0)

		// --- Then ---
		want := "" +
			"tgt-a    syn tgt-a\n"
		assert.Equal(t, want, have)
	})

	t.Run("more than one target", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtA(), TgtB(), TgtC()}

		// --- When ---
		have := HelpTargets(tgs, 0)

		// --- Then ---
		want := "" +
			"tgt-a     syn tgt-a\n" +
			"tgt-b*    syn tgt-b\n" +
			"tgt-c     syn tgt-c\n"
		assert.Equal(t, want, have)
	})

	t.Run("no targets", func(t *testing.T) {
		// --- Given ---
		var tgs []*Target

		// --- When ---
		have := HelpTargets(tgs, 0)

		// --- Then ---
		want := "no targets\n"
		assert.Equal(t, want, have)
	})

	t.Run("core and user targets separated", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtCore(), TgtA(), TgtB(), TgtC()}

		// --- When ---
		have := HelpTargets(tgs, 0)

		// --- Then ---
		want := "" +
			":tgt-core    syn tgt-core\n" +
			"\n" +
			"tgt-a        syn tgt-a\n" +
			"tgt-b*       syn tgt-b\n" +
			"tgt-c        syn tgt-c\n"
		assert.Equal(t, want, have)
	})

	t.Run("hidden target", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtCore(), TgtHidden(), TgtA(), TgtB()}

		// --- When ---
		have := HelpTargets(tgs, 0)

		// --- Then ---
		want := "" +
			":tgt-core    syn tgt-core\n" +
			"\n" +
			"tgt-a        syn tgt-a\n" +
			"tgt-b*       syn tgt-b\n"
		assert.Equal(t, want, have)
	})

	t.Run("with padding", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtCore(), TgtA(), TgtB(), TgtC()}

		// --- When ---
		have := HelpTargets(tgs, 2)

		// --- Then ---
		want := "" +
			"  :tgt-core    syn tgt-core\n" +
			"\n" +
			"  tgt-a        syn tgt-a\n" +
			"  tgt-b*       syn tgt-b\n" +
			"  tgt-c        syn tgt-c\n"
		assert.Equal(t, want, have)
	})

	t.Run("does not reorder the caller's slice", func(t *testing.T) {
		// --- Given ---
		a, b, c := TgtC(), TgtA(), TgtB()
		tgs := []*Target{a, b, c}

		// --- When ---
		_ = HelpTargets(tgs, 0)

		// --- Then ---
		assert.Same(t, a, tgs[0])
		assert.Same(t, b, tgs[1])
		assert.Same(t, c, tgs[2])
	})
}

func Test_HelpUsage(t *testing.T) {
	t.Run("print target help", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtCore(), TgtA(), TgtB()}
		argsOF := WithMakefileArgs("-h", "tgt-a")
		cmf := must.Value(NewMakefile(tgs, argsOF))

		// --- When ---
		have, err := HelpUsage(MakefileBin, cmf.rng.Args(), cmf.fs, tgs)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "tgt-a\tdoc tgt-a\n", have)
	})

	t.Run("print general help", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtCore(), TgtA(), TgtB()}
		argsOF := WithMakefileArgs("-h")
		cmf := must.Value(NewMakefile(tgs, argsOF))

		// --- When ---
		have, err := HelpUsage("my-bin", cmf.rng.Args(), cmf.fs, tgs)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "my-bin [options] [target] [args]", have)
	})

	t.Run("error - help for not existing target", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtCore(), TgtA(), TgtB()}
		argsOF := WithMakefileArgs("-h", "unknown")
		cmf := must.Value(NewMakefile(tgs, argsOF))

		// --- When ---
		have, err := HelpUsage(MakefileBin, cmf.rng.Args(), cmf.fs, tgs)

		// --- Then ---
		assert.ErrorIs(t, ErrUnkTarget, err)
		assert.Empty(t, have)
	})
}

func Test_HelpCommand(t *testing.T) {
	t.Run("usage", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtCore(), TgtA(), TgtB()}
		argsOF := WithMakefileArgs("--list")
		cmf := must.Value(NewMakefile(tgs, argsOF))

		// --- When ---
		have := helpCommand(MakefileBin, cmf.fs, cmf.targets)

		// --- Then ---
		want := `makefile [options] [target] [args]

The makefile is a make-like target runner.

options:
  -h, --help       show this help or target documentation
      --list       show available targets
      --timeout    target execution timeout (default: 0)
      --version    print version
      --wd         working directory for a target

targets:
  :tgt-core    syn tgt-core

  tgt-a        syn tgt-a
  tgt-b*       syn tgt-b
`
		assert.Equal(t, want, have)
	})

	t.Run("no targets", func(t *testing.T) {
		// --- Given ---
		argsOF := WithMakefileArgs("--list")
		cmf := must.Value(NewMakefile(nil, argsOF))

		// --- When ---
		have := helpCommand(MakefileBin, cmf.fs, cmf.targets)

		// --- Then ---
		want := `makefile [options] [target] [args]

The makefile is a make-like target runner.

options:
  -h, --help       show this help or target documentation
      --list       show available targets
      --timeout    target execution timeout (default: 0)
      --version    print version
      --wd         working directory for a target

targets:
  no targets
`
		assert.Equal(t, want, have)
	})
}

func Test_helpTarget(t *testing.T) {
	t.Run("target with doc string", func(t *testing.T) {
		// --- Given ---
		tgt := TgtA()

		// --- When ---
		have := helpTarget(tgt)

		// --- Then ---
		assert.Equal(t, "tgt-a\tdoc tgt-a\n", have)
	})

	t.Run("target without doc string", func(t *testing.T) {
		// --- Given ---
		tgt := TgtA()
		tgt.Doc = ""

		// --- When ---
		have := helpTarget(tgt)

		// --- Then ---
		assert.Equal(t, "tgt-a\t\n", have)
	})
}

func Test_Indent_tabular(t *testing.T) {
	tt := []struct {
		testN string

		tabs int
		in   string
		want string
	}{
		{"1", 1, "", ""},
		{"2", 1, "a", "\ta"},
		{"3", 1, "a\nb", "\ta\n\tb"},
		{"4", 1, "a\nb\n", "\ta\n\tb\n"},
		{"5", 1, "a\n\n", "\ta\n\n"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := Indent(tc.tabs, tc.in)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
