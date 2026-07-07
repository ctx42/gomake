// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
)

func Test_GenMain(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		dst := filepath.Join(t.TempDir(), "gen.go")

		// --- When ---
		err := GenMain(dst)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, dst)
	})

	t.Run("failure", func(t *testing.T) {
		// --- Given ---
		dst := filepath.Join(t.TempDir(), "not_existing", "gen.go")

		// --- When ---
		err := GenMain(dst)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.NoFileExist(t, dst)
	})
}
