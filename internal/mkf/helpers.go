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
	"syscall"
	"text/tabwriter"

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

	// exitCodeDeadline is exit code used when target exceeded given timeout.
	exitCodeDeadline = 125

	// ExitCodePickTarget is the exit code used with [ErrPickTarget] error.
	ExitCodePickTarget = 126

	// ExitCodeUnkTarget is exit code used with [ErrUnkTarget] error.
	ExitCodeUnkTarget = 127

	// exitCodeSignal is the base number to which signal number which stopped
	// the gomake is added (128+n).
	exitCodeSignal = 128

	// ExitCodeCompile is exit code used when compilation of the makefiles and
	// generated files failed.
	ExitCodeCompile = 129
)

// ExitCode returns an exit code associated with the given error. If the error
// is nil, it returns 0.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return ExitCodeOK
	case errors.Is(err, context.DeadlineExceeded):
		return exitCodeDeadline
	case errors.Is(err, ErrPickTarget):
		return ExitCodePickTarget
	case errors.Is(err, ErrUnkTarget):
		return ExitCodeUnkTarget
	default:
		if e, ok := errors.AsType[interruptedError](err); ok {
			return e.Signal()
		}
		return ExitCodeErr
	}
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
// when there is no target by that name.
func FindTarget(name string, tgs []*Target) (*Target, error) {
	for _, tgt := range tgs {
		if tgt.Name == name {
			return tgt, nil
		}
	}
	return nil, ErrUnkTarget
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
			done <- err
			close(done)
			return
		}

		var tgtErr error
		defer func() {
			if v := recover(); v != nil {
				rerr := RecoverError(v)
				tgtErr = fmt.Errorf("target panicked with: %w", rerr)
			}
			if rerr := os.Chdir(cwd); rerr != nil {
				if tgtErr != nil {
					done <- fmt.Errorf("%w (restore cwd: %w)", tgtErr, rerr)
				} else {
					done <- rerr
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
	}()

	// Handle interrupt and termination (containers send SIGTERM).
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal, context cancel, or the target result. Once the result
	// arrives, drain until the goroutine exits (deferred cwd restore) without
	// re-selecting on ctx/sig — a finished target must not be reported as a
	// deadline/signal error during that restore window. When cancel/signal
	// races with completion, prefer a result already on done.
	for {
		select {
		case itf := <-sig:
			cxl() // Notify the running target the context has been canceled.
			// Prefer a finished result, but not a cooperative cancel that
			// only mirrors our signal — keep the 128+n interrupt exit code.
			if err, ok := recvDone(done); ok {
				if err == nil || !errors.Is(err, context.Canceled) {
					return err
				}
			}
			if i, ok := itf.(syscall.Signal); ok {
				return interruptedError(exitCodeSignal + int(i))
			}
			return interruptedError(1)

		case <-ctx.Done():
			if err, ok := recvDone(done); ok {
				return err
			}
			return ctx.Err()

		case err, open := <-done:
			if !open {
				// Closed without a prior result (should not happen).
				return nil
			}
			return drainDone(done, err)
		}
	}
}

// recvDone non-blocking-reads a finished target result from done. When a
// value is present it drains the channel (cwd restore) and returns the
// result with ok true. When the channel is empty, ok is false.
func recvDone(done <-chan error) (err error, ok bool) {
	select {
	case err, open := <-done:
		if !open {
			return nil, true
		}
		return drainDone(done, err), true
	default:
		return nil, false
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
	var empty, core int
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
			if empty == 0 && core > 0 {
				_, _ = fmt.Fprintf(tw, "%s%s\t%s\n", pad, "", "")
			}
			empty++
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
// is the first element in the slice. Otherwise, it will call [helpCommand].
//
// Returns ErrUnkTarget error if target's, the help was requested for, does
// not exist. The name argument is usually set to the name of a binary being
// executed.
func HelpUsage(
	name string,
	args []string,
	fs *xflag.FlagSet,
	tgs []*Target,
) (string, error) {

	if len(args) > 0 {
		// Write help about the specific target to the standard error.
		tgt, err := FindTarget(args[0], tgs)
		if err != nil {
			return "", err
		}
		return helpTarget(tgt), nil
	}
	// Write general help to the standard error.
	return helpCommand(name, fs, tgs), nil
}

// helpCommand returns formatted help for a command. The name argument is
// usually set to the name of a binary being executed.
func helpCommand(name string, fs *xflag.FlagSet, tgs []*Target) string {
	buf := &bytes.Buffer{}
	_, _ = fmt.Fprintf(buf, "%s [options] [target] [args]\n\n", name)
	_, _ = fmt.Fprint(buf, "The makefile is a make-like target runner.\n\n")
	_, _ = fmt.Fprint(buf, "options:\n")
	_, _ = fmt.Fprint(buf, xflag.HelpOptions(fs))
	_, _ = fmt.Fprint(buf, "\n")
	_, _ = fmt.Fprint(buf, "targets:\n")
	_, _ = fmt.Fprint(buf, HelpTargets(tgs, 2))
	return buf.String()
}

// helpTarget returns formatted help for a target.
func helpTarget(tgt *Target) string {
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
