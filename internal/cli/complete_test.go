// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_runComplete(t *testing.T) {
	t.Run("unsupported shell returns a message", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("SHELL", "/bin/zsh")

		// --- When ---
		have, err := runComplete(rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "gomake --complete: no completion script " +
			"for \"/bin/zsh\" (supported: bash)\n"
		assert.Equal(t, want, have)
	})

	t.Run("bash shell installs completion and returns a message",
		func(t *testing.T) {
			// --- Given ---
			// Process HOME is a different directory, so a read of the
			// process environment cannot satisfy the assertion.
			procHome := t.TempDir()
			t.Setenv("HOME", procHome)
			t.Setenv("USERPROFILE", procHome)

			home := t.TempDir()

			rng := ring.New()
			rng.EnvSet("SHELL", "/bin/bash")
			rng.EnvSet("HOME", home)
			rng.EnvSet("USERPROFILE", home)
			rng.EnvSet("home", home)

			// --- When ---
			have, err := runComplete(rng)

			// --- Then ---
			assert.NoError(t, err)
			script := filepath.Join(home, ".bash_completion.d", "gomake")
			assert.FileExist(t, script)
			assert.Contain(t, "Gomake bash completion installed.", have)
		})

	t.Run("error - home unset", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("SHELL", "/bin/bash")
		rng.EnvSet("HOME", "")
		rng.EnvSet("USERPROFILE", "")
		rng.EnvSet("home", "")

		// --- When ---
		_, err := runComplete(rng)

		// --- Then ---
		assert.ErrorContain(t, "is not defined", err)
	})
}

func Test_setupBashCompletion(t *testing.T) {
	t.Run("first time setup", func(t *testing.T) {
		// --- Given ---
		home := t.TempDir()

		// --- When ---
		have, err := setupBashCompletion(home)

		// --- Then ---
		assert.NoError(t, err)

		scriptPath := filepath.Join(home, ".bash_completion.d", "gomake")
		assert.FileExist(t, scriptPath)

		data := oskit.ReadFileStr(t, scriptPath)
		assert.Equal(t, string(bashCompleteScript), data)

		rcPath := filepath.Join(home, ".bashrc")
		assert.FileExist(t, rcPath)

		rc := oskit.ReadFileStr(t, rcPath)
		assert.Contain(t, scriptPath, rc)
		assert.Contain(t, "# gomake completion", rc)
		assert.Contain(t, "Gomake bash completion installed.", have)
		assert.Contain(t, scriptPath, have)
		assert.Contain(t, "source "+rcPath, have)
	})

	t.Run("already configured", func(t *testing.T) {
		// --- Given ---
		home := t.TempDir()

		// Pre-configure: write script and put source line in .bashrc.
		dir := oskit.MkdirAll(t, home, ".bash_completion.d")
		scriptPath := oskit.Write(t, bashCompleteScript, dir, "gomake")
		content := "# gomake completion\nsource " + scriptPath + "\n"
		rcPath := oskit.Write(t, content, home, ".bashrc")

		// --- When ---
		have, err := setupBashCompletion(home)

		// --- Then ---
		assert.NoError(t, err)

		// .bashrc must not have a second source line added.
		rc := oskit.ReadFileStr(t, rcPath)
		assert.Equal(t, 1, strings.Count(rc, scriptPath))
		assert.Contain(t, "already configured", have)
		assert.Contain(t, "source "+rcPath, have)
	})

	t.Run("script updated even when already configured", func(t *testing.T) {
		// --- Given ---
		home := t.TempDir()

		dir := oskit.MkdirAll(t, home, ".bash_completion.d")
		scriptPath := oskit.Write(t, "old content", dir, "gomake")
		oskit.Write(t, "source "+scriptPath+"\n", home, ".bashrc")

		// --- When ---
		_, err := setupBashCompletion(home)

		// --- Then ---
		assert.NoError(t, err)
		data := oskit.ReadFileStr(t, scriptPath)
		assert.Equal(t, string(bashCompleteScript), data)
	})
}
