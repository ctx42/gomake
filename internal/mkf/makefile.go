// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mkf

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xflag/pkg/xflag"
)

// --- CODE MARK ---

// MakefileBin represents the name of the compiled makefile. It's the result of
// compiling files in the build directory.
const MakefileBin = "makefile"

// targetNameKey is the context key under which [Makefile.Execute] stores the
// running target's name. It is a plain string whose value must match the key
// used by [github.com/ctx42/gomake/pkg/gomake.TargetName], so a user target can
// read its own name even though this runtime is inlined into the generated
// makefile's "main" package and cannot share a defined key type with gomake.
const targetNameKey = "github.com/ctx42/gomake/pkg/gomake.targetName"

// Target execution errors.
var (
	// ErrInvTimeout is returned when an invalid timeout value is provided.
	ErrInvTimeout = errors.New("invalid timeout")

	// ErrUnkTarget is an error returned when a target name is not defined in
	// the makefile.
	ErrUnkTarget = errors.New("unknown target")

	// ErrPickTarget is an error returned when makefile has no default target
	// and none was provided.
	ErrPickTarget = errors.New("pick a target to execute")
)

// interruptedError wraps signal code and is returned when target execution has
// been interrupted by an OS signal.
type interruptedError int

func (ier interruptedError) Error() string { return "target interrupted" }

// Signal returns signal code.
func (ier interruptedError) Signal() int { return int(ier) }

// WithMakefileVersion is the [NewMakefile] option setting the [Makefile]
// version.
func WithMakefileVersion(version string) func(*Makefile) {
	return func(cmf *Makefile) { cmf.version = version }
}

// WithMakefileArgs is the [NewMakefile] option setting the [Makefile]
// arguments. If you call this function without arguments, the [Makefile]
// arguments are set to an empty slice instead of [os.Args].
func WithMakefileArgs(args ...string) func(*Makefile) {
	return func(cmf *Makefile) {
		if args == nil {
			args = []string{}
		}
		cmf.rng = cmf.rng.SetArgs(args)
	}
}

// WithMakefileRing is the [NewMakefile] option setting ring to use.
func WithMakefileRing(rng *ring.Ring) func(*Makefile) {
	return func(cmf *Makefile) { cmf.rng = rng }
}

// Makefile runs targets.
type Makefile struct {
	// Working directory for a target. By default, it's set to the current
	// working directory.
	wd string

	// Controls how much time a target has to finish. By default, it's set to
	// zero (no deadline), but its value can be set by the "-- timeout" option.
	timeout time.Duration

	// List of targets.
	targets []*Target

	// Global configuration flag set.
	fs *xflag.FlagSet

	// Makefile environment and I/O streams.
	rng *ring.Ring

	// Makefile version.
	version string

	// Show general help or extended target help.
	showHelp bool

	// Option target.
	// It's set to a non-nil value if an option like "--version" or "--list" is
	// provided through command options.
	optionTgt targetFn
}

// NewMakefile returns a new instance of Makefile. By default, [os.Args],
// [os.Stdout] and [os.Stderr] are used.
func NewMakefile(tgs []*Target, opts ...func(*Makefile)) (*Makefile, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	cmf := &Makefile{
		targets: tgs,
		rng:     ring.New(),
		version: "unknown version",
	}
	for _, opt := range opts {
		opt(cmf)
	}
	cmf.fs = xflag.NewFlagSet("makefile", flag.ContinueOnError)
	cmf.fs.SetOutput(io.Discard)

	// Option flags.
	cmf.fs.StringVar(&cmf.wd, "wd", wd, "working directory for a target")
	cmf.fs.DurationVar(
		&cmf.timeout,
		"timeout",
		cmf.timeout,
		"target execution timeout (default: 0)",
	)

	fHelp := cmf.fs.BoolSL(
		"help",
		"h",
		false,
		"show this help or target documentation",
	)
	fVersion := cmf.fs.Bool("version", false, "print version")
	fList := cmf.fs.Bool("list", false, "show available targets")
	cmf.fs.Usage = func() {}

	if err = cmf.fs.Parse(cmf.rng.Args()); err != nil {
		// Map an unparsable -timeout value to the domain error; any other
		// parse failure propagates unchanged.
		pe, ok := errors.AsType[*xflag.ParseError](err)
		if ok && pe.Flag == "timeout" {
			return nil, ErrInvTimeout
		}
		return nil, fmt.Errorf("parsing flags: %w", err)
	}
	cmf.rng = cmf.rng.SetArgs(cmf.fs.Args())

	switch {
	case *fHelp:
		cmf.showHelp = true
	case *fVersion:
		cmf.optionTgt = fTgtVersion(cmf.version)
	case *fList:
		cmf.optionTgt = fTgtList(cmf.targets)
	}
	return cmf, nil
}

// Execute executes a target. Target is picked based on arguments passed to the
// command.
//
// Execute may change the working directory. On normal completion it restores
// the original working directory before returning. On timeout or signal
// cancellation it returns immediately while the target goroutine restores the
// directory asynchronously, so the restore may not be visible on return.
func (cmf *Makefile) Execute(ctx context.Context) error {
	// Help was requested.
	if cmf.showHelp {
		help, err := HelpUsage(
			MakefileBin,
			cmf.rng.Args(),
			cmf.fs,
			cmf.targets,
		)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprint(cmf.rng.Stderr(), help)
		return nil
	}

	// Execute option target.
	if cmf.optionTgt != nil {
		return cmf.optionTgt(ctx, cmf.rng)
	}

	// Execute target.
	args := cmf.rng.Args()
	tgt, err := pickTarget(cmf.targets, &args)
	if err != nil {
		return err
	}
	cmf.rng = cmf.rng.SetArgs(args)

	sig := make(chan os.Signal, 1)
	defer signal.Stop(sig)

	var cxl context.CancelFunc
	if cmf.timeout > 0 {
		ctx, cxl = context.WithTimeout(ctx, cmf.timeout)
	} else {
		ctx, cxl = context.WithCancel(ctx)
	}
	defer cxl()

	// A string key is required so the value can be read via
	// pkg/gomake.TargetName across the inline boundary; see targetNameKey.
	ctx = context.WithValue(ctx, targetNameKey, tgt.Name) //nolint:staticcheck
	return runTarget(ctx, sig, tgt.Run, cmf.wd, cmf.rng)
}

// fTgtVersion returns an option target that prints the version.
func fTgtVersion(version string) targetFn {
	return func(_ context.Context, rng *ring.Ring) error {
		_, _ = fmt.Fprintln(rng.Stderr(), version)
		return nil
	}
}

// fTgtList returns an option target that prints the list of available targets.
func fTgtList(tgs []*Target) targetFn {
	return func(_ context.Context, rng *ring.Ring) error {
		_, _ = fmt.Fprint(rng.Stderr(), HelpTargets(tgs, 0))
		return nil
	}
}
