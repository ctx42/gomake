// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Command `install` builds and installs the gomake binary into GOBIN. Set
// GOBIN (or GOPATH) in the environment to control where the binary lands.
//
// When run from a published version it fetches the module source automatically.
// When run from the local source (go run ./cmd/install) it builds from the
// current working directory.
//
// These are typical invocations.
//
//	go run github.com/ctx42/gomake/cmd/install@latest
//	go run ./cmd/install
//	go run ./cmd/install --targets=path/to/targets.yaml
//	go run ./cmd/install --targets=http://example.com/targets.yaml
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xflag/pkg/xflag"

	"github.com/ctx42/gomake/internal/install"
)

func main() {
	rng := ring.New()
	fs := xflag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.SetOutput(rng.Stderr())
	targets := fs.String("targets", "", "path or URL to a targets.yaml file")
	_ = fs.Parse(os.Args[1:])

	if err := rejectArgs(fs); err != nil {
		_, _ = fmt.Fprintln(rng.Stderr(), err)
		fs.Usage()
		os.Exit(2)
	}

	// Trim so whitespace-only --targets= is treated as empty, matching the
	// note and the value passed to install.Main.
	tgs := strings.TrimSpace(*targets)

	// An explicitly empty --targets= is not an error: report that no external
	// targets were provided and continue installing.
	if note := emptyTargetsNote(fs, tgs); note != "" {
		_, _ = fmt.Fprintln(rng.Stderr(), note)
	}

	info, _ := debug.ReadBuildInfo()
	if err := install.Main(context.Background(), rng, info, tgs); err != nil {
		_, _ = fmt.Fprintln(rng.Stderr(), err)
		os.Exit(1)
	}
}

// rejectArgs reports the first positional argument. The targets file is
// taken only from --targets, so a bare path must not be ignored.
func rejectArgs(fs *xflag.FlagSet) error {
	args := fs.Args()
	if len(args) == 0 {
		return nil
	}
	return fmt.Errorf("gomake: unexpected argument: %q", args[0])
}

// emptyTargetsNote returns the note to print when --targets was set on fs with
// an empty value tgs. Whitespace-only values count as empty. It returns "" when
// the flag was absent or non-empty, so that an explicit --targets= is reported
// rather than treated as an error.
func emptyTargetsNote(fs *xflag.FlagSet, tgs string) string {
	if strings.TrimSpace(tgs) != "" {
		return ""
	}
	if !fs.WasSet("targets") {
		return ""
	}
	return "no external targets provided"
}
