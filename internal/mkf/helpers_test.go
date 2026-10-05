// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mkf

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring"
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
		{"timeout", fmt.Errorf("%w: x", errTimeout), 125},
		{"deadline exceeded", context.DeadlineExceeded, 1},
		{"pick target", ErrPickTarget, 126},
		{"unknown target", ErrUnkTarget, 127},
		{"interrupted", interruptedError{sig: syscall.SIGTERM}, 143},
		{"generic", errors.New("test error"), 1},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := ExitCode(tc.in)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

// reraiseEnv makes a Test_Reraise helper process re-raise SIGTERM.
const reraiseEnv = "GOMAKE_TEST_RERAISE"

func Test_Reraise(t *testing.T) {
	if os.Getenv(reraiseEnv) != "" {
		signal.Notify(make(chan os.Signal, 1), syscall.SIGTERM)
		err := interruptedError{sig: syscall.SIGTERM}
		Reraise(err)
		os.Exit(ExitCode(err))
	}

	t.Run("interrupted", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("signals cannot be sent to a process on windows")
		}

		// --- Given ---
		args := []string{"-test.run=^Test_Reraise$"}
		cmd := exec.CommandContext(t.Context(), os.Args[0], args...)
		cmd.Env = append(os.Environ(), reraiseEnv+"=1")

		// --- When ---
		err := cmd.Run()

		// --- Then ---
		assert.Error(t, err)
		sts := cmd.ProcessState.Sys().(syscall.WaitStatus)
		assert.True(t, sts.Signaled())
		assert.Equal(t, syscall.SIGTERM, sts.Signal())
	})

	t.Run("not interrupted", func(t *testing.T) {
		// --- When ---
		Reraise(errors.New("test error"))
	})
}

func Test_RecoverError(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		// --- Given ---
		in := errors.New("test error")

		// --- When ---
		have := RecoverError(in)

		// --- Then ---
		assert.Same(t, in, have)
	})

	t.Run("string", func(t *testing.T) {
		// --- When ---
		have := RecoverError("string have")

		// --- Then ---
		assert.ErrorEqual(t, "string have", have)
	})

	t.Run("other", func(t *testing.T) {
		// --- When ---
		have := RecoverError(123)

		// --- Then ---
		assert.ErrorEqual(t, "123", have)
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
		assert.ErrorContain(t, "unknown target: unknown", err)
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
		have, err := pickTarget(tgs, &args)

		// --- Then ---
		assert.Nil(t, have)
		assert.ErrorIs(t, ErrPickTarget, err)
	})

	t.Run("empty args with default", func(t *testing.T) {
		// --- Given ---
		wantTgt := TgtB()
		tgs := []*Target{TgtA(), wantTgt, TgtC()}
		var args []string

		// --- When ---
		have, err := pickTarget(tgs, &args)

		// --- Then ---
		assert.NoError(t, err)
		assert.Same(t, wantTgt, have)
		assert.Empty(t, args)
	})

	t.Run("pick target selected in args", func(t *testing.T) {
		// --- Given ---
		wantTgt := TgtC()
		tgs := []*Target{TgtA(), TgtB(), wantTgt}
		args := []string{"tgt-c", "arg0", "arg1"}

		// --- When ---
		have, err := pickTarget(tgs, &args)

		// --- Then ---
		assert.NoError(t, err)
		assert.Same(t, wantTgt, have)
		assert.Equal(t, []string{"arg0", "arg1"}, args)
	})

	t.Run("error - unknown target", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtA(), TgtB(), TgtC()}
		args := []string{"unknown"}

		// --- When ---
		have, err := pickTarget(tgs, &args)

		// --- Then ---
		assert.ErrorIs(t, ErrUnkTarget, err)
		assert.Nil(t, have)
		assert.Equal(t, []string{"unknown"}, args)
	})
}

