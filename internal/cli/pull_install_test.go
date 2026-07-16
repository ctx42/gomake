// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_installPath_envOverride(t *testing.T) {
	// --- Given ---
	want := filepath.Join(t.TempDir(), "gomake")
	oskit.Write(t, "x", want)
	rng := ring.New()
	rng.EnvSet(envKeyInstallPath, want)

	// --- When ---
	have, err := installPath(rng)

	// --- Then ---
	assert.NoError(t, err)
	assert.Equal(t, want, have)
}

func Test_installPath_executableLookup(t *testing.T) {
	// --- Given ---
	rng := ring.New()
	// Ensure the override key is absent so Executable() path is used.
	rng.EnvSet(envKeyInstallPath, "")

	// --- When ---
	have, err := installPath(rng)

	// --- Then ---
	assert.NoError(t, err)
	assert.NotEmpty(t, have)
}

func Test_installPathFallback(t *testing.T) {
	// --- When ---
	have, err := installPathFallback(ring.New())

	// --- Then ---
	assert.NoError(t, err)
	assert.NotEmpty(t, have)
	assert.Contain(t, binName, filepath.Base(have))
}

func Test_copyFile(t *testing.T) {
	// --- Given ---
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	oskit.Write(t, "data", src)

	// --- When ---
	err := copyFile(src, dst)

	// --- Then ---
	assert.NoError(t, err)
	data := oskit.ReadFileStr(t, dst)
	assert.Equal(t, "data", data)
}

func Test_copyFile_error_tabular(t *testing.T) {
	tt := []struct {
		testN string

		src string
	}{
		{
			"src missing",
			"missing",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			err := copyFile(
				filepath.Join(t.TempDir(), tc.src),
				filepath.Join(t.TempDir(), "dst"),
			)

			// --- Then ---
			assert.Error(t, err)
		})
	}
}

func Test_replaceInstall(t *testing.T) {
	// --- Given ---
	dir := t.TempDir()
	built := filepath.Join(dir, "built")
	installBin := filepath.Join(dir, "gomake")
	oskit.Write(t, "new-binary", built)
	oskit.Write(t, "old", installBin)

	// --- When ---
	err := replaceInstall(built, installBin)

	// --- Then ---
	assert.NoError(t, err)
	data := oskit.ReadFileStr(t, installBin)
	assert.Equal(t, "new-binary", data)
}

func Test_replaceInstall_error_tabular(t *testing.T) {
	tt := []struct {
		testN string

		setup func(t *testing.T, dir string) (built, installPath string)
	}{
		{
			"built binary missing",
			func(t *testing.T, dir string) (string, string) {
				return filepath.Join(dir, "missing-built"),
					filepath.Join(dir, "gomake")
			},
		},
		{
			"rename into unreadable dir",
			func(t *testing.T, dir string) (string, string) {
				built := filepath.Join(dir, "built")
				installPath := filepath.Join(dir, "sub", "gomake")
				oskit.Write(t, "x", built)
				assert.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0500))
				return built, installPath
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			dir := t.TempDir()
			built, installPath := tc.setup(t, dir)

			// --- When ---
			err := replaceInstall(built, installPath)

			// --- Then ---
			assert.Error(t, err)
		})
	}
}
