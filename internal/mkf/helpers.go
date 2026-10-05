// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mkf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xflag/pkg/xflag"
)

// --- CODE MARK ---

// Exit codes.
const (
	// ExitCodeOK is exit code used for success.
	ExitCodeOK = 0

	// ExitCodeErr is exit code for a general error.
	ExitCodeErr = 1

	// ExitCodeCompile is exit code used when compilation of the makefiles and
	// generated files failed. It sits below the 128+n signal range, so a
	// compile failure is never mistaken for a SIGHUP (129).
	ExitCodeCompile = 124

	// exitCodeDeadline is exit code used when target exceeded the --timeout
	// deadline.
	exitCodeDeadline = 125

	// ExitCodePickTarget is the exit code used with [ErrPickTarget] error.
	ExitCodePickTarget = 126

	// ExitCodeUnkTarget is exit code used with [ErrUnkTarget] error.
	ExitCodeUnkTarget = 127

	// exitCodeSignal is the base number to which signal number which stopped
	// the gomake is added (128+n).
	exitCodeSignal = 128
)

// Signal handling timings.
const (
	// signalGrace is how long a target interrupted by a signal has to finish
	// its cleanup before the makefile exits.
	signalGrace = 5 * time.Second

	// reraiseWait is how long [Reraise] waits for a re-raised signal to end
	// the process.
	reraiseWait = time.Second
)

// ExitCode returns an exit code associated with the given error. If the error
// is nil, it returns 0.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return ExitCodeOK
	case errors.Is(err, errTimeout):
		return exitCodeDeadline
	case errors.Is(err, ErrPickTarget):
		return ExitCodePickTarget
	case errors.Is(err, ErrUnkTarget):
		return ExitCodeUnkTarget
	default:
		if e, ok := errors.AsType[interruptedError](err); ok {
			return e.ExitCode()
		}
		return ExitCodeErr
	}
}

// Reraise makes the process die by the signal that interrupted the target
// run err reports, as it would without the makefile's handler, so a parent
// shell sees a signal death. It returns when err reports no signal or the
// signal cannot be re-raised; the caller then exits with [ExitCode].
func Reraise(err error) {
	ier, ok := errors.AsType[interruptedError](err)
	if !ok {
		return
	}
	signal.Reset(ier.sig)
	prc, err := os.FindProcess(os.Getpid())
	if err != nil {
		return
	}
	if err = prc.Signal(ier.sig); err != nil {
		return
	}
	// The default action ends the process asynchronously. A signal the
	// process inherited as ignored has no effect, so give up after a while.
	time.Sleep(reraiseWait)
}

// RecoverError takes value returned from recovery function and based on its
// type, wraps it in an error. If the type is not supported, it will return
// an error with a message returned by [fmt.Sprint].
func RecoverError(v any) error {
	switch e := v.(type) {
	case error:
		return e
	case string:
		return errors.New(e)
	default:
		return errors.New(fmt.Sprint(e))
	}
}

