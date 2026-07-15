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
// target imports. Imports under skipMod (an empty string disables this) are a
// module a Go workspace already provides from disk, so their `go get` is
// skipped.
func PrepareTargets(rng *ring.Ring, wd, skipMod string) error {
	if err := prepareExternalTargets(rng, wd, skipMod); err != nil {
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
// install and build scripts before compiling the binary. Imports under skipMod
// are provided by a Go workspace and their `go get` is skipped.
func prepareExternalTargets(rng *ring.Ring, wd, skipMod string) error {
	cfg, err := LoadExternalTargets(filepath.Join(wd, TargetsFile))
	if err != nil {
		return err
	}
	for _, ent := range cfg.imports {
		if underModule(ent.Path, skipMod) {
			continue
		}
		if err = runGoInDir(rng, wd, "get", ent.Path); err != nil {
			_ = runGoInDir(rng, wd, "mod", "tidy")
			return fmt.Errorf("go get %s: %w", ent.Path, err)
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
