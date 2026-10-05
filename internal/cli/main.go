// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/internal/builtin"
	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/pkg/gomake"
)

// Main is the gomake entrypoint: it resolves the requested target and runs it,
// returning the process exit code. The rng carries the command-line arguments,
// environment, and standard streams; ver is the version string reported by
// --version and recorded for the makefile; bip supplies the built-in targets
// (including any external targets compiled in) and the pre-run hooks.
//
// The work is dispatched from the parsed arguments. The meta-commands
// --version, --list, and --help print to stderr and return early, as does bash
// completion (driven by the COMP_LINE environment variable). A requested
// built-in target runs straight away without touching the makefile; otherwise
// the sources are analyzed, and a makefile is built, and --bin compiles that
// makefile to a standalone binary instead of executing a target. When a target
// runs, the pre-run hooks fire first.
//
// The exit code mirrors the outcome: 0 on success, 1 on a panic (recovered
// here) or a setup failure, and the mkf/gomake exit codes for a missing
// makefile, an unknown target, a compiler error, or a non-zero target run.
//
//nolint:cyclop,gocognit
func Main(
	ctx context.Context,
	rng *ring.Ring,
	ver string,
	bip builtin.Provider,
) (code int) {

	writeErr := func(err error) {
		_, _ = fmt.Fprintf(rng.Stderr(), "%s: %s\n", binName, err)
	}
	fail := func(err error, status int) int {
		writeErr(err)
		return status
	}

	defer func() {
		if v := recover(); v != nil {
			err := mkf.RecoverError(v)
			writeErr(fmt.Errorf("panicked with: %w", err))
			code = 1
		}
	}()

	cfg, err := newConfig(ver, rng)
	if err != nil {
		return fail(err, 1)
	}

	rng = rng.SetArgs(cfg.args) // Config may have consumed some arguments.
	// Public contract: GOMAKE_VERSION / GOMAKE_PROJECT_DIR are process env
	// keys (see pkg/gomake). EnvSet puts them into the ring so EnvAll ferries
	// them into the makefile subprocess; MetaSet keeps them for in-process
	// meta readers.
	rng.EnvSet(gomake.VersionEnvKey, ver)
	rng.EnvSet(gomake.ProjectDirEnvKey, cfg.src)
	rng.MetaSet(gomake.VersionEnvKey, ver)
	rng.MetaSet(gomake.ProjectDirEnvKey, cfg.src)

	if cfg.showVersion {
		_, _ = fmt.Fprintln(rng.Stdout(), ver)
		return 0
	}

	if cfg.showComplete {
		var msg string
		msg, err = runComplete(rng)
		if msg != "" {
			_, _ = fmt.Fprint(rng.Stderr(), msg)
		}
		if err != nil {
			writeErr(err)
		}
		return mkf.ExitCode(err)
	}

	tgs := bip.Targets()

	// When Bash calls the command to perform completion, it will set several
	// environment variables, including COMP_LINE. If this variable is set, the
	// go make returns the list of completion words for compgen program.
	// Known limitation: tab-completion only covers built-in targets.
	// User-defined targets require compiling the makefile, which is too
	// slow for the completion path.
	if _, ok := rng.EnvLookup("COMP_LINE"); ok {
		_, _ = fmt.Fprint(rng.Stdout(), complete(rng.Args(), tgs))
		return 0
	}

	// Prepare temporary directory.
	fi, err := os.Stat(cfg.tmp)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fail(err, mkf.ExitCode(err))
		}
		if err = os.MkdirAll(cfg.tmp, 0o750); err != nil {
			return fail(err, mkf.ExitCode(err))
		}
	} else if !fi.IsDir() {
		return fail(fmt.Errorf("%s must be a directory", cfg.tmp), 1)
	}

	if cfg.showCheckConfig {
		var report string
		report, err = runCheckConfig(rng, cfg, tgs)
		if err != nil {
			return fail(err, mkf.ExitCode(err))
		}
		_, _ = fmt.Fprint(rng.Stderr(), report)
		return 0
	}

	if cfg.showList {
		var all []*mkf.Target
		all, err = allTargets(rng, cfg, tgs)
		if err != nil {
			return fail(err, mkf.ExitCode(err))
		}
		_, _ = fmt.Fprint(rng.Stdout(), mkf.HelpTargets(all, 0))
		return 0
	}

	if cfg.showHelp {
		var all []*mkf.Target
		all, err = allTargets(rng, cfg, tgs)
		if err != nil {
			return fail(err, mkf.ExitCode(err))
		}
		// HelpUsage expects positionals only (target name, if any). cfg.args
		// also carries makefile flags such as --timeout / --wd, so pass the
		// target name alone rather than slicing past a presumed argv0.
		var helpArgs []string
		if cfg.target != "" {
			helpArgs = []string{cfg.target}
		}
		var out string
		out, err = mkf.HelpUsage(binName, helpArgs, cfg.fs, all)
		if err != nil {
			return fail(err, 1)
		}
		_, _ = fmt.Fprint(rng.Stderr(), out)
		return 0
	}

	// runPreRuns fires provider pre-run hooks before any target execution.
	runPreRuns := func() int {
		for _, fn := range bip.PreRuns() {
			if ctx, rng, err = fn(ctx, rng); err != nil {
				return fail(err, 1)
			}
		}
		return 0
	}

	// When bin is provided, we create binary instead of running them.
	if cfg.bin == "" && cfg.target != "" {
		// If the target to execute is one of the built-in targets, we don't
		// have to parse makefiles or compile anything... we can run it right
		// away.
		if tgt, _ := mkf.FindTarget(cfg.target, tgs); tgt != nil {
			if code = runPreRuns(); code != 0 {
				return code
			}
			if err = applyExternalTargetMeta(ctx, rng, cfg.src); err != nil {
				return fail(err, 1)
			}
			if err = deliverTargetConfig(rng, cfg, tgt, tgs); err != nil {
				return fail(err, 1)
			}
			if code, err = runWithoutCompile(ctx, rng, ver, tgs); err != nil {
				writeErr(err)
			}
			return code
		}
	}

	var gmk *goMake
	analyzeAct := func() error {
		gmk, err = newGoMake(rng, cfg)
		return err
	}
	err = withProgress(rng.Stderr(), "Analyzing sources...", analyzeAct)
	switch {
	case errors.Is(err, errNoMakefile) || errors.Is(err, gomake.ErrNoGoMod):
		// Outside a module, report the real go.mod error rather than
		// "unknown target" when the user named a target.
		if errors.Is(err, gomake.ErrNoGoMod) && cfg.target != "" {
			return fail(err, mkf.ExitCode(err))
		}
		if cfg.target != "" {
			return fail(mkf.ErrUnkTarget, mkf.ExitCodeUnkTarget)
		}

		if cfg.bin != "" {
			return fail(err, mkf.ExitCode(err))
		}

		if code = runPreRuns(); code != 0 {
			return code
		}
		if code, err = runWithoutCompile(ctx, rng, ver, tgs); err != nil {
			writeErr(err)
			mkf.Reraise(err)
		}
		return code

	case err != nil:
		return fail(err, mkf.ExitCode(err))
	}

	buildDir := gmk.cu.BuildDir
	defer func() { _ = os.RemoveAll(buildDir) }()
	defer watchBuildDir(buildDir)()

	for _, name := range gmk.cu.Ignored {
		_, _ = fmt.Fprint(rng.Stderr(), ignoreWarning(name))
	}

	if cfg.bin != "" {
		// If there are no custom targets, there is no point compiling custom
		// binary which would have "the same content" as gomake binary.
		if gmk.targets.Len() == 0 {
			return fail(errNoTargets, mkf.ExitCode(errNoTargets))
		}

		compileAct := func() error {
			return gmk.Compile(ctx, rng.EnvAll(), cfg.bin)
		}
		err = withProgress(rng.Stderr(), "Compiling makefile...", compileAct)
		if err != nil {
			return fail(err, compileExit(err))
		}
		return 0
	}

	if code = runPreRuns(); code != 0 {
		return code
	}

	if err = applyExternalTargetMeta(ctx, rng, cfg.src); err != nil {
		return fail(err, 1)
	}
	tgt := invokedTarget(cfg, gmk.targets)
	err = deliverTargetConfig(rng, cfg, tgt, gmk.targets.List())
	if err != nil {
		return fail(err, 1)
	}
	if err = gmk.Execute(ctx, rng); err != nil {
		if _, ok := errors.AsType[*errCompile](err); ok {
			return fail(err, compileExit(err))
		}
		if !gomake.HasRun(err) {
			writeErr(err)
		}
		return gomake.ExitStatus(err)
	}
	return 0
}