// FindTarget finds target by given name in the slice, returns [ErrUnkTarget]
// when there is no target by that name. The error text includes the name.
func FindTarget(name string, tgs []*Target) (*Target, error) {
	for _, tgt := range tgs {
		if tgt.Name == name {
			return tgt, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrUnkTarget, name)
}

// defaultTarget returns the default target or [ErrPickTarget] error when the
// default target is not defined.
func defaultTarget(tgs []*Target) (*Target, error) {
	for _, tgt := range tgs {
		if tgt.Default {
			return tgt, nil
		}
	}
	return nil, ErrPickTarget
}

// pickTarget finds a target to run based on arguments. If the first element in
// the args table is the name of an existing target, it will modify the args
// slice by removing the first element (target name).
func pickTarget(tgs []*Target, args *[]string) (*Target, error) {
	// Find the default target when no args.
	if len(*args) == 0 {
		return defaultTarget(tgs)
	}

	// Find the target by name.
	tgt, err := FindTarget((*args)[0], tgs)
	if err != nil {
		return nil, err
	}

	// Remove the target name from the argument slice.
	*args = (*args)[1:]
	return tgt, nil
}

// runTarget runs function in working directory "wd" with arguments.
//
//nolint:cyclop,gocognit
func runTarget(
	ctx context.Context,
	sig chan os.Signal,
	fn targetFn,
	wd string,
	rng *ring.Ring,
) error {

	ctx, cxl := context.WithCancel(ctx)
	defer cxl()

	// Run the target in a goroutine. The channel is buffered so the single
	// send (normal return, panic path, chdir-error path, or getwd-error path)
	// never blocks when runTarget has already returned via the signal or
	// context-cancellation paths below. An unbuffered channel would leak the
	// goroutine and skip its deferred os.Chdir, violating Execute's contract
	// of restoring the original working directory. Buffer size 1 is enough:
	// the deferred path always sends once, then closes.
	done := make(chan error, 1)
	go func() {
		// Remember the working directory before running the target.
		cwd, err := os.Getwd()
		if err != nil {
			done <- fmt.Errorf("working directory: %w", err)
			close(done)
			return
		}

		var tgtErr error
		var returned bool // The target returned rather than calling Goexit.
		defer func() {
			if v := recover(); v != nil {
				err = RecoverError(v)
				tgtErr = fmt.Errorf("target panicked with: %w", err)
			} else if !returned && tgtErr == nil {
				tgtErr = errors.New("target exited without returning")
			}
			if err = os.Chdir(cwd); err != nil {
				if tgtErr != nil {
					done <- fmt.Errorf("%w (restore cwd: %w)", tgtErr, err)
				} else {
					done <- fmt.Errorf("restore cwd: %w", err)
				}
			} else {
				done <- tgtErr
			}
			close(done)
		}()

		// Change the working directory before executing the target.
		if err = os.Chdir(wd); err != nil {
			tgtErr = err
			return
		}

		// This is where the target is actually run.
		tgtErr = fn(ctx, rng)
		returned = true
	}()

	// Wait for a signal on sig, a context cancel, or the target result. Once
	// the result arrives, drain until the goroutine exits (deferred cwd
	// restore) without re-selecting on ctx/sig — a finished target must not
	// be reported as a deadline/signal error during that restore window. When
	// cancel/signal races with completion, prefer a result already on done.
	select {
	case itf := <-sig:
		cxl() // Notify the running target the context has been canceled.
		// Prefer a result that raced the signal, but not a cooperative cancel
		// that only mirrors it — keep the 128+n interrupt code.
		if ok, err := recvDone(done); ok {
			if err == nil || !errors.Is(err, context.Canceled) {
				return err
			}
		} else {
			// Let the target run its deferred cleanup (killing its
			// subprocesses) before the process exits; a second signal stops
			// the wait.
			awaitDone(done, sig, signalGrace)
		}
		return interruptedError{sig: itf}

	case <-ctx.Done():
		if ok, err := recvDone(done); ok {
			return timedOut(ctx, err)
		}
		return timedOut(ctx, ctx.Err())

	case err, open := <-done:
		if !open {
			return errNoResult // The goroutine always sends before closing.
		}
		return timedOut(ctx, drainDone(done, err))
	}
}

// timedOut marks err with errTimeout when the --timeout deadline canceled ctx
// and err reports that deadline, so only gomake's own timeout maps to
// exitCodeDeadline. Any other error is returned unchanged.
func timedOut(ctx context.Context, err error) error {
	if !errors.Is(err, context.DeadlineExceeded) ||
		!errors.Is(context.Cause(ctx), errTimeout) {
		return err
	}
	return fmt.Errorf("%w: %w", errTimeout, err)
}

// awaitDone waits until done is closed, a second signal arrives on sig, or
// grace elapses, whichever comes first.
func awaitDone(done <-chan error, sig <-chan os.Signal, grace time.Duration) {
	tmr := time.NewTimer(grace)
	defer tmr.Stop()
	for {
		select {
		case _, open := <-done:
			if !open {
				return
			}
		case <-sig:
			return
		case <-tmr.C:
			return
		}
	}
}

// recvDone non-blocking-reads a finished target result from done. When a
// value is present it drains the channel (cwd restore) and returns ok true
// with the result. A closed channel yields ok true with errNoResult. When the
// channel is empty, ok is false.
func recvDone(done <-chan error) (ok bool, err error) {
	select {
	case err, open := <-done:
		if !open {
			return true, errNoResult
		}
		return true, drainDone(done, err)
	default:
		return false, nil
	}
}

// drainDone waits until done is closed after the first result err, so the
// target goroutine can finish its deferred cwd restore.
func drainDone(done <-chan error, err error) error {
	for {
		if _, open := <-done; !open {
			return err
		}
	}
}

// HelpTargets returns formatted help with the list of targets and their
// synopses. The default target is marked with an asterisk. An empty slice
// yields "no targets\n"; a non-empty slice whose targets are all hidden
// yields an empty string.
func HelpTargets(tgs []*Target, padding int) string {
	buf := &bytes.Buffer{}
	pad := strings.Repeat(" ", padding)
	if len(tgs) == 0 {
		_, _ = fmt.Fprintf(buf, "%sno targets\n", pad)
		return buf.String()
	}

	tgs = append([]*Target(nil), tgs...)
	sort.Slice(tgs, func(i, j int) bool {
		return tgs[i].Name < tgs[j].Name
	})

	tw := tabwriter.NewWriter(buf, 0, 8, 4, ' ', 0)
	var user, core int
	for _, tgt := range tgs {
		if tgt.Hidden {
			continue
		}
		action := tgt.Name
		if tgt.Default {
			action += "*"
		}
		if tgt.Name != "" && tgt.Name[0] == ':' {
			core++
		} else {
			if user == 0 && core > 0 {
				_, _ = fmt.Fprintf(tw, "%s%s\t%s\n", pad, "", "")
			}
			user++
		}
		_, _ = fmt.Fprintf(tw, "%s%s\t%s\n", pad, action, tgt.Synopsis)
	}
	_ = tw.Flush()

	// The tabwriter pads every cell to its column width, so targets without a
	// synopsis get trailing spaces. Trim them off each line.
	lines := strings.Split(buf.String(), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}

// HelpUsage returns formatted help for the whole command or for a target. If
// the args slice is not empty, it will show the help for the target whose name
// is the first element in the slice. Otherwise, it returns the command help
// built by helpCommand.
//
// Returns [ErrUnkTarget] when the target the help was requested for does not
// exist. The name argument is usually set to the name of a binary being
// executed.
func HelpUsage(
	name string,
	args []string,
	fs *xflag.FlagSet,
	tgs []*Target,
) (string, error) {

	if len(args) > 0 {
		// Help about the specific target.
		tgt, err := FindTarget(args[0], tgs)
		if err != nil {
			return "", err
		}
		return tgt.help(), nil
	}
	// General help.
	return helpCommand(name, fs, tgs), nil
}

// helpCommand returns formatted help for a command. The name argument is
// usually set to the name of a binary being executed.
func helpCommand(name string, fs *xflag.FlagSet, tgs []*Target) string {
	buf := &bytes.Buffer{}
	_, _ = fmt.Fprintf(buf, "%s [options] [target] [args]\n\n", name)
	_, _ = fmt.Fprint(buf, "The makefile is a make-like target runner.\n\n")
	_, _ = fmt.Fprint(buf, "options:\n")
	_, _ = fmt.Fprint(buf, fs.HelpOptions())
	_, _ = fmt.Fprint(buf, "\n")
	_, _ = fmt.Fprint(buf, "targets:\n")
	_, _ = fmt.Fprint(buf, HelpTargets(tgs, 2))
	return buf.String()
}

// help returns formatted help for a target.
func (tgt *Target) help() string {
	return fmt.Sprintf("%s\t%s\n", tgt.Name, tgt.Doc)
}

// Indent indents all lines in v with n tab characters.
func Indent(n int, v string) string {
	pad := strings.Repeat("\t", n)
	lines := strings.Split(v, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}
