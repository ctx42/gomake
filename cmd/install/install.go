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
//	go run ./cmd/install --targets=a/targets.yaml --targets=b/targets.yaml
//
// The --targets flag may be repeated: the imports of all the files are
// combined, an import listed identically in several files is kept once, and
// the same package imported with a different version, namespace, or config
// fails the install.
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
	var tgs targetsFlag
	usage := "path or URL to a targets.yaml file; may be repeated"
	fs.Var(&tgs, "targets", usage)
	_ = fs.Parse(os.Args[1:])

	if err := rejectArgs(fs); err != nil {
		_, _ = fmt.Fprintln(rng.Stderr(), err)
		fs.Usage()
		os.Exit(2)
	}

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

// emptyTargetsNote returns the note to print when --targets was set on fs but
// collected no file into tgs. It returns "" when the flag was absent or named
// a file, so that an explicit --targets= is reported rather than treated as an
// error.
func emptyTargetsNote(fs *xflag.FlagSet, tgs targetsFlag) string {
	if len(tgs) > 0 {
		return ""
	}
	if !fs.WasSet("targets") {
		return ""
	}
	return "no external targets provided"
}

// targetsFlag collects the values of the repeatable --targets flag. Values
// are trimmed, and whitespace-only ones are dropped.
type targetsFlag []string

var _ flag.Value = (*targetsFlag)(nil)

func (tf *targetsFlag) String() string { return strings.Join(*tf, ",") }

func (tf *targetsFlag) Set(val string) error {
	if val = strings.TrimSpace(val); val != "" {
		*tf = append(*tf, val)
	}
	return nil
}
