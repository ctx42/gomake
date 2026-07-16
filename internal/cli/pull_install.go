// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ctx42/ring/pkg/ring"
)

// envKeyInstallPath overrides [installPath] (tests only).
const envKeyInstallPath = "GOMAKE_INSTALL_PATH"

// installPath returns the absolute path to the running gomake binary. The env
// supplies the environment for the [GoBinPath] fallback.
func installPath(env ring.Environ) (string, error) {
	if p, ok := env.EnvLookup(envKeyInstallPath); ok {
		if p = strings.TrimSpace(p); p != "" {
			return filepath.Abs(p)
		}
	}
	execPath, err := os.Executable()
	if err != nil {
		return installPathFallback(env)
	}
	resolved, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return installPathFallback(env)
	}
	return filepath.Abs(resolved)
}

// installPathFallback returns the expected binary path under GOBIN when the
// running executable path cannot be resolved via [os.Executable]. The env
// supplies the environment for [GoBinPath].
func installPathFallback(env ring.Environ) (string, error) {
	gobin, err := GoBinPath(env)
	if err != nil {
		return "", err
	}
	name := binName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(gobin, name), nil
}

// replaceInstall atomically installs built binary at installPath.
func replaceInstall(built, installPath string) error {
	tmp := installPath + ".tmp"
	if err := copyFile(built, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, installPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// copyFile copies src to dst, creating or truncating dst with mode 0700.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0700)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