func Test_runTarget(t *testing.T) {
	// Cooperative targets that return ctx.Err() after cancel must still
	// surface as interruptedError (128+n), not a plain context.Canceled.
	t.Run("error - signal with cooperative cancel", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		fn := func(c context.Context, _ *ring.Ring) error {
			<-c.Done()
			return c.Err()
		}
		wd := must.Value(os.Getwd())

		// --- When ---
		exited := make(chan error, 1)
		go func() {
			exited <- runTarget(t.Context(), sig, fn, wd, tst.Ring())
		}()
		// Let the target block on ctx.Done, then signal.
		time.Sleep(20 * time.Millisecond)
		sig <- syscall.SIGTERM
		err := <-exited

		// --- Then ---
		var ier interruptedError
		assert.True(t, errors.As(err, &ier))
		assert.Equal(t, syscall.SIGTERM, ier.sig)
	})

	t.Run("execute target", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())

		// --- When ---
		err := runTarget(t.Context(), sig, TgtA().Run, wd, tst.Ring())

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

	t.Run("error - deadline before return", func(t *testing.T) {
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

	t.Run("error - target panicking", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())

		// --- When ---
		err := runTarget(t.Context(), sig, TgtPanicString().Run, wd, tst.Ring())

		// --- Then ---
		assert.ErrorEqual(t, "target panicked with: panic string", err)
		assert.Empty(t, tst.Stdout())
		assert.Empty(t, tst.Stderr())
	})

	t.Run("error - canceled with signal", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())

		// Wait for "started" and "exited" to appear on stdout.
		started := func() bool {
			return strings.HasPrefix(tst.Stdout(), "started ")
		}
		printed := func() bool {
			return strings.HasSuffix(tst.Stdout(), " exited")
		}

		// --- When ---
		exited := make(chan struct{})
		var err error
		go func() {
			err = runTarget(t.Context(), sig, TgtWaiting().Run, wd, tst.Ring())
			close(exited)
		}()

		must.Nil(check.Wait(
			"1s",
			started,
			check.WithWaitThrottle(10*time.Millisecond),
		))

		sig <- syscall.SIGINT

		must.Nil(check.Wait(
			"1s",
			printed,
			check.WithWaitThrottle(10*time.Millisecond),
		))

		<-exited // Goroutine exited.

		// --- Then ---
		var ier interruptedError
		assert.ErrorAs(t, &ier, err)
		assert.Equal(t, syscall.SIGINT, ier.sig)
		assert.Equal(t, "started context canceled exited", tst.Stdout())
		assert.Equal(t, "", tst.Stderr())
	})

	t.Run("error - canceled with SIGTERM", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())

		started := func() bool {
			return strings.HasPrefix(tst.Stdout(), "started ")
		}

		// --- When ---
		exited := make(chan struct{})
		var err error
		go func() {
			err = runTarget(t.Context(), sig, TgtWaiting().Run, wd, tst.Ring())
			close(exited)
		}()

		must.Nil(check.Wait(
			"1s",
			started,
			check.WithWaitThrottle(10*time.Millisecond),
		))

		sig <- syscall.SIGTERM

		<-exited

		// --- Then ---
		var ier interruptedError
		assert.ErrorAs(t, &ier, err)
		assert.Equal(t, syscall.SIGTERM, ier.sig)
	})

	t.Run("error - target calls Goexit", func(t *testing.T) {
		// --- Given ---
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())

		fn := func(_ context.Context, _ *ring.Ring) error {
			runtime.Goexit()
			return nil
		}

		// --- When ---
		err := runTarget(t.Context(), sig, fn, wd, ringtest.New(t).Ring())

		// --- Then ---
		assert.ErrorEqual(t, "target exited without returning", err)
	})

	t.Run("target cleanup runs after signal", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())

		started := make(chan struct{})
		fn := func(ctx context.Context, rng *ring.Ring) error {
			close(started)
			<-ctx.Done()
			time.Sleep(50 * time.Millisecond) // Cleanup.
			_, _ = fmt.Fprint(rng.Stdout(), "cleaned")
			return ctx.Err()
		}
		go func() { <-started; sig <- syscall.SIGINT }()

		// --- When ---
		err := runTarget(t.Context(), sig, fn, wd, tst.Ring())

		// --- Then ---
		var ier interruptedError
		assert.ErrorAs(t, &ier, err)
		assert.Equal(t, "cleaned", tst.Stdout())
	})

	t.Run("second signal skips cleanup wait", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())

		release := make(chan struct{})
		defer close(release)
		started := make(chan struct{})
		fn := func(_ context.Context, _ *ring.Ring) error {
			close(started)
			<-release // Ignores the canceled context.
			return nil
		}
		go func() {
			<-started
			sig <- syscall.SIGINT
			sig <- syscall.SIGINT
		}()

		// --- When ---
		err := runTarget(t.Context(), sig, fn, wd, tst.Ring())

		// --- Then ---
		var ier interruptedError
		assert.ErrorAs(t, &ier, err)
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
		must.Nil(check.Wait(
			"1s",
			restored,
			check.WithWaitThrottle(10*time.Millisecond),
		))

		assert.Equal(t, wd, must.Value(os.Getwd()))
		// The directory is restored only after the target's deferred " exited"
		// print, so by now stdout is complete.
		assert.Equal(t, "started context canceled exited", tst.Stdout())
	})

	t.Run("cwd restored after signal", func(t *testing.T) {
		// --- Given ---
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
		exited := make(chan struct{})
		var err error
		go func() {
			err = runTarget(t.Context(), sig, TgtWaiting().Run, dir, tst.Ring())
			close(exited)
		}()

		must.Nil(check.Wait(
			"1s",
			started,
			check.WithWaitThrottle(10*time.Millisecond),
		))

		sig <- syscall.SIGINT

		<-exited // RunTarget returned.

		// --- Then ---
		var ier interruptedError
		assert.ErrorAs(t, &ier, err)
		// The target goroutine outlives RunTarget on the signal path too; its
		// deferred os.Chdir restores the working directory only after the send
		// completes. Polling for it proves the goroutine did not leak.
		restored := func() bool { return must.Value(os.Getwd()) == wd }
		must.Nil(check.Wait(
			"1s",
			restored,
			check.WithWaitThrottle(10*time.Millisecond),
		))

		assert.Equal(t, wd, must.Value(os.Getwd()))
	})

	t.Run("change working directory", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout().WetStderr()
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())
		cwd := pathkit.AbsPath(t, "testdata/dir")

		// --- When ---
		err := runTarget(t.Context(), sig, TgtListFiles().Run, cwd, tst.Ring())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "fil0.txt\nfil1.txt\n", tst.Stdout())
		want := fmt.Sprintf("Listing files in: %s\n", cwd)
		assert.Equal(t, want, tst.Stderr())
		assert.Equal(t, wd, must.Value(os.Getwd()))
	})

	t.Run("error - change working directory", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())
		cwd := pathkit.AbsPath(t, "testdata/not-existing")

		// --- When ---
		err := runTarget(t.Context(), sig, TgtListFiles().Run, cwd, tst.Ring())

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.Equal(t, wd, must.Value(os.Getwd()))
		assert.Empty(t, tst.Stdout())
		assert.Empty(t, tst.Stderr())
	})

	t.Run("error - target returning", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())

		// --- When ---
		err := runTarget(t.Context(), sig, TgtError().Run, wd, tst.Ring())

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
		tst := ringtest.New(t)
		sig := make(chan os.Signal, 1)
		defer signal.Stop(sig)
		wd := must.Value(os.Getwd())
		t.Cleanup(func() { _ = os.Chdir(wd) })

		cwd := t.TempDir()
		must.Nil(os.Chdir(cwd))
		must.Nil(os.Remove(cwd))

		// --- When ---
		err := runTarget(t.Context(), sig, TgtCore().Run, cwd, tst.Ring())

		// --- Then ---
		var e *os.SyscallError
		assert.ErrorAs(t, &e, err)
		assert.Equal(t, "getwd", e.Syscall)
		assert.Equal(t, syscall.ENOENT, e.Err)
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

		// --- When ---
		err := runTarget(ctx, sig, TgtChdir().Run, wd, tst.Ring(cwd))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, wd, must.Value(os.Getwd()))
		assert.Equal(t, fmt.Sprintf("changed to %s", cwd), tst.Stdout())
		assert.Empty(t, tst.Stderr())
	})
}

