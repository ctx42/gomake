// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
)

func Test_Root(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		// --- When ---
		have, err := Root(".")

		// --- Then ---
		assert.NoError(t, err)
		want := must.Value(filepath.Abs("../.."))
		assert.Equal(t, want, have)
	})

	t.Run("could not find project root", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()

		// --- When ---
		have, err := Root(dir)

		// --- Then ---
		assert.ErrorIs(t, ErrNoGoMod, err)
		assert.ErrorContain(t, dir, err)
		assert.Equal(t, "", have)
	})

	t.Run("path", func(t *testing.T) {
		// --- When ---
		have, err := Root(".", "internal", "vtst")

		// --- Then ---
		assert.NoError(t, err)
		want := must.Value(filepath.Abs("../../internal/vtst"))
		assert.Equal(t, want, have)
	})
}
