// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/pathkit"

	gmt "github.com/ctx42/gomake/internal/cli/clitest"
)

func Test_withPkgSpec(t *testing.T) {
	// --- Given ---
	pkg := &Package{
		ImpPath: "example.com/spec/repo/pkg",
		args:    []string{"arg0"},
	}

	// --- When ---
	withPkgSpec(pkg)

	// --- Then ---
	assert.Empty(t, pkg.ImpPath)
	assert.Equal(t, "example.com/spec/repo/pkg", pkg.ImpSpec)
}

func Test_withPkgNS(t *testing.T) {
	// --- Given ---
	pkg := &Package{}

	// --- When ---
	withPkgNS("ns")(pkg)

	// --- Then ---
	assert.Equal(t, "ns", pkg.PkgNS)
}

func Test_NewPackage(t *testing.T) {
	t.Run("by path", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/imports/pkg2"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		rng := ring.New()

		// --- When ---
		pkg, err := NewPackage(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", pkg.PkgNS)
		assert.Equal(t, prj.Root(), pkg.ImpPath)
		assert.Equal(t, "example.com/comp/pkg2", pkg.ImpSpec)
		assert.Equal(t, "pkg2", pkg.Name)
		assert.Equal(t, []string{"file0.go", "file1.go"}, pkg.Files)
	})

	t.Run("with namespace", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/imports/pkg2"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		rng := ring.New()

		// --- When ---
		pkg, err := NewPackage(rng, prj.Root(), withPkgNS("ns"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "ns", pkg.PkgNS)
	})

	t.Run("not tagged files no build tag", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "386")

		// --- When ---
		pkg, err := NewPackage(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			"main.go",
			"main_helpers.go",
			"makefile.go",
			"makefile_386.go",
			"makefile_windows.go",
			"makefile_windows_386.go",
		}
		assert.Equal(t, want, pkg.Files)
	})

	t.Run("not tagged files with build tag", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "386")
		rng = SetBuildTag(rng)

		// --- When ---
		pkg, err := NewPackage(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			"main.go",
			"main_helpers.go",
			"makefile.go",
			"makefile_386.go",
			"makefile_windows.go",
			"makefile_windows_386.go",
		}
		assert.Equal(t, want, pkg.Files)
	})

	t.Run("tagged files no build tag", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os_build_tag/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "386")

		// --- When ---
		pkg, err := NewPackage(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			"main.go",
			"main_helpers.go",
		}
		assert.Equal(t, want, pkg.Files)
	})

	t.Run("tagged files with build tag", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/arch_os_build_tag/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "386")
		rng = SetBuildTag(rng)

		// --- When ---
		pkg, err := NewPackage(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			"main.go",
			"main_helpers.go",
			"makefile.go",
			"makefile_386.go",
			"makefile_windows.go",
			"makefile_windows_386.go",
		}
		assert.Equal(t, want, pkg.Files)
	})

	t.Run("not existing import path", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		pth := pathkit.AbsPath(t, "testing/not/existing")

		// --- When ---
		pkg, err := NewPackage(rng, pth)

		// --- Then ---
		assert.ErrorIs(t, ErrGoList, err)
		assert.Nil(t, pkg)
	})

	t.Run("empty import path", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		// --- When ---
		pkg, err := NewPackage(rng, "")

		// --- Then ---
		assert.ErrorIs(t, ErrAbsPath, err)
		assert.Nil(t, pkg)
	})

	t.Run("hyphened package name", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/imports/xx-pkg"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		rng := ring.New()

		// --- When ---
		pkg, err := NewPackage(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", pkg.PkgNS)
		assert.Equal(t, prj.Root(), pkg.ImpPath)
		assert.Equal(t, "example.com/comp/xx-pkg", pkg.ImpSpec)
		assert.Equal(t, "pkg", pkg.Name)
		assert.Equal(t, []string{"file0.go"}, pkg.Files)
	})

	t.Run("by spec with build tag", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/imports/pkg0"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		rng := SetBuildTag(ring.New())

		// --- When ---
		pkg, err := NewPackage(rng, prj.ImpSpec(), withPkgSpec)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", pkg.PkgNS)
		assert.Equal(t, prj.Root(), pkg.ImpPath)
		assert.Equal(t, "example.com/comp/pkg0", pkg.ImpSpec)
		assert.Equal(t, "pkg0", pkg.Name)
		assert.Equal(t, []string{"file0.go"}, pkg.Files)
	})

	t.Run("import path not absolute error", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		relPath := "../../testdata/imports/pkg2"

		// --- When ---
		pkg, err := NewPackage(rng, relPath)

		// --- Then ---
		assert.ErrorIs(t, ErrAbsPath, err)
		assert.Nil(t, pkg)
	})

	t.Run("package by import spec", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/imports/pkg2"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		rng := ring.New()

		// --- When ---
		pkg, err := NewPackage(rng, prj.ImpSpec(), withPkgSpec)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", pkg.PkgNS)
		assert.Equal(t, prj.Root(), pkg.ImpPath)
		assert.Equal(t, "example.com/comp/pkg2", pkg.ImpSpec)
		assert.Equal(t, "pkg2", pkg.Name)
		assert.Equal(t, []string{"file0.go", "file1.go"}, pkg.Files)
	})

	t.Run("package by import spec with namespace", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/imports/pkg2"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		rng := ring.New()

		// --- When ---
		pkg, err := NewPackage(
			rng,
			prj.ImpSpec(),
			withPkgSpec,
			withPkgNS("ns"),
		)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "ns", pkg.PkgNS)
		assert.Equal(t, prj.Root(), pkg.ImpPath)
		assert.Equal(t, "example.com/comp/pkg2", pkg.ImpSpec)
		assert.Equal(t, "pkg2", pkg.Name)
		assert.Equal(t, []string{"file0.go", "file1.go"}, pkg.Files)
	})

	t.Run("not existing import spec", func(t *testing.T) {
		// --- Given ---
		imp := "example.com/not/existing"

		rng := ring.New()

		// --- When ---
		pkg, err := NewPackage(rng, imp, withPkgSpec)

		// --- Then ---
		assert.ErrorIs(t, ErrGoList, err)
		assert.Nil(t, pkg)
	})

	t.Run("served from cache without go list", func(t *testing.T) {
		// --- Given ---
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		proj := modkit.Root()
		spec := "example.com/cached/pkg"
		rng := ring.New()
		rng.EnvSet("GOMAKE_PROJECT_DIR", proj)
		key, ok := listCacheKey(rng, proj, spec)
		assert.True(t, ok)
		storeListCache(key, []byte(`{"Name":"cached"}`))

		// --- When ---
		pkg, err := NewPackage(rng, spec, withPkgSpec)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "cached", pkg.Name)
	})
}
