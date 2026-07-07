// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/internal/cli"
	"github.com/ctx42/gomake/internal/version"
)

// Main builds and installs the gomake binary into GOBIN, returning a non-nil
// error if any step fails. The destination is resolved via [cli.GoBinPath]:
// set GOBIN (or GOPATH) in the environment to control where the binary lands.
// The tgs argument may be a local path or a URL to a `targets.yaml` file; an
// empty string uses the source `targets.yaml`. The rng carries the environment
// and standard streams; info is the [debug.BuildInfo] embedded by the
// toolchain, which supplies both the version to record and the installation
// mode ("(devel)" for `go run ./cmd/install`, an actual version for
// `go run ...@version`).
func Main(rng *ring.Ring, info *debug.BuildInfo, tgs string) error {
	dst, err := cli.GoBinPath(rng)
	if err != nil {
		return fmt.Errorf("gomake: %w", err)
	}
	return installTo(rng, info, dst, tgs)
}

// installTo builds the gomake binary and installs it into dst. It is the
// internal indirection behind [Main]: [Main] resolves dst from GOBIN, while
// tests pass an explicit dst to avoid touching the real GOBIN.
//
// When the effective targets config has no imports, installTo builds straight
// from the read-only module source (the live working tree for a devel build):
// the shipped builtins already match an empty config, and `go build` only
// reads the source tree, so the temp copy, go get, and builtin regeneration
// are all skipped. With imports present it builds from a writable tree (a temp
// copy for a published build), writes the effective targets.yaml, regenerates
// the builtins, then compiles.
func installTo(rng *ring.Ring, info *debug.BuildInfo, dst, tgs string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("gomake: %w", err)
	}

	// Build metadata embedded by the Go toolchain gives both the version to
	// record and the installation mode: "(devel)" for `go run ./cmd/install`,
	// an actual version for `go run ...@version`.
	if info == nil {
		return errors.New("gomake: build info unavailable")
	}
	version.PopulateVersion(rng, info)

	// Resolve the read-only source: the live working tree for a devel build,
	// the module-cache directory for a published one.
	src := wd
	devel := info.Main.Version == "(devel)"
	if !devel {
		module := info.Main.Path + "@" + info.Main.Version
		if src, err = moduleCacheDir(rng, module); err != nil {
			return err
		}
	}

	// Resolve the effective imports: a --targets file (path or URL) overrides
	// the source targets.yaml.
	cfg, err := effectiveImports(src, tgs)
	if err != nil {
		return fmt.Errorf("gomake: %w", err)
	}

	ldflags := version.LDFlags()

	// Fast path: no external targets. Build directly from the read-only source
	// without copying, fetching, or regenerating builtins.
	if len(cfg.Imports()) == 0 {
		return Build(rng, src, dst, ldflags)
	}

	// Full path: external targets present. Build from a writable tree, write
	// the effective targets.yaml into it, then go get + regenerate builtins.
	buildDir := src
	if !devel {
		tmp, cleanup, cerr := copyToTemp(src)
		if cerr != nil {
			return cerr
		}
		defer cleanup()
		buildDir = tmp
	}
	if tgs != "" {
		out := filepath.Join(buildDir, cli.TargetsFile)
		if err = os.WriteFile(out, cfg.Raw(), 0o644); err != nil {
			return fmt.Errorf("gomake: write targets: %w", err)
		}
	}
	if err = cli.PrepareTargets(rng, buildDir); err != nil {
		return fmt.Errorf("gomake: %w", err)
	}
	return Build(rng, buildDir, dst, ldflags)
}

// effectiveImports resolves the imports the build should compile in. When tgs
// names a --targets file (a local path or an HTTP/HTTPS URL) it is loaded
// directly; otherwise the source targets.yaml in srcDir is used. A missing
// source file returns an empty config. A missing --targets file is an error.
func effectiveImports(srcDir, tgs string) (*cli.ImportsConfig, error) {
	pth := filepath.Join(srcDir, cli.TargetsFile)
	if tgs != "" {
		pth = tgs
	}
	cfg, err := cli.LoadExternalTargets(pth)
	if err != nil {
		return nil, err
	}
	if tgs != "" && cfg.Raw() == nil {
		return nil, fmt.Errorf("read targets %q: %w", tgs, os.ErrNotExist)
	}
	return cfg, nil
}

// moduleCacheDir runs `go mod download -json <module>` and returns the local
// cache directory for that module.
func moduleCacheDir(env ring.Environ, module string) (string, error) {
	args := []string{"mod", "download", "-json", module}
	cmd := exec.Command("go", args...)
	cmd.Env = env.EnvAll()
	out, err := cmd.Output()
	if err != nil {
		if e, ok := errors.AsType[*exec.ExitError](err); ok {
			msg := strings.TrimSpace(string(e.Stderr))
			return "", fmt.Errorf("module download %s: %w: %s", module, e, msg)
		}
		return "", fmt.Errorf("module download %s: %w", module, err)
	}
	info := struct {
		Dir string `json:"Dir"`
	}{}
	if err = json.Unmarshal(out, &info); err != nil {
		return "", fmt.Errorf("go mod download parse: %w", err)
	}
	if info.Dir == "" {
		return "", fmt.Errorf("go mod download %s: empty Dir", module)
	}
	return info.Dir, nil
}

// copyToTemp copies the read-only module source at srcDir into a fresh temp
// directory the caller owns, so go get and codegen can write into it. It
// returns the temp path and a cleanup the caller must defer. On error it
// removes any temp directory it created and returns a no-op cleanup.
func copyToTemp(src string) (string, func(), error) {
	noop := func() {}
	tempDir, err := os.MkdirTemp("", "gomake-install-*")
	if err != nil {
		return "", noop, err
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	if err = copyDir(src, tempDir); err != nil {
		cleanup()
		return "", noop, err
	}
	return tempDir, cleanup, nil
}

// copyDir recursively copies src into dst. Files are written with mode 0o644
// so that module-cache read-only permissions do not carry over.
func copyDir(src, dst string) error {
	fn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}
	return filepath.WalkDir(src, fn)
}
