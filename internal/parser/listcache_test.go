// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_listCacheKey(t *testing.T) {
	t.Run("stable and cacheable inside a module", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		dir := modkit.Path("testdata/projects/simple_tagged/project")

		// --- When ---
		hKey1, hOk1 := listCacheKey(rng, dir, "example.com/x")
		hKey2, hOk2 := listCacheKey(rng, dir, "example.com/x")

		// --- Then ---
		assert.True(t, hOk1)
		assert.True(t, hOk2)
		assert.Equal(t, hKey1, hKey2)
	})

	t.Run("different spec", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		dir := modkit.Path("testdata/projects/simple_tagged/project")

		// --- When ---
		hKey1, _ := listCacheKey(rng, dir, "example.com/a")
		hKey2, _ := listCacheKey(rng, dir, "example.com/b")

		// --- Then ---
		assert.NotEqual(t, hKey1, hKey2)
	})

	t.Run("changes when ring CGO_ENABLED changes", func(t *testing.T) {
		// --- Given ---
		dir := modkit.Path("testdata/projects/simple_tagged/project")

		base := SetBuildTag(ring.New())
		base.EnvSet("CGO_ENABLED", "0")

		alt := SetBuildTag(ring.New())
		alt.EnvSet("CGO_ENABLED", "1")

		// --- When ---
		hKey1, hOk1 := listCacheKey(base, dir, "example.com/x")
		hKey2, hOk2 := listCacheKey(alt, dir, "example.com/x")

		// --- Then ---
		assert.True(t, hOk1)
		assert.True(t, hOk2)
		assert.NotEqual(t, hKey1, hKey2)
	})

	t.Run("unreadable go.mod is not cached", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		dir := t.TempDir()
		oskit.MkdirAll(t, dir, "go.mod")

		// --- When ---
		hKey, hOk := listCacheKey(rng, dir, "example.com/x")

		// --- Then ---
		assert.False(t, hOk)
		assert.Equal(t, "", hKey)
	})

	t.Run("unreadable go.sum is not cached", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		dir := t.TempDir()
		oskit.Write(t, "module example.com/m\n", dir, "go.mod")
		oskit.MkdirAll(t, dir, "go.sum")

		// --- When ---
		hKey, hOk := listCacheKey(rng, dir, "example.com/x")

		// --- Then ---
		assert.False(t, hOk)
		assert.Equal(t, "", hKey)
	})

	t.Run("not cacheable outside a module", func(t *testing.T) {
		// --- When ---
		hKey, hOk := listCacheKey(ring.New(), t.TempDir(), "example.com/x")

		// --- Then ---
		assert.False(t, hOk)
		assert.Equal(t, "", hKey)
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
		have, hOk := loadListCache(key)

		// --- Then ---
		assert.True(t, hOk)
		assert.Equal(t, want, have)
	})

	t.Run("load missing entry", func(t *testing.T) {
		// --- Given ---
		t.Setenv("XDG_CACHE_HOME", t.TempDir())

		// --- When ---
		have, hOk := loadListCache("missing")

		// --- Then ---
		assert.False(t, hOk)
		assert.Nil(t, have)
	})
}
