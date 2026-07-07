// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
)

func Test_TargetConfig(t *testing.T) {
	t.Run("decodes json block", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.MetaSet(ConfigMetaKey, `{"region":"eu","count":3}`)
		var have struct {
			Region string `json:"region"`
			Count  int    `json:"count"`
		}

		// --- When ---
		err := TargetConfig(rng, &have)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "eu", have.Region)
		assert.Equal(t, 3, have.Count)
	})

	t.Run("no config", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		have := struct {
			Region string `json:"region"`
		}{Region: "keep"}

		// --- When ---
		err := TargetConfig(rng, &have)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "keep", have.Region)
	})

	t.Run("non-string meta value", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.MetaSet(ConfigMetaKey, 123)
		var have map[string]any

		// --- When ---
		err := TargetConfig(rng, &have)

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, have)
	})

	t.Run("error - invalid json", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.MetaSet(ConfigMetaKey, "{not json")
		var have map[string]any

		// --- When ---
		err := TargetConfig(rng, &have)

		// --- Then ---
		assert.Error(t, err)
	})
}
