// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"go/build"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_newLocalPackage(t *testing.T) {
	t.Run("tagged package with build tag", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		dir := modkit.Path("testdata/projects/simple_tagged/project")
		pkg := &Package{ImpPath: dir}

		// --- When ---
		have := newLocalPackage(rng, pkg)

		// --- Then ---
		assert.True(t, have)
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
		have := newLocalPackage(rng, pkg)

		// --- Then ---
		assert.True(t, have)
		assert.Equal(t, "main", pkg.Name)
		assert.Equal(t, []string{"makefile.go"}, pkg.Files)
	})

	t.Run("tagged file excluded without build tag", func(t *testing.T) {
		// --- Given ---
		dir := modkit.Path("testdata/projects/simple_tagged/project")
		pkg := &Package{ImpPath: dir}

		// --- When ---
		have := newLocalPackage(ring.New(), pkg)

		// --- Then ---
		assert.False(t, have)
	})

	t.Run("no Go files falls back", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		dir := modkit.Path("testdata/projects/empty")
		pkg := &Package{ImpPath: dir}

		// --- When ---
		have := newLocalPackage(rng, pkg)

		// --- Then ---
		assert.False(t, have)
	})
}

func Test_importDir(t *testing.T) {
	t.Run("includes tagged files with build tag", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		dir := modkit.Path("testdata/projects/simple_tagged/project")

		// --- When ---
		have, err := importDir(rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"makefile.go"}, have.GoFiles)
	})

	t.Run("cgo disabled drops cgo files", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("CGO_ENABLED", "0")

		dir := importDirFixture(t)

		// --- When ---
		have, err := importDir(rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"old.go", "plain.go"}, sortedGoFiles(have))
	})

	t.Run("cgo enabled keeps cgo files", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("CGO_ENABLED", "1")

		dir := importDirFixture(t)

		// --- When ---
		have, err := importDir(rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{"cgo.go", "old.go", "plain.go"}
		assert.Equal(t, want, sortedGoFiles(have))
	})

	t.Run("error - tagged files excluded without tag", func(t *testing.T) {
		// --- Given ---
		dir := modkit.Path("testdata/projects/simple_tagged/project")

		// --- When ---
		_, err := importDir(ring.New(), dir)

		// --- Then ---
		assert.ErrorContain(t, "no buildable Go source files", err)
	})
}

func importDirFixture(t tester.T) string {
	t.Helper()
	dir := t.TempDir()
	oskit.Write(t, "package p\n", dir, "plain.go")
	oskit.Write(t, "//go:build cgo\n\npackage p\n", dir, "cgo.go")
	oskit.Write(t, "//go:build go1.1\n\npackage p\n", dir, "old.go")
	oskit.Write(t, "//go:build go1.99\n\npackage p\n", dir, "future.go")
	return dir
}

func sortedGoFiles(bp *build.Package) []string {
	have := append([]string(nil), bp.GoFiles...)
	sort.Strings(have)
	return have
}

func Test_goFlagsTags_tabular(t *testing.T) {
	tt := []struct {
		testN string

		flags string
		want  []string
	}{
		{"empty", "", nil},
		{"equals form", "-tags=a,b", []string{"a", "b"}},
		{"separate value", "-tags a,b", []string{"a", "b"}},
		{"double dash", "--tags a", []string{"a"}},
		{"missing value", "-tags", nil},
		{"other flags", "-mod=mod -tags=x -v", []string{"x"}},
		{"repeated", "-tags=a -tags=b", []string{"a", "b"}},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := goFlagsTags(tc.flags)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_buildTags(t *testing.T) {
	t.Run("meta tag only", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		rng.EnvSet("GOFLAGS", "")

		// --- When ---
		have := buildTags(rng)

		// --- Then ---
		assert.Equal(t, []string{BuildTag}, have)
	})

	t.Run("unions goflags and the meta tag", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		rng.EnvSet("GOFLAGS", "-tags=extra,other")

		// --- When ---
		have := buildTags(rng)

		// --- Then ---
		assert.Equal(t, []string{"extra", "other", BuildTag}, have)
	})

	t.Run("goflags without a meta tag", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("GOFLAGS", "-tags extra")

		// --- When ---
		have := buildTags(rng)

		// --- Then ---
		assert.Equal(t, []string{"extra"}, have)
	})
}

func Test_listTagArgs(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("GOFLAGS", "")

		// --- When ---
		have := listTagArgs(rng)

		// --- Then ---
		assert.Nil(t, have)
	})

	t.Run("one flag for the whole list", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())
		rng.EnvSet("GOFLAGS", "-tags=extra")

		// --- When ---
		have := listTagArgs(rng)

		// --- Then ---
		assert.Equal(t, []string{"-tags", "extra," + BuildTag}, have)
	})
}

func Test_splitTags_tabular(t *testing.T) {
	tt := []struct {
		testN string

		in   string
		want []string
	}{
		{"empty", "", []string{}},
		{"one", "a", []string{"a"}},
		{"several", "a,b", []string{"a", "b"}},
		{"spaces and blanks", " a , ,b ", []string{"a", "b"}},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := splitTags(tc.in)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
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
			"module root",
			"example.com/mod",
			"/src/mod",
			"/src/mod",
			"example.com/mod",
		},
		{
			"sub package",
			"example.com/mod",
			"/src/mod",
			"/src/mod/a/b",
			"example.com/mod/a/b",
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
		assert.ErrorIs(t, os.ErrNotExist, err)
		assert.Equal(t, "", have)
	})

	t.Run("error - no module directive", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Write(t, "go 1.26\n", t.TempDir(), "go.mod")

		// --- When ---
		have, err := readModulePath(pth)

		// --- Then ---
		assert.ErrorIs(t, errNoModule, err)
		assert.Equal(t, "", have)
	})
}
