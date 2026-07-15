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
//
// When tgs is a local path inside a Go module, that module is resolved from
// disk through a temporary Go workspace (see [setupWorkspace]) rather than
// fetched with go get, so an unpublished target module is compiled in and the
// build tree's go.mod is left untouched.
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

	// A devel build compiles in place. Regenerating the builtins would leave
	// the working tree with a targets.go importing external modules that only
	// resolve inside the temporary workspace, breaking the next `go build` or
	// `go run`. Snapshot those artifacts and restore them after the build; the
	// binary already carries the compiled-in targets.
	if devel {
		restore, serr := snapshotGenerated(buildDir)
		if serr != nil {
			return fmt.Errorf("gomake: %w", serr)
		}
		defer restore()
	}

	// Local --targets in a Go module: resolve that module from disk via a
	// temporary workspace so the build compiles in unpublished edits without
	// go get, keeping go.mod untouched. skipMod names the module whose imports
	// the workspace provides; its go get is skipped below.
	skipMod, cleanup, err := setupWorkspace(rng, buildDir, tgs)
	if err != nil {
		return fmt.Errorf("gomake: %w", err)
	}
	defer cleanup()

	if tgs != "" {
		out := filepath.Join(buildDir, cli.TargetsFile)
		if err = os.WriteFile(out, cfg.Raw(), 0o644); err != nil {
			return fmt.Errorf("gomake: write targets: %w", err)
		}
	}
	if err = cli.PrepareTargets(rng, buildDir, skipMod); err != nil {
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

// setupWorkspace enables disk resolution of a local --targets module. When tgs
// is a filesystem path whose directory lies inside a Go module, it creates a
// temporary Go workspace covering both buildDir and that module, points the
// build subprocess environment at it via GOWORK, and returns the module's
// import path so the caller can skip `go get` for its packages. Resolving from
// the workspace instead of the proxy keeps buildDir's go.mod untouched and
// compiles in unpublished local edits.
//
// It returns an empty module path and a no-op cleanup when tgs is empty, is a
// URL, or its directory is not inside a module: the caller then falls back to
// `go get`. The returned cleanup removes the workspace file and unsets GOWORK;
// it is always safe to call.
func setupWorkspace(env ring.Environ, buildDir, tgs string) (
	string,
	func(),
	error,
) {
	noop := func() {}
	if tgs == "" ||
		strings.HasPrefix(tgs, "http://") ||
		strings.HasPrefix(tgs, "https://") {
		return "", noop, nil
	}
	mod, root, ok := moduleAt(env, filepath.Dir(tgs))
	if !ok {
		return "", noop, nil
	}

	wsDir, err := os.MkdirTemp("", "gomake-work-*")
	if err != nil {
		return "", noop, err
	}
	cleanup := func() {
		env.EnvUnset("GOWORK")
		_ = os.RemoveAll(wsDir)
	}
	env.EnvSet("GOWORK", filepath.Join(wsDir, "go.work"))
	if err = goWorkInit(env, wsDir, buildDir, root); err != nil {
		cleanup()
		return "", noop, err
	}
	return mod, cleanup, nil
}

// moduleAt reports the Go module that dir belongs to, returning the module's
// import path and root directory. The ok result is false when dir is not
// inside a module or the toolchain reports no usable module, signalling the
// caller to fall back to `go get`.
func moduleAt(env ring.Environ, dir string) (mod, root string, ok bool) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Path}}::{{.Dir}}")
	cmd.Env = env.EnvAll()
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", "", false
	}
	mod, root, ok = strings.Cut(strings.TrimSpace(string(out)), "::")
	if !ok || mod == "" || root == "" {
		return "", "", false
	}
	return mod, root, true
}

// goWorkInit runs `go work init buildDir modRoot` in wsDir, writing the
// workspace file that lists both modules.
func goWorkInit(env ring.Environ, wsDir, buildDir, modRoot string) error {
	cmd := exec.Command("go", "work", "init", buildDir, modRoot)
	cmd.Env = env.EnvAll()
	cmd.Dir = wsDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		return fmt.Errorf("go work init: %w: %s", err, msg)
	}
	return nil
}

// snapshotGenerated records the current contents of the files a full in-source
// build overwrites — the effective targets.yaml, the two generated builtin
// sources, and the go.mod/go.sum that `go get` rewrites — and returns a
// function that restores them. Restoring undoes the regeneration so the working
// tree is left byte-identical and still compiles without the temporary
// workspace. Only files that exist at snapshot time are tracked; an absent path
// is left untouched (never created). In a real module all are committed, so the
// tree is fully restored. The returned function is meant to run via defer after
// the build.
func snapshotGenerated(buildDir string) (func(), error) {
	// These mirror cli.PrepareTargets's outputs: the effective targets config,
	// builtin.GenImports's two generated files, and the go.mod/go.sum that its
	// `go get` step rewrites to add the external target modules.
	paths := []string{
		filepath.Join(buildDir, cli.TargetsFile),
		filepath.Join(buildDir, "internal", "builtin", "targets.go"),
		filepath.Join(buildDir, "internal", "builtin", "data",
			"targets_main.go_"),
		filepath.Join(buildDir, "go.mod"),
		filepath.Join(buildDir, "go.sum"),
	}
	type snapshot struct {
		data []byte
		mode os.FileMode
	}
	saved := make(map[string]snapshot, len(paths))
	for _, pth := range paths {
		info, err := os.Stat(pth)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(pth)
		if err != nil {
			return nil, err
		}
		saved[pth] = snapshot{data: data, mode: info.Mode()}
	}
	return func() {
		for pth, snap := range saved {
			_ = os.WriteFile(pth, snap.data, snap.mode)
		}
	}, nil
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
