// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package cli implements the gomake CLI: configuration, out-of-source
// builds, makefile preparation, and execution of user-defined targets.
package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/internal/parser"
	"github.com/ctx42/gomake/pkg/gomake"
)

// binName represents name of the gomake binary.
const binName = "gomake"

// buildTagLine is the build tag line written at the top of tagged makefiles.
const buildTagLine = parser.BuildTagLine

// GoMake errors.
var (
	// errNoMakefile represents an error when project directory contains no
	// makefiles.
	errNoMakefile = errors.New("no makefile found")

	// errGoModEdit is an error returned when go.mod file editing fails.
	errGoModEdit = errors.New("editing \"go.mod\" file")

	// errGoWorkEdit is an error returned when go.work file editing fails.
	errGoWorkEdit = errors.New("editing \"go.work\" file")

	// errBinExists represents error returned when path to binary makefile
	// already exists.
	errBinExists = errors.New("path to makefile binary exists")
)

// goMake represents gomake command.
type goMake struct {
	cfg     *config         // Runtime configuration.
	cu      *compUnit       // Compilation unit.
	targets *parser.Targets // User targets.
}

// newGoMake returns new instance of [goMake].
func newGoMake(rng *ring.Ring, cfg *config) (gmk *goMake, err error) {
	gmk = &goMake{cfg: cfg}
	rng.EnvSet("GOOS", cfg.goos)
	rng.EnvSet("GOARCH", cfg.goarch)
	rng = parser.SetBuildTag(rng)

	// Bring to build directory user defined targets and empty built-in targets.
	gmk.cu, err = prepare(rng, cfg.tmp, cfg.src)
	if err != nil {
		return nil, err
	}

	buildDir := gmk.cu.BuildDir
	defer func() {
		if err != nil {
			_ = os.RemoveAll(buildDir)
		}
	}()

	// Build a synthetic Package for the build dir from the already-known file
	// list — avoids a second "go list" subprocess call. Name must be set so
	// targets get PkgName = "main" and are not included as external imports.
	buildPkg := &parser.Package{
		ImpPath: gmk.cu.BuildDir,
		Name:    gmk.cu.SrcPkg.Name,
		Files:   gmk.cu.MkfNames,
	}
	var pmf *parser.Makefile
	if pmf, err = parser.MakefileFromPackage(rng, buildPkg); err != nil {
		return nil, err
	}
	gen := parser.NewGenerator(pmf.Targets)
	var code []byte
	if code, err = gen.Generate(
		parser.WithGenNames("main", "User"),
		parser.WithGenReg,
	); err != nil {
		return nil, err
	}
	if err = parser.CreateFile(gmk.cu.MainUser, code); err != nil {
		return nil, err
	}
	gmk.targets = pmf.Targets

	// Generate glue code between user defined targets and built-in targets.
	if err = genMain(gmk.cu.MainGen, cfg.version); err != nil {
		return nil, err
	}
	return gmk, nil
}

// Compile compiles the makefile and returns the absolute path to the binary.
func (gmk *goMake) Compile(
	ctx context.Context,
	env []string,
	dst string,
) (string, error) {

	if err := compile(ctx, env, gmk.cu.BuildDir, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// Execute executes a target. The target is picked based on the arguments
// passed to the command. The compiled makefile binary runs as a subprocess in
// the configured working directory.
func (gmk *goMake) Execute(ctx context.Context, rng *ring.Ring) error {
	env := rng.EnvAll()

	gowork := rng.EnvGet("GOWORK")
	binPath, cached := lookupBinaryCache(
		gmk.cfg.src, gmk.cu.MkfNames,
		gmk.cfg.version, gmk.cfg.goos, gmk.cfg.goarch, gowork,
	)
	if !cached {
		var err error
		compileAct := func() error {
			_, err = gmk.Compile(ctx, env, gmk.cu.MainBin)
			return err
		}
		err = withProgress(rng.Stderr(), "Compiling makefile...", compileAct)
		if err != nil {
			return err
		}
		storeBinaryCache(
			gmk.cu.MainBin,
			gmk.cfg.src, gmk.cu.MkfNames, gmk.cfg.version,
			gmk.cfg.goos, gmk.cfg.goarch, gowork,
		)
		binPath = gmk.cu.MainBin
	}

	// Ferry the invoked target's configuration to the subprocess as an
	// internal argument. The generated main strips it and loads it into the
	// ring meta store; it never reaches the target as an argument, and the
	// environment is left untouched.
	args := gmk.cfg.args
	if cfgJSON, ok := rng.MetaLookup(gomake.ConfigMetaKey); ok {
		if text, ok := cfgJSON.(string); ok {
			args = append([]string{targetConfigArg + "=" + text}, args...)
		}
	}

	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Env = env
	cmd.Stdin = rng.Stdin()
	cmd.Stdout = rng.Stdout()
	cmd.Stderr = rng.Stderr()
	cmd.Dir = gmk.cfg.wd
	return cmd.Run()
}
