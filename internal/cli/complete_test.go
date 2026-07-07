// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
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
		msg, err := runComplete(rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "gomake --complete: no completion script " +
			"for \"/bin/zsh\" (supported: bash)\n"
		assert.Equal(t, want, msg)
	})

	t.Run("bash shell installs completion and returns a message",
		func(t *testing.T) {
			// --- Given ---
			home := t.TempDir()
			t.Setenv("HOME", home)
			rng := ring.New()
			rng.EnvSet("SHELL", "/bin/bash")

			// --- When ---
			msg, err := runComplete(rng)

			// --- Then ---
			assert.NoError(t, err)
			assert.Contain(t, "Gomake bash completion installed.", msg)
		})
}

func Test_setupBashCompletion(t *testing.T) {
	t.Run("first time setup", func(t *testing.T) {
		// --- Given ---
		home := t.TempDir()

		// --- When ---
		msg, err := setupBashCompletion(home)

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

		assert.Contain(t, "Gomake bash completion installed.", msg)
		assert.Contain(t, scriptPath, msg)
		assert.Contain(t, "source "+rcPath, msg)
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
		msg, err := setupBashCompletion(home)

		// --- Then ---
		assert.NoError(t, err)

		// .bashrc must not have a second source line added.
		rc := oskit.ReadFileStr(t, rcPath)
		assert.Equal(t, 1, strings.Count(rc, scriptPath))

		assert.Contain(t, "already configured", msg)
		assert.Contain(t, "source "+rcPath, msg)
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

func Test_fileContainsStr(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "f")
		oskit.Write(t, "foo bar baz", p)
		ok, err := fileContainsStr(p, "bar")
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("absent", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "f")
		oskit.Write(t, "foo baz", p)
		ok, err := fileContainsStr(p, "bar")
		assert.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("file does not exist", func(t *testing.T) {
		ok, err := fileContainsStr("/nonexistent/path", "x")
		assert.False(t, ok)
		assert.ErrorIs(t, os.ErrNotExist, err)
	})
}