func Test_timedOut(t *testing.T) {
	t.Run("timeout deadline", func(t *testing.T) {
		// --- Given ---
		ctx, cxl := context.WithTimeoutCause(t.Context(), 0, errTimeout)
		defer cxl()

		// --- When ---
		err := timedOut(ctx, context.DeadlineExceeded)

		// --- Then ---
		assert.ErrorIs(t, errTimeout, err)
		assert.ErrorIs(t, context.DeadlineExceeded, err)
	})

	t.Run("other deadline", func(t *testing.T) {
		// --- Given ---
		ctx, cxl := context.WithTimeout(t.Context(), 0)
		defer cxl()

		// --- When ---
		err := timedOut(ctx, context.DeadlineExceeded)

		// --- Then ---
		assert.ErrorIsNot(t, errTimeout, err)
		assert.ErrorIs(t, context.DeadlineExceeded, err)
	})

	t.Run("not a deadline", func(t *testing.T) {
		// --- Given ---
		ctx, cxl := context.WithTimeoutCause(t.Context(), 0, errTimeout)
		defer cxl()
		want := errors.New("test error")

		// --- When ---
		err := timedOut(ctx, want)

		// --- Then ---
		assert.Same(t, want, err)
	})

	t.Run("nil", func(t *testing.T) {
		// --- When ---
		err := timedOut(t.Context(), nil)

		// --- Then ---
		assert.NoError(t, err)
	})
}

