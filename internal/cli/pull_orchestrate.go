// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/internal/builtin"
	"github.com/ctx42/gomake/internal/parser"
)

// PrepareTargets runs PrepareExternalTargets, then loads the resulting config
// and announces each external target import to rng.Stderr(). It is the single
// call used by install and build scripts to prepare and announce external
// target imports.
func PrepareTargets(rng *ring.Ring, wd string) error {
	if err := prepareExternalTargets(rng, wd); err != nil {
		return err
	}
	cfg, err := LoadExternalTargets(filepath.Join(wd, TargetsFile))
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
// install and build scripts before compiling the binary.
func prepareExternalTargets(rng *ring.Ring, wd string) error {
	cfg, err := LoadExternalTargets(filepath.Join(wd, TargetsFile))
	if err != nil {
		return err
	}
	for _, ent := range cfg.imports {
		if err = runGoInDir(rng, wd, "get", ent.Path); err != nil {
			_ = runGoInDir(rng, wd, "mod", "tidy")
			return fmt.Errorf("go get %s: %w", ent.Path, err)
		}
	}
	return regenBuiltins(rng, wd, cfg)
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
		builtin.WithGenEnv(rng),
		builtin.WithoutGenEmptySrc,
	)
	if err != nil {
		return fmt.Errorf("codegen builtins: %w", err)
	}
	return nil
}

// runGoInDir executes a go subcommand in dir, capturing stderr on error.
func runGoInDir(env ring.Environ, dir string, args ...string) error {
	cmd := exec.Command("go", args...)
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
