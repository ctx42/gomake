// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ctx42/ring/pkg/ring"
)

// GoBinPath returns the absolute path to the directory where `go install`
// places binaries. This is $GOBIN when set and non-empty, otherwise the first
// entry of $GOPATH joined with "/bin".
//
// It runs `go env` with env as the subprocess environment, so an overridden
// GOBIN or GOPATH (for example, in tests) is honored exactly as a user's
// exported value would be.
//
// Do not "simplify" this to a direct env.EnvLookup of GOBIN or GOPATH. Those
// are Go toolchain settings, not plain environment variables: their effective
// values come from three sources, highest first: an environment variable, the
// `go env -w` config file, then a computed default (GOPATH defaults to
// $HOME/go, and an empty GOBIN means "use GOPATH/bin"). An Environ carries
// only the first source, so a direct read misses the config file and the
// defaults and resolves the wrong directory on machines that do not export
// them. Let `go env` merge all three; feed it env, never replace it.
//
// It returns an error if GOBIN is empty and GOPATH is empty or unset.
func GoBinPath(env ring.Environ) (string, error) {
	cmd := exec.Command("go", "env", "-json", "GOBIN", "GOPATH")
	cmd.Env = env.EnvAll()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cannot determine GOBIN/GOPATH: %w", err)
	}
	return goBinPath(string(out))
}

// goBinPath derives the installation directory from the JSON output of
// `go env -json GOBIN GOPATH`. It is the pure, testable core of [GoBinPath].
func goBinPath(envJSON string) (string, error) {
	var values struct {
		GOBIN  string `json:"GOBIN"`
		GOPATH string `json:"GOPATH"`
	}
	if err := json.Unmarshal([]byte(envJSON), &values); err != nil {
		return "", fmt.Errorf("cannot parse 'go env' output: %w", err)
	}

	pth := strings.TrimSpace(values.GOBIN)
	if pth == "" && values.GOPATH != "" {
		for _, entry := range filepath.SplitList(values.GOPATH) {
			entry = strings.TrimSpace(entry)
			if entry != "" {
				pth = filepath.Join(entry, "bin")
				break
			}
		}
	}
	if pth == "" {
		msg := "cannot determine bin directory: GOBIN and GOPATH unusable"
		return "", errors.New(msg)
	}
	abs, err := filepath.Abs(pth)
	if err != nil {
		return "", fmt.Errorf("cannot resolve bin path %q: %w", pth, err)
	}
	return abs, nil
}
