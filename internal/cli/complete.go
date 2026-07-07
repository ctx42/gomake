// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx42/ring/pkg/ring"
)

//go:embed data/gomake-bash-complete.sh
var bashCompleteScript []byte

// runComplete implements the --complete option. It returns the status message
// the entry point writes to standard error.
func runComplete(rng *ring.Ring) (string, error) {
	shell := rng.EnvGet("SHELL")
	switch {
	case strings.Contains(shell, "bash"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return setupBashCompletion(home)
	default:
		format := "gomake --complete: no completion script " +
			"for %q (supported: bash)\n"
		return fmt.Sprintf(format, shell), nil
	}
}

// setupBashCompletion installs the bash completion script in home and
// configures ~/.bashrc to source it, returning the status message. If
// completion is already configured it reports that and reminds the user how
// to activate it in the current shell.
func setupBashCompletion(home string) (string, error) {
	scriptPath := filepath.Join(home, ".bash_completion.d", "gomake")
	rcPath := filepath.Join(home, ".bashrc")

	// Always write the latest script.
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(scriptPath, bashCompleteScript, 0644); err != nil {
		return "", err
	}

	// Check whether ~/.bashrc already references our script.
	configured, err := fileContainsStr(rcPath, scriptPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if configured {
		format := "" +
			"Gomake bash completion is already configured.\n\n" +
			"To activate in the current shell:\n" +
			"  source %s\n"
		return fmt.Sprintf(format, rcPath), nil
	}

	// Add source line to ~/.bashrc.
	f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	src := "\n# gomake completion\nsource %s\n"
	if _, err = fmt.Fprintf(f, src, scriptPath); err != nil {
		return "", err
	}

	format := "" +
		"Gomake bash completion installed.\n\n" +
		"  Script:     %s\n" +
		"  Configured: %s\n\n" +
		"To activate in the current shell:\n" +
		"  source %s\n"
	return fmt.Sprintf(format, scriptPath, rcPath, rcPath), nil
}

// fileContainsStr reports whether the file at the path contains substr.
// Returns false (not true) when the file does not exist.
func fileContainsStr(path, substr string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), substr), nil
}
