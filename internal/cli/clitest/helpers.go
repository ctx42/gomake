// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package clitest

import (
	"bufio"
	"bytes"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ctx42/testing/pkg/tester"
)

// TestEnv returns environment with minimum number of variables.
//
// GOCACHE is derived from `go env`, and XDG_CONFIG_HOME is set to an isolated
// empty temp directory so user-level gomake.yaml resolution never reads the
// developer's real $HOME/.config/gomake/gomake.yaml.
//
// Variables set form the current environment (if they are set):
//   - GOROOT
//   - GO111MODULE
//   - GOPATH
//   - SHELL
//   - PATH
//   - HOME
//   - USER
//   - TERM
//   - SSH_AUTH_SOCK
func TestEnv(t tester.T) []string {
	t.Helper()

	env := make([]string, 0, 10)
	env = append(env, "GOCACHE="+goCache(t))

	// Point user config at an empty dir; HOME stays real for git, but a real
	// $HOME/.config/gomake/gomake.yaml must not leak into config resolution.
	env = append(env, "XDG_CONFIG_HOME="+t.TempDir())

	env = fromEnv("GOROOT", env)
	env = fromEnv("GO111MODULE", env)
	env = fromEnv("GOPATH", env)
	env = fromEnv("SHELL", env)
	env = fromEnv("PATH", env)
	env = fromEnv("HOME", env)
	env = fromEnv("USER", env)
	env = fromEnv("TERM", env)
	env = fromEnv("SSH_AUTH_SOCK", env) // Used by git command.
	return env
}

// fromEnv sets internal environment variable form real environment named key.
// The internal variable is set only if it is set in the real environment.
func fromEnv(key string, env []string) []string {
	if val, set := os.LookupEnv(key); set {
		return append(env, key+"="+val)
	}
	return env
}

// goCache returns path to go cache. Uses "go env" with os.Environ.
func goCache(t tester.T) string {
	t.Helper()
	out := &bytes.Buffer{}
	c := exec.Command("go", "env", "GOCACHE")
	c.Env = os.Environ()
	c.Stdout = out
	c.Stderr = out
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out.String())
}

// findMakefiles returns the base names of "makefile*.go" files in dir only
// (non-recursive). Nested directories are skipped so MakefilesFrom can join
// each name with the source root safely.
func findMakefiles(t tester.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Error(err)
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

// JoinImpSpec joins import spec elements.
func JoinImpSpec(t tester.T, base string, elem ...string) string {
	t.Helper()
	ret, err := url.JoinPath(base, elem...)
	if err != nil {
		t.Error(err)
	}
	return ret
}

// rowColValue extracts column value from a line which starts with the header.
// When a line is malformed, it marks the test as failed, returns an empty
// string and continues execution.
//
// For example, having text:
//
//	Name:       first
//	Last Name:  second
//
// and calling `rowColValue("Last Name:", 1, text)` returns "second" (the header
// must include the colon so Fields indexes align with data columns).
func rowColValue(t tester.T, header string, column int, text string) string {
	t.Helper()

	if column < 1 {
		t.Errorf("expected column to be positive, got: %d", column)
		return ""
	}

	scn := bufio.NewScanner(strings.NewReader(text))
	for scn.Scan() {
		line := strings.TrimSpace(scn.Text())
		if !strings.HasPrefix(line, header) {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, header))
		fields := strings.Fields(line)
		fc := len(fields)
		if fc == 0 {
			continue
		}
		if column >= fc+1 {
			t.Errorf("expected row to have at least %d fields", column)
			return ""
		}
		return strings.TrimSpace(fields[column-1])
	}
	if err := scn.Err(); err != nil {
		t.Error(err)
		return ""
	}
	t.Errorf("expected row with header %q to exist", header)
	return ""
}

// infoToEnv expects to get the output containing two columns which are then
// parsed and used in the map: first as a key, second as a value. It marks the
// test as failed if the output line has more than two columns or keys are
// repeating. Returns constructed map.
func infoToEnv(t tester.T, output string) map[string]string {
	t.Helper()
	m := make(map[string]string, 10)
	for lin := range strings.SplitSeq(output, "\n") {
		lin = strings.TrimSpace(lin)
		fls := strings.Fields(lin)
		if len(fls) == 2 {
			key := fls[0]
			if _, exists := m[key]; exists {
				t.Errorf("did not expect keys to repeat, key: %q", key)
			}
			m[key] = fls[1]
			continue
		}
		if len(fls) != 0 {
			t.Errorf("expected line to have two fields, got: %q", lin)
		}
	}
	return m
}
