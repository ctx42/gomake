// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package install

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_Build(t *testing.T) {
	const mainSrc = "package main\n\nfunc main() {}\n"
	const goMod = "module example.test\n\ngo 1.24\n"

	setup := func(t *testing.T) (root, cmdDir string) {
		t.Helper()
		root = t.TempDir()
		oskit.Write(t, goMod, root, "go.mod")
		cmdDir = oskit.MkdirAll(t, root, "cmd", gomakeBinName)
		return root, cmdDir
	}

	t.Run("creates dstDir and builds cmd/gomake", func(t *testing.T) {
		// --- Given ---
		env := ring.New()
		root, cmdDir := setup(t)
		oskit.Write(t, mainSrc, cmdDir, "main.go")
		dst := filepath.Join(t.TempDir(), "out")

		// --- When ---
		err := Build(env, root, dst, "")

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, filepath.Join(dst, gomakeBinName))
	})

	t.Run("builds into an existing dstDir", func(t *testing.T) {
		// --- Given ---
		env := ring.New()
		root, cmdDir := setup(t)
		oskit.Write(t, mainSrc, cmdDir, "main.go")
		dst := t.TempDir() // already exists

		// --- When ---
		err := Build(env, root, dst, "")

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, filepath.Join(dst, gomakeBinName))
	})

	t.Run("error - cmd/gomake is missing", func(t *testing.T) {
		// --- Given ---
		env := ring.New()
		root, _ := setup(t)
		dst := t.TempDir()

		// --- When ---
		err := Build(env, root, dst, "")

		// --- Then ---
		var ee *exec.ExitError
		assert.ErrorAs(t, &ee, err)
	})
}

func Test_buildMain(t *testing.T) {
	const src = "package main\n\nfunc main() {}\n"

	t.Run("builds a main package", func(t *testing.T) {
		// --- Given ---
		env := ring.New()
		dir := t.TempDir()
		out := filepath.Join(dir, "bin")
		mainGo := oskit.Write(t, src, dir, "main.go")

		// --- When ---
		err := buildMain(env, dir, out, mainGo, "")

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, out)
	})

	t.Run("passes ldflags", func(t *testing.T) {
		// --- Given ---
		env := ring.New()
		dir := t.TempDir()
		out := filepath.Join(dir, "bin")
		mainGo := oskit.Write(t, src, dir, "main.go")

		// --- When ---
		err := buildMain(env, dir, out, mainGo, "-s -w")

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, out)
	})

	t.Run("error - build fails", func(t *testing.T) {
		// --- Given ---
		env := ring.New()
		dir := t.TempDir()
		out := filepath.Join(dir, "bin")
		missing := filepath.Join(dir, "does-not-exist.go")

		// --- When ---
		err := buildMain(env, dir, out, missing, "")

		// --- Then ---
		var ee *exec.ExitError
		assert.ErrorAs(t, &ee, err)
		assert.NoFileExist(t, out)
	})
}
