// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/modkit"

	gmt "github.com/ctx42/gomake/internal/cli/clitest"
)

func Test_NewMakefile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/showcase_imports/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()
		prj.Chdir()

		rng := ring.New()

		// --- When ---
		pmf, err := NewMakefile(rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "Package main is a simple makefile example.", pmf.Doc)
		assert.Empty(t, pmf.Default)
		want := []string{
			"abc:ns0:hello",
			"abc:ns0:ns1:hello",
			"abc:ns0:ns1:ns2:hello-hello",
			"imported",
			"local",
			"mx:panic-string",
			"mx:print",
			"ns:pkg1",
			"pkg0",
		}
		assert.Equal(t, want, pmf.Targets.Names())
	})

	t.Run("error - import path not absolute", func(t *testing.T) {
		// --- Given ---
		relPath := "../../testdata/projects/showcase_imports/project"

		rng := ring.New()

		// --- When ---
		pmf, err := NewMakefile(rng, relPath)

		// --- Then ---
		assert.ErrorIs(t, ErrAbsPath, err)
		assert.Nil(t, pmf)
	})

	t.Run("import path to empty directory", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/empty"
		absPath := modkit.Path(relPath)

		rng := ring.New()

		// --- When ---
		pmf, err := NewMakefile(rng, absPath)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", pmf.Doc)
		assert.Equal(t, 0, pmf.Targets.Len())
		assert.Equal(t, "", pmf.Default)
	})
}

func Test_MakefileFromPackage(t *testing.T) {
	t.Run("basic with requested GOOS", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/showcase_targets/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		pkg := NewTestHelper(t, rng, prj.Root()).pkg

		// --- When ---
		pmf, err := MakefileFromPackage(rng, pkg)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			"basic",
			"basic-args",
			"basic-env",
			"basic-win",
			"kebab-case:hello-world",
			"long",
			"ns-def",
			"ns-def:hello",
			"ns-def:world",
			"ns0:hello",
			"ns0:ns1:hello",
			"ns0:ns1:ns2:hello-hello",
			"ns0:ns1:ns2:say-my-name",
			"ns0:ns3:hello",
			"ns0:ns3:ns4:hello",
			"panic-string",
			"print-args",
			"say-hello",
		}
		assert.Equal(t, want, pmf.Targets.Names())
	})

	t.Run("default as local function", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/default_local/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		rng := ring.New()
		pkg := NewTestHelper(t, rng, prj.Root()).pkg

		// --- When ---
		pmf, err := MakefileFromPackage(rng, pkg)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", pmf.Doc)
		assert.Equal(t, "hello", pmf.Default)
		assert.NotNil(t, pmf.Targets.Get("hello"))
		assert.NotNil(t, pmf.Targets.Get("bye"))
		assert.Len(t, 2, pmf.Targets.List())
	})

	t.Run("default as local namespace method", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/default_from_ns/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		rng := ring.New()
		pkg := NewTestHelper(t, rng, prj.Root()).pkg

		// --- When ---
		pmf, err := MakefileFromPackage(rng, pkg)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", pmf.Doc)
		assert.Equal(t, "ns:hello", pmf.Default)
		assert.NotNil(t, pmf.Targets.Get("ns:hello"))
		assert.Len(t, 1, pmf.Targets.List())
	})

	t.Run("error - duplicated imported target", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/dup_imported/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := ring.New()
		pkg := NewTestHelper(t, rng, prj.Root()).pkg

		// --- When ---
		pmf, err := MakefileFromPackage(rng, pkg)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.ErrorContain(t, "PKG1, pkg1.Pkg1", err)
		assert.Nil(t, pmf)
	})

	t.Run("error - duplicated imported namespace", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/dup_imported_ns/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		rng := ring.New()
		pkg := NewTestHelper(t, rng, prj.Root()).pkg

		// --- When ---
		pmf, err := MakefileFromPackage(rng, pkg)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.ErrorContain(t, "NS.M0, pkg4.NS.M0", err)
		assert.Nil(t, pmf)
	})

	t.Run("error - duplicated local target", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/dup_local/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		rng := ring.New()
		pkg := NewTestHelper(t, rng, prj.Root()).pkg

		// --- When ---
		pmf, err := MakefileFromPackage(rng, pkg)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.ErrorContain(t, "HELLO, Hello", err)
		assert.Nil(t, pmf)
	})

	t.Run("error - duplicated namespaced targets", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/dup_ns/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		rng := ring.New()
		pkg := NewTestHelper(t, rng, prj.Root()).pkg

		// --- When ---
		pmf, err := MakefileFromPackage(rng, pkg)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.ErrorContain(t, "NS1.HELLO, NS1.Hello", err)
		assert.Nil(t, pmf)
	})
}
