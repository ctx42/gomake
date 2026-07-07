// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/modkit"
)

func Test_newLocalPackage(t *testing.T) {
	t.Run("resolves a tagged package with build tag", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		dir := modkit.Path("testdata/projects/simple_tagged/project")
		pkg := &Package{ImpPath: dir}

		// --- When ---
		ok := newLocalPackage(rng, pkg)

		// --- Then ---
		assert.True(t, ok)
		assert.Equal(t, "main", pkg.Name)
		assert.Equal(t, []string{"makefile.go"}, pkg.Files)
		assert.Equal(t, "github.com/ctx42/gomake", pkg.Module.ImpSpec)
		want := pkg.Module.ImpSpec +
			"/testdata/projects/simple_tagged/project"
		assert.Equal(t, want, pkg.ImpSpec)
		assert.Equal(t, filepath.Dir(pkg.Module.ModPath), pkg.Module.ImpPath)
	})

	t.Run("resolves an untagged package", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		dir := modkit.Path("testdata/projects/simple_untagged/project")
		pkg := &Package{ImpPath: dir}

		// --- When ---
		ok := newLocalPackage(rng, pkg)

		// --- Then ---
		assert.True(t, ok)
		assert.Equal(t, "main", pkg.Name)
		assert.Equal(t, []string{"makefile.go"}, pkg.Files)
	})

	t.Run("tagged file excluded without build tag", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		dir := modkit.Path("testdata/projects/simple_tagged/project")
		pkg := &Package{ImpPath: dir}

		// --- When ---
		ok := newLocalPackage(rng, pkg)

		// --- Then ---
		assert.False(t, ok)
	})

	t.Run("no Go files falls back", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		dir := modkit.Path("testdata/projects/empty")
		pkg := &Package{ImpPath: dir}

		// --- When ---
		ok := newLocalPackage(rng, pkg)

		// --- Then ---
		assert.False(t, ok)
	})
}

func Test_importDir(t *testing.T) {
	t.Run("includes tagged files with build tag", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		dir := modkit.Path("testdata/projects/simple_tagged/project")

		// --- When ---
		bp, err := importDir(rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"makefile.go"}, bp.GoFiles)
	})

	t.Run("error - tagged files excluded without tag", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		dir := modkit.Path("testdata/projects/simple_tagged/project")

		// --- When ---
		_, err := importDir(rng, dir)

		// --- Then ---
		assert.Error(t, err)
	})
}

func Test_importSpec_tabular(t *testing.T) {
	tt := []struct {
		testN string

		modSpec string
		root    string
		dir     string
		want    string
	}{
		{
			testN:   "module root",
			modSpec: "example.com/mod",
			root:    "/src/mod",
			dir:     "/src/mod",
			want:    "example.com/mod",
		},
		{
			testN:   "sub package",
			modSpec: "example.com/mod",
			root:    "/src/mod",
			dir:     "/src/mod/a/b",
			want:    "example.com/mod/a/b",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := importSpec(tc.modSpec, tc.root, tc.dir)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_readModulePath(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		path := modkit.Path("go.mod")

		// --- When ---
		have, err := readModulePath(path)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "github.com/ctx42/gomake", have)
	})

	t.Run("error - missing file", func(t *testing.T) {
		// --- Given ---
		path := modkit.Path("testdata/projects/empty/go.mod")

		// --- When ---
		have, err := readModulePath(path)

		// --- Then ---
		assert.Error(t, err)
		assert.Equal(t, "", have)
	})
}
