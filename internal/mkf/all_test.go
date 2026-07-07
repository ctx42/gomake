// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mkf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testkit/pkg/selfkit"

	"github.com/ctx42/gomake/pkg/gomake"
)

func TestMain(m *testing.M) {
	runTests, exitCode := selfkit.New().Run(os.Stdout, os.Stderr)
	if runTests {
		os.Exit(m.Run())
	}
	os.Exit(exitCode)
}

// /////////////////////////////////////////////////////////////////////////////

// TgtA returns target printing "a" to standard output. Always returns
// nil error.
func TgtA() *Target {
	return &Target{
		Name:     "tgt-a",
		CodeRef:  "TgtA",
		Synopsis: "syn tgt-a",
		Doc:      "doc tgt-a",
		Run: func(_ context.Context, rng *ring.Ring) error {
			_, _ = fmt.Fprint(rng.Stdout(), "a")
			return nil
		},
	}
}

// TgtB returns target printing "b" to standard output and is set as default.
// Always returns nil error.
func TgtB() *Target {
	return &Target{
		Name:     "tgt-b",
		CodeRef:  "TgtB",
		Synopsis: "syn tgt-b",
		Doc:      "doc tgt-b",
		Run: func(_ context.Context, rng *ring.Ring) error {
			_, _ = fmt.Fprint(rng.Stdout(), "b")
			return nil
		},
		Default: true,
	}
}

// TgtC returns target printing "c" to standard output. Always returns nil
// error.
func TgtC() *Target {
	return &Target{
		Name:     "tgt-c",
		CodeRef:  "TgtC",
		Synopsis: "syn tgt-c",
		Doc:      "doc tgt-c",
		Run: func(_ context.Context, rng *ring.Ring) error {
			_, _ = fmt.Fprint(rng.Stdout(), "c")
			return nil
		},
	}
}

// TgtCore returns target printing "core" to standard output. Always returns
// nil error.
func TgtCore() *Target {
	return &Target{
		Name:     ":tgt-core",
		CodeRef:  "TgtCore",
		Synopsis: "syn tgt-core",
		Doc:      "doc tgt-core",
		Run: func(_ context.Context, rng *ring.Ring) error {
			_, _ = fmt.Fprint(rng.Stdout(), "core")
			return nil
		},
	}
}

// TgtHidden returns hidden target printing "hidden" to standard output. Always
// returns nil error.
func TgtHidden() *Target {
	return &Target{
		Name:     ":tgt-hidden",
		CodeRef:  "TgtHidden",
		Synopsis: "syn tgt-hidden",
		Doc:      "doc tgt-hidden",
		Hidden:   true,
		Run: func(_ context.Context, rng *ring.Ring) error {
			_, _ = fmt.Fprint(rng.Stdout(), "hidden")
			return nil
		},
	}
}

// TgtPrintArgs returns target printing arguments to the stdout. Always returns
// nil error.
func TgtPrintArgs() *Target {
	return &Target{
		Name:     "tgt-print-args",
		CodeRef:  "TgtPrintArgs",
		Synopsis: "syn tgt-print-args",
		Doc:      "doc tgt-print-args",
		Run: func(_ context.Context, rng *ring.Ring) error {
			_, _ = fmt.Fprintf(rng.Stdout(), "%v", rng.Args())
			return nil
		},
	}
}

// TgtPrintName returns the target printing its own name to the stdout. Always
// returns nil error.
func TgtPrintName() *Target {
	return &Target{
		Name:     "tgt-print-name",
		CodeRef:  "TgtPrintName",
		Synopsis: "syn tgt-print-name",
		Doc:      "doc tgt-print-name",
		Run: func(ctx context.Context, rng *ring.Ring) error {
			name, _ := gomake.TargetName(ctx)
			_, _ = fmt.Fprintf(rng.Stdout(), "%s", name)
			return nil
		},
	}
}

