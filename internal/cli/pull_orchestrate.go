// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/internal/builtin"
	"github.com/ctx42/gomake/internal/parser"
)

// PrepareTargets runs PrepareExternalTargets, then loads the resulting config
// and announces each external target import to rng.Stderr(). It is the single
// call used by install and build scripts to prepare and announce external
// target imports. Imports under any of skipMods are in modules a Go workspace
// already provides from disk, so their `go get` is skipped.
func PrepareTargets(
	ctx context.Context,
	rng *ring.Ring,
	wd string,
	skipMods []string,
) error {

	if err := prepareExternalTargets(ctx, rng, wd, skipMods); err != nil {
		return err
	}
	cfg, err := LoadExternalTargets(ctx, rng, filepath.Join(wd, TargetsFile))
	if err != nil {
		return err
	}
	for _, line := range cfg.importLines() {
		_, _ = fmt.Fprintln(rng.Stderr(), line)
	}
	return nil
}

// prepareExternalTargets reads `wd/targets.yaml`, runs go get for each
// listed import, and regenerates `internal/builtin/targets.go`. It's called by
// install and build scripts before compiling the binary. Imports under any of
// skipMods are provided by a Go workspace and their `go get` is skipped.
func prepareExternalTargets(
	ctx context.Context,
	rng *ring.Ring,
	wd string,
	skipMods []string,
) error {

	cfg, err := LoadExternalTargets(ctx, rng, filepath.Join(wd, TargetsFile))
	if err != nil {
		return err
	}
	for _, ent := range cfg.imports {
		inWorkspace := func(mod string) bool {
			return underModule(ent.Path, mod)
		}
		if slices.ContainsFunc(skipMods, inWorkspace) {
			continue
		}
		if err = runGoInDir(ctx, rng, wd, "get", ent.Path); err != nil {
			getErr := fmt.Errorf("go get %s: %w", ent.Path, err)
			if terr := runGoInDir(ctx, rng, wd, "mod", "tidy"); terr != nil {
				return fmt.Errorf("%w; go mod tidy: %w", getErr, terr)
			}
			return getErr
		}
	}
	return regenBuiltins(rng, wd, cfg)
}

// underModule reports whether the import path belongs to module mod: either
// the package is the module itself or lives beneath it. A version suffix on
// the import path is ignored. An empty mod matches nothing.
func underModule(importPath, mod string) bool {
	if mod == "" {
		return false
	}
	if i := strings.Index(importPath, "@"); i >= 0 {
		importPath = importPath[:i]
	}
	return importPath == mod || strings.HasPrefix(importPath, mod+"/")
}

// regenBuiltins runs builtin.GenImports to regenerate
// internal/builtin/targets.go, carrying each import's namespace.
func regenBuiltins(rng *ring.Ring, wd string, cfg *ImportsConfig) error {
	imports := make([]parser.Import, 0, len(cfg.imports))
	for _, ent := range cfg.imports {
		imports = append(imports, parser.Import{
			Path:      ent.Path,
			Namespace: ent.Namespace,
		})
	}
	err := builtin.GenImports(
		imports,
		builtin.WithGenDst(filepath.Join(wd, "internal", "builtin")),
		builtin.WithGenWorkDir(wd),
		builtin.WithGenEnv(rng),
		builtin.WithoutGenEmptySrc,
	)
	if err != nil {
		return fmt.Errorf("codegen builtins: %w", err)
	}
	return nil
}

// runGoInDir executes a go subcommand in dir, capturing stderr on error. The
// ctx cancels the subcommand.
func runGoInDir(
	ctx context.Context,
	env ring.Environ,
	dir string,
	args ...string,
) error {

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Env = env.EnvAll()
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
