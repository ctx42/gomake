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
		home, err := homeDir(rng)
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
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(scriptPath, bashCompleteScript, 0o600); err != nil {
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
	src := "\n# gomake completion\nsource %s\n"
	_, err = fmt.Fprintf(f, src, scriptPath)
	if cerr := f.Close(); err == nil {
		err = cerr // A failed close can mean the line never reached disk.
	}
	if err != nil {
		return "", fmt.Errorf("update %s: %w", rcPath, err)
	}

	format := "" +
		"Gomake bash completion installed.\n\n" +
		"  Script:     %s\n" +
		"  Configured: %s\n\n" +
		"To activate in the current shell:\n" +
		"  source %s\n"
	return fmt.Sprintf(format, scriptPath, rcPath, rcPath), nil
}
