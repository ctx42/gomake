// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package clitest

import (
	"bytes"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/ctx42/testing/pkg/tester"
)

// TestEnv returns an environment with the minimum number of variables.
//
// GOCACHE and GOENV are derived from `go env`, and XDG_CONFIG_HOME is set to
// an isolated empty temp directory so user-level gomake.yaml resolution never
// reads the developer's real $HOME/.config/gomake/gomake.yaml. GOENV keeps
// the `go env -w` settings, which would otherwise move with XDG_CONFIG_HOME.
//
// Variables copied from the current environment, when they are set:
//   - GOROOT
//   - GO111MODULE
//   - GOPATH
//   - GOMODCACHE
//   - GOPROXY, GOPRIVATE, GONOPROXY, GONOSUMDB, GOINSECURE
//   - SHELL
//   - PATH
//   - HOME
//   - USER
//   - TERM
//   - SSH_AUTH_SOCK
func TestEnv(t tester.T) []string {
	t.Helper()

	env := make([]string, 0, 10)

	// Point user config at an empty dir; HOME stays real for git, but a real
	// $HOME/.config/gomake/gomake.yaml must not leak into config resolution.
	env = append(
		env,
		"GOCACHE="+goEnv(t, "GOCACHE"),
		"GOENV="+goEnv(t, "GOENV"),
		"XDG_CONFIG_HOME="+t.TempDir(),
	)

	env = fromEnv("GOROOT", env)
	env = fromEnv("GO111MODULE", env)
	env = fromEnv("GOPATH", env)
	env = fromEnv("GOMODCACHE", env)
	env = fromEnv("GOPROXY", env)
	env = fromEnv("GOPRIVATE", env)
	env = fromEnv("GONOPROXY", env)
	env = fromEnv("GONOSUMDB", env)
	env = fromEnv("GOINSECURE", env)
	env = fromEnv("SHELL", env)
	env = fromEnv("PATH", env)
	env = fromEnv("HOME", env)
	env = fromEnv("USER", env)
	env = fromEnv("TERM", env)
	env = fromEnv("SSH_AUTH_SOCK", env) // Used by git command.
	return env
}

// fromEnv appends the variable named key from the real environment to env,
// which is the point of the snapshot: it reads the process environment on
// purpose. The variable is appended only if it is set.
func fromEnv(key string, env []string) []string {
	if val, set := os.LookupEnv(key); set {
		return append(env, key+"="+val)
	}
	return env
}

// goEnv returns the value of the Go environment variable key as reported by
// "go env" run with the real environment. A non-empty stderr fails the test
// so a warning cannot become part of the value.
func goEnv(t tester.T, key string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(t.Context(), "go", "env", key)
	cmd.Env = os.Environ()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
		return ""
	}
	if stderr.Len() > 0 {
		t.Fatal(stderr.String())
		return ""
	}
	return strings.TrimSpace(stdout.String())
}

// findMakefiles returns the base names of "makefile*.go" files in dir only
// (non-recursive). Nested directories are skipped so MakefilesFrom can join
// each name with the source root safely.
func findMakefiles(t tester.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
		return nil
	}
	var list []string
	for _, d := range entries {
		if d.IsDir() {
			continue
		}
		name := d.Name()
		if strings.HasPrefix(name, "makefile") && filepath.Ext(name) == ".go" {
			list = append(list, name)
		}
	}
	return list
}

// JoinImpSpec joins import path elements with slashes, as [path.Join] does;
// nothing is escaped, since an import path is not a URL.
func JoinImpSpec(t tester.T, base string, elem ...string) string {
	t.Helper()
	return path.Join(append([]string{base}, elem...)...)
}
