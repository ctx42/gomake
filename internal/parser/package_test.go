// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/pathkit"

	gmt "github.com/ctx42/gomake/internal/cli/clitest"
	"github.com/ctx42/gomake/pkg/gomake"
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

func Test_withPkgDir(t *testing.T) {
	// --- Given ---
	pkg := &Package{}

	// --- When ---
	withPkgDir("/path/to")(pkg)

	// --- Then ---
	assert.Equal(t, "/path/to", pkg.ImpPath)
}

func Test_NewPackage(t *testing.T) {
	t.Run("by path", func(t *testing.T) {
		// --- Given ---
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path("testdata/imports/pkg2"))
		prj.GoModInit()
		prj.Close()

		// --- When ---
		have, err := NewPackage(ring.New(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", have.PkgNS)
		assert.Equal(t, prj.Root(), have.ImpPath)
		assert.Equal(t, "example.com/comp/pkg2", have.ImpSpec)
		assert.Equal(t, "pkg2", have.Name)
		assert.Equal(t, []string{"file0.go", "file1.go"}, have.Files)
	})

	t.Run("with namespace", func(t *testing.T) {
		// --- Given ---
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path("testdata/imports/pkg2"))
		prj.GoModInit()
		prj.Close()

		// --- When ---
		have, err := NewPackage(ring.New(), prj.Root(), withPkgNS("ns"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "ns", have.PkgNS)
	})

	t.Run("not tagged files no build tag", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "386")

		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path("testdata/projects/arch_os/project"))
		prj.GoModInit()
		prj.Close()

		// --- When ---
		have, err := NewPackage(rng, prj.Root())

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
		assert.Equal(t, want, have.Files)
	})

	t.Run("not tagged files with build tag", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "386")
		rng = SetBuildTag(rng)

		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path("testdata/projects/arch_os/project"))
		prj.GoModInit()
		prj.Close()

		// --- When ---
		have, err := NewPackage(rng, prj.Root())

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
		assert.Equal(t, want, have.Files)
	})

	t.Run("tagged files no build tag", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "386")

		relPath := "testdata/projects/arch_os_build_tag/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		// --- When ---
		have, err := NewPackage(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			"main.go",
			"main_helpers.go",
		}
		assert.Equal(t, want, have.Files)
	})

	t.Run("tagged files with build tag", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "386")
		rng = SetBuildTag(rng)

		relPath := "testdata/projects/arch_os_build_tag/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		// --- When ---
		have, err := NewPackage(rng, prj.Root())

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
		assert.Equal(t, want, have.Files)
	})

	t.Run("error - not existing import path", func(t *testing.T) {
		// --- Given ---
		pth := pathkit.AbsPath(t, "testing/not/existing")

		// --- When ---
		have, err := NewPackage(ring.New(), pth)

		// --- Then ---
		assert.ErrorIs(t, ErrGoList, err)
		assert.Nil(t, have)
	})

	t.Run("error - empty import path", func(t *testing.T) {
		// --- When ---
		have, err := NewPackage(ring.New(), "")

		// --- Then ---
		assert.ErrorIs(t, ErrAbsPath, err)
		assert.Nil(t, have)
	})

	t.Run("hyphened package name", func(t *testing.T) {
		// --- Given ---
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path("testdata/imports/xx-pkg"))
		prj.GoModInit()
		prj.Close()

		// --- When ---
		have, err := NewPackage(ring.New(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", have.PkgNS)
		assert.Equal(t, prj.Root(), have.ImpPath)
		assert.Equal(t, "example.com/comp/xx-pkg", have.ImpSpec)
		assert.Equal(t, "pkg", have.Name)
		assert.Equal(t, []string{"file0.go"}, have.Files)
	})

	t.Run("by spec with build tag", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())

		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path("testdata/imports/pkg0"))
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		// --- When ---
		have, err := NewPackage(rng, prj.ImpSpec(), withPkgSpec)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", have.PkgNS)
		assert.Equal(t, prj.Root(), have.ImpPath)
		assert.Equal(t, "example.com/comp/pkg0", have.ImpSpec)
		assert.Equal(t, "pkg0", have.Name)
		assert.Equal(t, []string{"file0.go"}, have.Files)
	})

	t.Run("error - import path not absolute", func(t *testing.T) {
		// --- When ---
		have, err := NewPackage(ring.New(), "../../testdata/imports/pkg2")

		// --- Then ---
		assert.ErrorIs(t, ErrAbsPath, err)
		assert.Nil(t, have)
	})

	t.Run("package by import spec", func(t *testing.T) {
		// --- Given ---
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path("testdata/imports/pkg2"))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		// --- When ---
		have, err := NewPackage(ring.New(), prj.ImpSpec(), withPkgSpec)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", have.PkgNS)
		assert.Equal(t, prj.Root(), have.ImpPath)
		assert.Equal(t, "example.com/comp/pkg2", have.ImpSpec)
		assert.Equal(t, "pkg2", have.Name)
		assert.Equal(t, []string{"file0.go", "file1.go"}, have.Files)
	})

	t.Run("package by import spec with namespace", func(t *testing.T) {
		// --- Given ---
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path("testdata/imports/pkg2"))
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		// --- When ---
		have, err := NewPackage(
			ring.New(),
			prj.ImpSpec(),
			withPkgSpec,
			withPkgNS("ns"),
		)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "ns", have.PkgNS)
		assert.Equal(t, prj.Root(), have.ImpPath)
		assert.Equal(t, "example.com/comp/pkg2", have.ImpSpec)
		assert.Equal(t, "pkg2", have.Name)
		assert.Equal(t, []string{"file0.go", "file1.go"}, have.Files)
	})

	t.Run("error - missing import spec", func(t *testing.T) {
		// --- Given ---
		imp := "example.com/not/existing"

		// --- When ---
		have, err := NewPackage(ring.New(), imp, withPkgSpec)

		// --- Then ---
		assert.ErrorIs(t, ErrGoList, err)
		assert.Nil(t, have)
	})

	t.Run("served from cache without go list", func(t *testing.T) {
		// --- Given ---
		proj := modkit.Root()

		rng := ring.New()
		rng.EnvSet("XDG_CACHE_HOME", t.TempDir())
		rng.EnvSet(gomake.ProjectDirEnvKey, proj)

		spec := "example.com/cached/pkg"

		key, ok := listCacheKey(rng, proj, spec)
		assert.True(t, ok)
		storeListCache(rng, key, []byte(`{"Name":"cached"}`))

		// --- When ---
		have, err := NewPackage(rng, spec, withPkgSpec)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "cached", have.Name)
	})

	t.Run("cache entry for another package", func(t *testing.T) {
		// --- Given ---
		proj := modkit.Root()
		rng := ring.New()
		rng.EnvSet("XDG_CACHE_HOME", t.TempDir())
		rng.EnvSet(gomake.ProjectDirEnvKey, proj)

		spec := "github.com/ctx42/ring/pkg/ring"
		key, _ := listCacheKey(rng, proj, spec)
		entry := `{"ImportPath":"example.com/other","Name":"cached","Dir":"/"}`
		storeListCache(rng, key, []byte(entry))

		// --- When ---
		have, err := NewPackage(rng, spec, withPkgSpec)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "ring", have.Name)
	})

	t.Run("cache entry with vanished directory", func(t *testing.T) {
		// --- Given ---
		proj := modkit.Root()
		rng := ring.New()
		rng.EnvSet("XDG_CACHE_HOME", t.TempDir())
		rng.EnvSet(gomake.ProjectDirEnvKey, proj)

		spec := "github.com/ctx42/ring/pkg/ring"
		key, _ := listCacheKey(rng, proj, spec)
		gone := filepath.Join(t.TempDir(), "gone")
		entry := `{"ImportPath":"` + spec + `","Name":"cached","Dir":"` +
			gone + `"}`
		storeListCache(rng, key, []byte(entry))

		// --- When ---
		have, err := NewPackage(rng, spec, withPkgSpec)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "ring", have.Name)
	})
}