// TgtLong returns target taking 100 milliseconds to print a message to
// standard output. Always returns nil error.
func TgtLong() *Target {
	return &Target{
		Name:     "tgt-long",
		CodeRef:  "TgtLong",
		Synopsis: "syn tgt-long",
		Doc:      "doc tgt-long",
		Run: func(_ context.Context, rng *ring.Ring) error {
			time.Sleep(100 * time.Millisecond)
			_, _ = fmt.Fprint(rng.Stdout(), "done after 100ms")
			return nil
		},
	}
}

// TgtError returns target which always returns an error.
func TgtError() *Target {
	return &Target{
		Name:     "tgt-error",
		CodeRef:  "TgtError",
		Synopsis: "syn tgt-error",
		Doc:      "doc tgt-error",
		Run: func(context.Context, *ring.Ring) error {
			return errors.New("always error")
		},
	}
}

// TgtPanicString returns target panicking with string argument.
func TgtPanicString() *Target {
	return &Target{
		Name:     "tgt-panic-string",
		CodeRef:  "TgtPanicString",
		Synopsis: "syn tgt-panic-string",
		Doc:      "doc tgt-panic-string",
		Run: func(context.Context, *ring.Ring) error {
			panic("panic string")
		},
	}
}

// TgtPanicError returns target panicking with error argument.
func TgtPanicError() *Target {
	return &Target{
		Name:     "tgt-panic-error",
		CodeRef:  "TgtPanicError",
		Synopsis: "syn tgt-panic-error",
		Doc:      "doc tgt-panic-error",
		Run: func(context.Context, *ring.Ring) error {
			panic(errors.New("panic error"))
		},
	}
}

// TgtPanicOther returns target panicking with integer argument.
func TgtPanicOther() *Target {
	return &Target{
		Name:     "tgt-panic-other",
		CodeRef:  "TgtPanicOther",
		Synopsis: "syn tgt-panic-other",
		Doc:      "doc tgt-panic-other",
		Run: func(context.Context, *ring.Ring) error {
			panic(123)
		},
	}
}

// TgtWaiting returns target waiting one second for the context to be canceled,
// or it returns an error. Prints status to standard output.
func TgtWaiting() *Target {
	return &Target{
		Name:     "tgt-waiting",
		CodeRef:  "TgtWaiting",
		Synopsis: "syn tgt-waiting",
		Doc:      "doc tgt-waiting",
		Run: func(ctx context.Context, rng *ring.Ring) error {
			delay := time.NewTimer(time.Second)
			_, _ = fmt.Fprint(rng.Stdout(), "started ")
			defer func() { _, _ = fmt.Fprint(rng.Stdout(), " exited") }()

			select {
			case <-ctx.Done():
				_, _ = fmt.Fprint(rng.Stdout(), "context canceled")
				if !delay.Stop() {
					<-delay.C
				}
				return ctx.Err()
			case <-delay.C:
				return errors.New("signal not received")
			}
		},
	}
}

// TgtListFiles returns target listing files in current working directory. It
// prints file list to status to standard output and header to standard error.
func TgtListFiles() *Target {
	return &Target{
		Name:     "tgt-list-files",
		CodeRef:  "TgtListFiles",
		Synopsis: "syn tgt-list-files",
		Doc:      "doc tgt-list-files",
		Run: func(_ context.Context, rng *ring.Ring) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(rng.Stderr(), "Listing files in: %s\n", wd)
			fls, err := os.ReadDir(wd)
			if err != nil {
				return err
			}

			for _, fil := range fls {
				_, _ = fmt.Fprintf(rng.Stdout(), "%s\n", fil.Name())
			}
			return nil
		},
	}
}

// TgtChdir returns target which changes directory to the path passed as the
// first argument.
func TgtChdir() *Target {
	return &Target{
		Name:     "tgt-chdir",
		CodeRef:  "TgtChdir",
		Synopsis: "syn tgt-chdir",
		Doc:      "doc tgt-chdir",
		Run: func(_ context.Context, rng *ring.Ring) error {
			args := rng.Args()
			if len(args) == 0 {
				return errors.New("must provide directory to change to")
			}
			if err := os.Chdir(args[0]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(rng.Stdout(), "changed to %s", args[0])
			return nil
		},
	}
}