// watchBuildDir removes dir when SIGINT or SIGTERM arrives, then re-raises
// the signal so the process terminates as it would without the watch. The
// returned stop ends the watch and does not remove dir. Callers still remove
// dir when the run returns; this covers a signal that skips that cleanup.
func watchBuildDir(dir string) (stop func()) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case s := <-sig:
			_ = os.RemoveAll(dir)
			signal.Stop(sig)
			prc, err := os.FindProcess(os.Getpid())
			if err == nil {
				err = prc.Signal(s)
			}
			if err != nil {
				os.Exit(1) // The signal cannot be re-raised on this OS.
			}

		case <-done:
		}
	}()
	return func() {
		signal.Stop(sig)
		close(done)
	}
}

// compileExit is the exit code for a compile failure. Context
// cancel/deadline wrapped in *errCompile use the context exit codes, not
// ExitCodeCompile.
func compileExit(err error) int {
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) {
		return mkf.ExitCode(err)
	}
	if _, ok := errors.AsType[*errCompile](err); ok {
		return mkf.ExitCodeCompile
	}
	return mkf.ExitCode(err)
}

// applyExternalTargetMeta loads [TargetsFile] from srcDir and sets
// Ring.meta for each entry that has a "config" field. The meta-key is the
// entry's namespace or the last path segment of the import path. Two configs
// that resolve to the same key return errDupMetaKey.
//
// A load error is intentionally ignored: applying external-target metadata is
// best-effort, and a missing or malformed targets file must not abort the run.
func applyExternalTargetMeta(
	ctx context.Context,
	rng *ring.Ring,
	srcDir string,
) error {

	pth := filepath.Join(srcDir, TargetsFile)
	cfg, err := LoadExternalTargets(ctx, rng, pth)
	if err != nil {
		// A missing or malformed targets file must not abort the run.
		return nil //nolint:nilerr
	}
	seen := make(map[string]string, len(cfg.imports))
	for _, ent := range cfg.imports {
		if len(ent.Config) == 0 {
			continue
		}
		key := ent.MetaKey()
		if prev, ok := seen[key]; ok {
			format := "%w %q: %s and %s"
			return fmt.Errorf(format, errDupMetaKey, key, prev, ent.Path)
		}
		seen[key] = ent.Path
		rng.MetaSet(key, string(ent.Config))
	}
	return nil
}

