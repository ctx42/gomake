// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_Root(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		// --- When ---
		have, err := Root(".")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, must.Value(filepath.Abs("../..")), have)
	})

	t.Run("error - project root not found", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()

		// --- When ---
		have, err := Root(dir)

		// --- Then ---
		assert.ErrorIs(t, ErrNoGoMod, err)
		assert.ErrorRegexp(t, "^gomake: root: .* starting at "+dir, err)
		assert.Equal(t, "", have)
	})

	t.Run("error - go.mod is a directory", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.MkdirAll(t, dir, "go.mod")

		// --- When ---
		have, err := Root(dir)

		// --- Then ---
		assert.ErrorIs(t, ErrNoGoMod, err)
		assert.Equal(t, "", have)
	})

	t.Run("path", func(t *testing.T) {
		// --- When ---
		have, err := Root(".", "internal", "vtst")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, must.Value(filepath.Abs("../../internal/vtst")), have)
	})
}
