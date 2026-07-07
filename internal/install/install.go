// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package install builds and installs the gomake binary. [Main] is the
// entrypoint used by cmd/install. It relies only on the Go toolchain and
// never shells out to a VCS such as git. Build metadata is read and stored
// separately by the [version] package.
//
// [version]: github.com/ctx42/gomake/internal/version
package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ctx42/ring/pkg/ring"
)

const gomakeBinName = "gomake" // Installed binary name.

// Build creates dst (parents included) if absent, then compiles cmd/gomake
// from src into dst/<binary-name>. The caller resolves the destination via
// [cli.GoBinPath]. MkdirAll matches `go install`: a missing bin directory is
// created, and a path component that is a file surfaces a clear error. The env
// supplies the build subprocess environment.
func Build(env ring.Environ, src, dst, ldflags string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("create %q: %w", dst, err)
	}
	name := gomakeBinName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	pkg := "./cmd/" + gomakeBinName
	return buildMain(env, src, filepath.Join(dst, name), pkg, ldflags)
}

// buildMain runs `go build -o out` for mainPkg from dir.
//
// We use `go build` because if someone built with `go get`, then `go install`
// turns into a no-op, and `go install -a` fails on machines where go is
// installed in a non-writeable directory (such as normal OS installs in
// /usr/bin).
func buildMain(env ring.Environ, dir, out, mainPkg, ldflags string) error {
	args := []string{"build", "-o", out}
	if strings.TrimSpace(ldflags) != "" {
		args = append(args, "-ldflags", ldflags)
	}
	args = append(args, mainPkg)
	cmd := exec.Command("go", args...)
	cmd.Env = env.EnvAll()
	cmd.Dir = dir
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(outBytes)))
	}
	return nil
}
