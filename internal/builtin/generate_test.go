// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package builtin

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_generateMain(t *testing.T) {
	t.Run("error - getwd", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		root := t.TempDir()
		wd := oskit.MkdirAll(t, root, "gone")
		bin := filepath.Join(root, "genmain")

		build := exec.CommandContext(ctx,
			"go",
			"build",
			"-o",
			bin,
			"00_generate_main.go",
		)
		bout, berr := build.CombinedOutput()
		assert.NoError(t, berr, string(bout))

		// Removing the working directory makes Getwd fail before any
		// write.
		cmd := exec.CommandContext(ctx,
			"/bin/sh",
			"-c",
			`cd "$1" && rmdir "$1" && exec "$2"`,
			"sh",
			wd,
			bin,
		)

		// --- When ---
		out, err := cmd.CombinedOutput()

		// --- Then ---
		assert.Error(t, err)
		assert.Contain(t, "getwd:", string(out))
	})
}