func Test_awaitDone(t *testing.T) {
	t.Run("done closed", func(t *testing.T) {
		// --- Given ---
		done := make(chan error, 1)
		done <- nil
		close(done)

		// --- When ---
		awaitDone(done, make(chan os.Signal), time.Hour)

		// --- Then ---
		_, open := <-done
		assert.False(t, open)
	})

	t.Run("second signal", func(t *testing.T) {
		// --- Given ---
		sig := make(chan os.Signal, 1)
		sig <- syscall.SIGINT

		// --- When ---
		awaitDone(make(chan error), sig, time.Hour)

		// --- Then ---
		assert.Len(t, 0, sig)
	})

	t.Run("grace elapsed", func(t *testing.T) {
		// --- Given ---
		start := time.Now()

		// --- When ---
		awaitDone(make(chan error), make(chan os.Signal), time.Millisecond)

		// --- Then ---
		assert.True(t, time.Since(start) < time.Hour)
	})
}

func Test_recvDone(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// --- When ---
		hOk, err := recvDone(make(chan error, 1))

		// --- Then ---
		assert.False(t, hOk)
		assert.NoError(t, err)
	})

	t.Run("ready result", func(t *testing.T) {
		// --- Given ---
		done := make(chan error, 2)
		done <- nil
		close(done)

		// --- When ---
		hOk, err := recvDone(done)

		// --- Then ---
		assert.True(t, hOk)
		assert.NoError(t, err)
	})

	t.Run("ready error", func(t *testing.T) {
		// --- Given ---
		done := make(chan error, 2)
		want := errors.New("target failed")
		done <- want
		close(done)

		// --- When ---
		hOk, have := recvDone(done)

		// --- Then ---
		assert.True(t, hOk)
		assert.ErrorIs(t, want, have)
	})

	t.Run("error - closed without result", func(t *testing.T) {
		// --- Given ---
		done := make(chan error)
		close(done)

		// --- When ---
		hOk, err := recvDone(done)

		// --- Then ---
		assert.True(t, hOk)
		assert.ErrorIs(t, errNoResult, err)
	})
}

func Test_drainDone(t *testing.T) {
	// --- Given ---
	done := make(chan error, 1)
	done <- errors.New("extra")
	close(done)
	want := errors.New("first")

	// --- When ---
	have := drainDone(done, want)

	// --- Then ---
	assert.Same(t, want, have)
	_, open := <-done
	assert.False(t, open)
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
		assert.Equal(t, "no targets\n", have)
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

	t.Run("does not reorder input", func(t *testing.T) {
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

	t.Run("all hidden", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{{Name: "x", Hidden: true}}

		// --- When ---
		have := HelpTargets(tgs, 0)

		// --- Then ---
		assert.Equal(t, "", have)
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

func Test_helpCommand(t *testing.T) {
	t.Run("usage", func(t *testing.T) {
		// --- Given ---
		tgs := []*Target{TgtCore(), TgtA(), TgtB()}
		argsOF := WithMakefileArgs("--list")
		cmf := must.Value(NewMakefile(tgs, argsOF))

		// --- When ---
		have := helpCommand(MakefileBin, cmf.fs, cmf.targets)

		// --- Then ---
		want := "" +
			"makefile [options] [target] [args]\n" +
			"\n" +
			"The makefile is a make-like target runner.\n" +
			"\n" +
			"options:\n" +
			"  -h, --help       show this help or target documentation\n" +
			"      --list       show available targets\n" +
			"      --timeout    target execution timeout (default: 0)\n" +
			"      --version    print version\n" +
			"      --wd         working directory for a target\n" +
			"\n" +
			"targets:\n" +
			"  :tgt-core    syn tgt-core\n" +
			"\n" +
			"  tgt-a        syn tgt-a\n" +
			"  tgt-b*       syn tgt-b\n"
		assert.Equal(t, want, have)
	})

	t.Run("no targets", func(t *testing.T) {
		// --- Given ---
		argsOF := WithMakefileArgs("--list")
		cmf := must.Value(NewMakefile(nil, argsOF))

		// --- When ---
		have := helpCommand(MakefileBin, cmf.fs, cmf.targets)

		// --- Then ---
		want := "" +
			"makefile [options] [target] [args]\n" +
			"\n" +
			"The makefile is a make-like target runner.\n" +
			"\n" +
			"options:\n" +
			"  -h, --help       show this help or target documentation\n" +
			"      --list       show available targets\n" +
			"      --timeout    target execution timeout (default: 0)\n" +
			"      --version    print version\n" +
			"      --wd         working directory for a target\n" +
			"\n" +
			"targets:\n" +
			"  no targets\n"
		assert.Equal(t, want, have)
	})
}

func Test_Target_help(t *testing.T) {
	t.Run("target with doc string", func(t *testing.T) {
		// --- Given ---
		tgt := TgtA()

		// --- When ---
		have := tgt.help()

		// --- Then ---
		assert.Equal(t, "tgt-a\tdoc tgt-a\n", have)
	})

	t.Run("target without doc string", func(t *testing.T) {
		// --- Given ---
		tgt := TgtA()
		tgt.Doc = ""

		// --- When ---
		have := tgt.help()

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
