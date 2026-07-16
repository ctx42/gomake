// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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

	defer func() {
		if v := recover(); v != nil {
			err := mkf.RecoverError(v)
			fail(rng, fmt.Errorf("panicked with: %w", err))
			code = 1
		}
	}()

	cfg, err := newConfig(ver, rng)
	if err != nil {
		fail(rng, err)
		return 1
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
		_, _ = fmt.Fprintln(rng.Stderr(), ver)
		return 0
	}

	if cfg.showComplete {
		msg, err := runComplete(rng)
		if msg != "" {
			_, _ = fmt.Fprint(rng.Stderr(), msg)
		}
		if err != nil {
			fail(rng, err)
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
			return failCode(rng, err)
		}
		if err = os.MkdirAll(cfg.tmp, 0755); err != nil {
			return failCode(rng, err)
		}
	} else if !fi.IsDir() {
		fail(rng, fmt.Errorf("%s must be a directory", cfg.tmp))
		return 1
	}

	if cfg.showCheckConfig {
		if err = runCheckConfig(rng, cfg, tgs); err != nil {
			return failCode(rng, err)
		}
		return 0
	}

	if cfg.showList {
		all, err := allTargets(rng, cfg, tgs)
		if err != nil {
			return failCode(rng, err)
		}
		_, _ = fmt.Fprint(rng.Stderr(), mkf.HelpTargets(all, 0))
		return 0
	}

	if cfg.showHelp {
		all, err := allTargets(rng, cfg, tgs)
		if err != nil {
			return failCode(rng, err)
		}
		// HelpUsage expects positionals only (target name, if any). cfg.args
		// also carries makefile flags such as --timeout / --wd, so pass the
		// target name alone rather than slicing past a presumed argv0.
		var helpArgs []string
		if cfg.target != "" {
			helpArgs = []string{cfg.target}
		}
		out, err := mkf.HelpUsage(binName, helpArgs, cfg.fs, all)
		if err != nil {
			fail(rng, err)
			return 1
		}
		_, _ = fmt.Fprint(rng.Stderr(), out)
		return 0
	}

	// runPreRuns fires provider pre-run hooks before any target execution.
	runPreRuns := func() int {
		for _, fn := range bip.PreRuns() {
			if ctx, rng, err = fn(ctx, rng); err != nil {
				fail(rng, err)
				return 1
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
			if code := runPreRuns(); code != 0 {
				return code
			}
			applyExternalTargetMeta(rng, cfg.src)
			if err = deliverTargetConfig(rng, cfg, tgt, tgs); err != nil {
				fail(rng, err)
				return 1
			}
			return runWithoutCompile(ctx, rng, ver, tgs)
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
			return failCode(rng, err)
		}
		if cfg.target != "" {
			fail(rng, mkf.ErrUnkTarget)
			return mkf.ExitCodeUnkTarget
		}

		if cfg.bin != "" {
			return failCode(rng, err)
		}

		if code := runPreRuns(); code != 0 {
			return code
		}
		return runWithoutCompile(ctx, rng, ver, tgs)

	case err != nil:
		return failCode(rng, err)
	}

	buildDir := gmk.cu.BuildDir
	defer func() { _ = os.RemoveAll(buildDir) }()

	for _, name := range gmk.cu.Ignored {
		_, _ = fmt.Fprint(rng.Stderr(), ignoreWarning(name))
	}

	if cfg.bin != "" {
		// If there are no custom targets, there is no point compiling custom
		// binary which would have "the same content" as gomake binary.
		if gmk.targets.Len() == 0 {
			return failCode(rng, errNoMakefile)
		}

		compileAct := func() error {
			_, err = gmk.Compile(ctx, rng.EnvAll(), cfg.bin)
			return err
		}
		err = withProgress(rng.Stderr(), "Compiling makefile...", compileAct)
		if err != nil {
			return compileFailCode(rng, err)
		}
		return 0
	}

	if code := runPreRuns(); code != 0 {
		return code
	}

	applyExternalTargetMeta(rng, cfg.src)
	tgt := invokedTarget(cfg, gmk.targets)
	err = deliverTargetConfig(rng, cfg, tgt, gmk.targets.List())
	if err != nil {
		fail(rng, err)
		return 1
	}
	if err = gmk.Execute(ctx, rng); err != nil {
		if _, ok := errors.AsType[*errCompile](err); ok {
			return compileFailCode(rng, err)
		}
		if !gomake.HasRun(err) {
			fail(rng, err)
		}
		return gomake.ExitStatus(err)
	}
	return 0
}

// compileFailCode reports a compile failure. Context cancel/deadline wrapped
// in *errCompile use the context exit codes, not ExitCodeCompile.
func compileFailCode(rng *ring.Ring, err error) int {
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) {
		return failCode(rng, err)
	}
	if _, ok := errors.AsType[*errCompile](err); ok {
		fail(rng, err)
		return mkf.ExitCodeCompile
	}
	return failCode(rng, err)
}

// applyExternalTargetMeta loads [gomake.TargetsFile] from srcDir and sets
// Ring.meta for each entry that has a "config" field. The meta-key is the
// entry's namespace or the last path segment of the import path.
//
// A load error is intentionally ignored: applying external-target metadata is
// best-effort, and a missing or malformed targets file must not abort the run.
func applyExternalTargetMeta(rng *ring.Ring, srcDir string) {
	pth := filepath.Join(srcDir, TargetsFile)
	cfg, err := LoadExternalTargets(context.Background(), pth)
	if err != nil {
		return
	}
	for _, ent := range cfg.imports {
		if len(ent.Config) > 0 {
			rng.MetaSet(ent.MetaKey(), string(ent.Config))
		}
	}
}

// runWithoutCompile runs a target without compiling a makefile. It is an
// optimization for built-in targets.
func runWithoutCompile(
	ctx context.Context,
	rng *ring.Ring,
	ver string,
	tgs []*mkf.Target,
) int {

	verOF := mkf.WithMakefileVersion(ver)
	rngOF := mkf.WithMakefileRing(rng)
	cmf, err := mkf.NewMakefile(tgs, rngOF, verOF)
	if err != nil {
		fail(rng, err)
		return mkf.ExitCodeErr
	}
	if err = cmf.Execute(ctx); err != nil {
		return failCode(rng, err)
	}
	return 0
}

// complete returns the space-joined bash command-line completion suggestions
// for the targets, given the completion args. It returns an empty string when
// args do not describe a completion request or the word is already completed.
func complete(args []string, ts []*mkf.Target) string {
	// Get the current partial word to be completed
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