// runWithoutCompile runs a target without compiling a makefile. It is an
// optimization for built-in targets. It returns the exit code and the error
// to report: a NewMakefile failure exits with ExitCodeErr, an Execute
// failure with the code ExitCode gives it.
func runWithoutCompile(
	ctx context.Context,
	rng *ring.Ring,
	ver string,
	tgs []*mkf.Target,
) (int, error) {

	verOF := mkf.WithMakefileVersion(ver)
	rngOF := mkf.WithMakefileRing(rng)
	cmf, err := mkf.NewMakefile(tgs, rngOF, verOF)
	if err != nil {
		return mkf.ExitCodeErr, err
	}
	if err = cmf.Execute(ctx); err != nil {
		return mkf.ExitCode(err), err
	}
	return 0, nil
}

// complete returns the space-joined bash command-line completion suggestions
// for the targets, given the completion args. It returns an empty string when
// args do not describe a completion request or the word is already completed.
func complete(args []string, ts []*mkf.Target) string {
	// Get the current partial word to be completed.
	if len(args) != 3 {
		return ""
	}

	// Meaning of indexes in args slice:
	//  0 - The name of the command.
	//  1 - The current word being completed (empty unless we are in the
	//      middle of typing a word).
	//  2 - The word before the word being completed.
	partial := args[1]

	var words []string
	for _, tgt := range ts {
		if tgt.Hidden {
			continue
		}
		if partial == "" || strings.HasPrefix(tgt.Name, partial) {
			// We already suggested the target, so no more suggestions.
			if args[2] == tgt.Name {
				return ""
			}
			words = append(words, tgt.Name)
		}
	}
	return strings.Join(words, " ")
}
