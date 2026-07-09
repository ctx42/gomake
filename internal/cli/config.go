// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xflag/pkg/xflag"

	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/internal/parser"
	"github.com/ctx42/gomake/pkg/gomake"
)

// Environment configuration keys.
const (
	// envKeyTmpDir represents environment variable setting absolute path to
	// the temporary directory used by gomake.
	envKeyTmpDir = "GOMAKE_TMP_DIR"
)

// config represents gomake run configuration.
type config struct {
	// Absolute path to a directory where gomake looks for source files.
	// By default, it's set to the current working directory, but its value
	// can be set by "--src" option.
	src string

	// Absolute path to a directory where compiled makefile file will run.
	// By default, it's set to the same directory as src but its value can
	// be set by "--wd" option.
	wd string

	// Absolute path to a temporary directory. By default, it is set to the
	// operating system temporary directory, but its value can be set by the
	// [EnvKeyTmpDir] environment variable or the "--tmp" option. When both are
	// provided, the option takes precedence.
	tmp string

	// Absolute path to the compiled makefile.
	bin string

	// Controls how much time a target has to finish. By default, it's set
	// to zero (no deadline), but its value can be set by "--timeout" option.
	timeout time.Duration

	// Show gomake or target help. By default, it's set to false, but its value
	// can be set by "--help" option.
	showHelp bool

	// Show list of targets. By default, it's set to false, but its value
	// can be set by "--list" option.
	showList bool

	// Show gomake version. By default, it's set to false, but its value
	// can be set by "--version" option.
	showVersion bool

	// Install shell completion. By default, it's set to false, but its value
	// can be set by "--complete" option. When true no other flags or targets
	// may be provided.
	showComplete bool

	// Validate the gomake.yaml files and list target config keys. By default,
	// it's set to false, but its value can be set by "--check-config" option.
	showCheckConfig bool

	// The operating system to compile makefiles for. By default, it is set to
	// [runtime.GOOS].
	goos string

	// The architecture to compile makefiles for. By default, it is set to
	// [runtime.GOARCH].
	goarch string

	// Gomake version.
	version string

	// Global configuration flag set.
	fs *xflag.FlagSet

	// The arguments to pass to makefile along with target name and its
	// arguments (if present).
	args []string

	// The index in args slice pointing to target name. It's set to -1 when
	// target name was not provided to gomake.
	targetIdx int

	// Target name extracted from arguments. It might be empty for invocations
	// without a target name, for example when requesting to run default target.
	target string

	// User-level target configuration tree, keyed by import path. Populated by
	// [config.applyFileConfig].
	userTargets map[string]any

	// Project-level target configuration tree, keyed by import path. Populated
	// by [config.applyFileConfig].
	projectTargets map[string]any
}

// newConfig returns new instance of [config].
func newConfig(ver string, rng *ring.Ring) (*config, error) {
	env := rng.EnvAll()
	cfg := &config{
		goos:      gomake.GetGOOS(env),
		goarch:    gomake.GetGOARCH(env),
		version:   ver,
		targetIdx: -1,
	}
	if err := cfg.parse(env, rng.Args()); err != nil {
		return nil, err
	}
	return cfg, nil
}

// parse sets [config] fields based on program arguments.
//
// nolint: gocognit, cyclop
func (cfg *config) parse(env, args []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	cfg.fs = xflag.NewFlagSet("gomake", flag.ContinueOnError)
	cfg.fs.SetOutput(io.Discard)

	// Option flags that are superset of the makefile flags.
	cfg.fs.StringVar(&cfg.src, "src", wd, "path where to look for targets")
	cfg.fs.StringVar(
		&cfg.wd,
		"wd",
		wd,
		"working directory for compiled makefile",
	)
	cfg.fs.StringVar(
		&cfg.bin,
		"bin",
		"",
		"path to put the compiled makefile in",
	)
	cfg.fs.StringVar(&cfg.tmp, "tmp", "", "absolute path temporary directory")
	cfg.fs.DurationVar(
		&cfg.timeout,
		"timeout",
		cfg.timeout,
		"target execution timeout (default: 0)",
	)
	fHelp := cfg.fs.BoolSL(
		"help",
		"h",
		false,
		"show this help or description of the target",
	)
	cfg.fs.BoolVar(&cfg.showList, "list", false, "show available targets")
	cfg.fs.BoolVar(
		&cfg.showCheckConfig,
		"check-config",
		false,
		"validate gomake.yaml and list target config keys",
	)
	cfg.fs.BoolVar(&cfg.showVersion, "version", false, "print version")
	cfg.fs.BoolVar(
		&cfg.showComplete,
		"complete",
		false,
		"install shell completion",
	)
	cfg.fs.Usage = func() {}

	if err = cfg.fs.Parse(args); err != nil {
		pe, ok := errors.AsType[*xflag.ParseError](err)
		if ok && pe.Flag == "timeout" {
			return mkf.ErrInvTimeout
		}
		return err
	}
	cfg.showHelp = *fHelp

	if cfg.showComplete {
		if cfg.fs.NFlag() > 1 || len(cfg.fs.Args()) > 0 {
			return fmt.Errorf("--complete must be the only option")
		}
		return nil
	}

	// Make source and working directories absolute before loading the
	// gomake.yaml files, which are located relative to the source directory.
	if !filepath.IsAbs(cfg.src) {
		cfg.src = filepath.Join(wd, cfg.src)
	}
	if !filepath.IsAbs(cfg.wd) {
		cfg.wd = filepath.Join(wd, cfg.wd)
	}

	// Load gomake.yaml files; this also resolves the temporary directory and
	// the timeout from the settings section when not set by option or
	// environment.
	if err = cfg.applyFileConfig(env); err != nil {
		return err
	}

	if !filepath.IsAbs(cfg.tmp) {
		return fmt.Errorf("tmp directory %w: %s", parser.ErrAbsPath, cfg.tmp)
	}

	var mkfArgs []string
	// There is no point of setting makefile working directory
	// if it's the same as current working directory.
	if cfg.wd != "" && cfg.wd != wd {
		mkfArgs = append(mkfArgs, "--wd", cfg.wd)
	}
	if cfg.timeout > 0 {
		mkfArgs = append(mkfArgs, "--timeout", cfg.timeout.String())
	}
	if cfg.showHelp {
		mkfArgs = append(mkfArgs, "--help")
	}
	if cfg.showList {
		mkfArgs = append(mkfArgs, "--list")
	}
	if cfg.showVersion {
		mkfArgs = append(mkfArgs, "--version")
	}

	left := cfg.fs.Args()
	if len(left) > 0 {
		cfg.target = left[0]
		cfg.targetIdx = len(mkfArgs)
	}
	mkfArgs = append(mkfArgs, left...)
	cfg.args = mkfArgs

	// Override binary file destination if needed.
	if cfg.bin != "" {
		if !filepath.IsAbs(cfg.bin) {
			cfg.bin = filepath.Join(wd, cfg.bin)
		}
		if gomake.PathExists(cfg.bin) {
			return fmt.Errorf("%w: %s", errBinExists, cfg.bin)
		}

		if cfg.showHelp {
			return fmt.Errorf("-h, --help cannot be used with --bin")
		}

		if cfg.showVersion {
			return fmt.Errorf("--version cannot be used with --bin")
		}

		if cfg.showList {
			return fmt.Errorf("--list cannot be used with --bin")
		}

		if cfg.timeout > 0 {
			return fmt.Errorf("--timeout cannot be used with --bin")
		}

		if cfg.target != "" {
			return fmt.Errorf("--bin cannot be used with targets")
		}
	}

	return nil
}
