// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/modkit"
)

func Test_listCacheKey(t *testing.T) {
	t.Run("stable and cacheable inside a module", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		dir := modkit.Path("testdata/projects/simple_tagged/project")

		// --- When ---
		key1, ok1 := listCacheKey(rng, dir, "example.com/x")
		key2, ok2 := listCacheKey(rng, dir, "example.com/x")

		// --- Then ---
		assert.True(t, ok1)
		assert.True(t, ok2)
		assert.Equal(t, key1, key2)
	})

	t.Run("different spec yields different key", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		dir := modkit.Path("testdata/projects/simple_tagged/project")

		// --- When ---
		key1, _ := listCacheKey(rng, dir, "example.com/a")
		key2, _ := listCacheKey(rng, dir, "example.com/b")

		// --- Then ---
		assert.NotEqual(t, key1, key2)
	})

	t.Run("not cacheable outside a module", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		dir := t.TempDir()

		// --- When ---
		key, ok := listCacheKey(rng, dir, "example.com/x")

		// --- Then ---
		assert.False(t, ok)
		assert.Equal(t, "", key)
	})
}

func Test_cacheableModule_tabular(t *testing.T) {
	tt := []struct {
		testN string

		mod  module
		want bool
	}{
		{"versioned external module", module{Version: "v1.2.3"}, true},
		{"main module has no version", module{Version: ""}, false},
		{
			"versioned but replaced module",
			module{Version: "v1.2.3", Replace: &module{}},
			false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := cacheableModule(tc.mod)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_listCache_roundtrip(t *testing.T) {
	t.Run("store then load", func(t *testing.T) {
		// --- Given ---
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		key := "deadbeef"
		want := []byte(`{"Name":"x"}`)

		// --- When ---
		storeListCache(key, want)
		have, ok := loadListCache(key)

		// --- Then ---
		assert.True(t, ok)
		assert.Equal(t, want, have)
	})

	t.Run("load missing entry", func(t *testing.T) {
		// --- Given ---
		t.Setenv("XDG_CACHE_HOME", t.TempDir())

		// --- When ---
		have, ok := loadListCache("missing")

		// --- Then ---
		assert.False(t, ok)
		assert.Nil(t, have)
	})
}
