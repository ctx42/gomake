// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"runtime"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"

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

		// --- When ---
		have, err := NewMakefile(ring.New(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "Package main is a simple makefile example.", have.Doc)
		assert.Empty(t, have.Default)
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
		assert.Equal(t, want, have.Targets.Names())
	})

	t.Run("error - import path not absolute", func(t *testing.T) {
		// --- Given ---
		relPath := "../../testdata/projects/showcase_imports/project"

		// --- When ---
		have, err := NewMakefile(ring.New(), relPath)

		// --- Then ---
		assert.ErrorIs(t, ErrAbsPath, err)
		assert.Nil(t, have)
	})

	t.Run("import path to empty directory", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/empty"

		// --- When ---
		have, err := NewMakefile(ring.New(), modkit.Path(relPath))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", have.Doc)
		assert.Equal(t, 0, have.Targets.Len())
		assert.Equal(t, "", have.Default)
	})
	t.Run("warns only about aliased context and ring", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := SetBuildTag(tst.Ring())

		dir := t.TempDir()
		oskit.Write(t, "module example.com/mk\n\ngo 1.26\n", dir, "go.mod")
		src := "" +
			"//go:build gomake\n" +
			"\n" +
			"package main\n" +
			"\n" +
			"import (\n" +
			"\t\"context\"\n" +
			"\n" +
			"\tr \"github.com/ctx42/ring/pkg/ring\"\n" +
			"\t\"example.com/other\"\n" +
			")\n" +
			"\n" +
			"func Aliased(ctx context.Context, rng *r.Ring) error {\n" +
			"\treturn nil\n" +
			"}\n" +
			"\n" +
			"func Other(ctx context.Context, rng *other.Ring) error {\n" +
			"\treturn nil\n" +
			"}\n"
		oskit.Write(t, src, dir, "makefile.go")

		// --- When ---
		have, err := NewMakefile(rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 0, have.Targets.Len())
		want := "gomake: skipping Aliased: " +
			"aliased context or ring parameter\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("ring from another package", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())

		dir := t.TempDir()
		oskit.Write(t, "module example.com/mk\n\ngo 1.26\n", dir, "go.mod")
		src := "" +
			"//go:build gomake\n" +
			"\n" +
			"package main\n" +
			"\n" +
			"import (\n" +
			"\t\"container/ring\"\n" +
			"\t\"context\"\n" +
			")\n" +
			"\n" +
			"func Other(ctx context.Context, rng *ring.Ring) error {\n" +
			"\treturn nil\n" +
			"}\n"
		oskit.Write(t, src, dir, "makefile.go")

		// --- When ---
		have, err := NewMakefile(rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 0, have.Targets.Len())
	})

	t.Run("namespace not borrowed across packages", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())

		dir := t.TempDir()
		oskit.Write(t, "module example.com/mk\n\ngo 1.26\n", dir, "go.mod")
		libDir := oskit.MkdirAll(t, dir, "lib")
		oskit.Write(t, "package lib\n\ntype Docker struct{}\n", libDir, "a.go")
		lib := "" +
			"package lib\n" +
			"\n" +
			"import (\n" +
			"\t\"context\"\n" +
			"\n" +
			"\t\"github.com/ctx42/ring/pkg/ring\"\n" +
			")\n" +
			"\n" +
			"type Compose Docker\n" +
			"\n" +
			"func (Compose) Up(ctx context.Context, rng *ring.Ring) error {\n" +
			"\treturn nil\n" +
			"}\n"
		oskit.Write(t, lib, libDir, "b.go")
		src := "" +
			"//go:build gomake\n" +
			"\n" +
			"package main\n" +
			"\n" +
			"import (\n" +
			"\t\"context\"\n" +
			"\n" +
			"\t\"github.com/ctx42/ring/pkg/ring\"\n" +
			"\n" +
			"\t\"example.com/mk/lib\" //gomake:import\n" +
			")\n" +
			"\n" +
			"type Docker struct{} //gomake:ns_root\n" +
			"\n" +
			"func (Docker) Build(\n" +
			"\tctx context.Context,\n" +
			"\trng *ring.Ring,\n" +
			") error {\n" +
			"\treturn nil\n" +
			"}\n" +
			"\n" +
			"var _ lib.Compose\n"
		oskit.Write(t, src, dir, "makefile.go")

		// --- When ---
		have, err := NewMakefile(rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"docker:build"}, have.Targets.Names())
	})

	t.Run("unexported namespace in import", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())

		dir := t.TempDir()
		oskit.Write(t, "module example.com/mk\n\ngo 1.26\n", dir, "go.mod")
		lib := "" +
			"package lib\n" +
			"\n" +
			"import (\n" +
			"\t\"context\"\n" +
			"\n" +
			"\t\"github.com/ctx42/ring/pkg/ring\"\n" +
			")\n" +
			"\n" +
			"type tools struct{} //gomake:ns_root\n" +
			"\n" +
			"func (tools) Lint(ctx context.Context, rng *ring.Ring) error {\n" +
			"\treturn nil\n" +
			"}\n"
		oskit.Write(t, lib, oskit.MkdirAll(t, dir, "lib"), "lib.go")
		src := "" +
			"//go:build gomake\n" +
			"\n" +
			"package main\n" +
			"\n" +
			"import \"example.com/mk/lib\" //gomake:import\n" +
			"\n" +
			"var _ = lib.X\n"
		oskit.Write(t, src, dir, "makefile.go")

		// --- When ---
		have, err := NewMakefile(rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 0, have.Targets.Len())
	})

	t.Run("generic namespace", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())

		dir := t.TempDir()
		oskit.Write(t, "module example.com/mk\n\ngo 1.26\n", dir, "go.mod")
		src := "" +
			"//go:build gomake\n" +
			"\n" +
			"package main\n" +
			"\n" +
			"import (\n" +
			"\t\"context\"\n" +
			"\n" +
			"\t\"github.com/ctx42/ring/pkg/ring\"\n" +
			")\n" +
			"\n" +
			"type B[T any] struct{} //gomake:ns_root\n" +
			"\n" +
			"func (B[T]) Run(ctx context.Context, rng *ring.Ring) error {\n" +
			"\treturn nil\n" +
			"}\n"
		oskit.Write(t, src, dir, "makefile.go")

		// --- When ---
		have, err := NewMakefile(rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 0, have.Targets.Len())
	})

	t.Run("import alias avoids main names", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())

		dir := t.TempDir()
		oskit.Write(t, "module example.com/mk\n\ngo 1.26\n", dir, "go.mod")
		lib := "" +
			"package git\n" +
			"\n" +
			"import (\n" +
			"\t\"context\"\n" +
			"\n" +
			"\t\"github.com/ctx42/ring/pkg/ring\"\n" +
			")\n" +
			"\n" +
			"func Build(ctx context.Context, rng *ring.Ring) error {\n" +
			"\treturn nil\n" +
			"}\n"
		oskit.Write(t, lib, oskit.MkdirAll(t, dir, "x", "git"), "git.go")
		src := "" +
			"//go:build gomake\n" +
			"\n" +
			"package main\n" +
			"\n" +
			"import gt \"example.com/mk/x/git\" //gomake:import\n" +
			"\n" +
			"var _ = gt.Build\n" +
			"\n" +
			"func git() {}\n"
		oskit.Write(t, src, dir, "makefile.go")
		have := must.Value(NewMakefile(rng, dir))

		// --- When ---
		imports := have.Targets.goImports()

		// --- Then ---
		assert.Equal(t, "\ngit2 \"example.com/mk/x/git\"\n", imports)
		assert.Contain(t, "git2.Build", have.Targets.goCode(false))
	})

	t.Run("same import in two files", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())

		dir := t.TempDir()
		oskit.Write(t, "module example.com/mk\n\ngo 1.26\n", dir, "go.mod")
		lib := "" +
			"package lib\n" +
			"\n" +
			"import (\n" +
			"\t\"context\"\n" +
			"\n" +
			"\t\"github.com/ctx42/ring/pkg/ring\"\n" +
			")\n" +
			"\n" +
			"func Build(ctx context.Context, rng *ring.Ring) error {\n" +
			"\treturn nil\n" +
			"}\n"
		oskit.Write(t, lib, oskit.MkdirAll(t, dir, "lib"), "lib.go")
		src := "" +
			"//go:build gomake\n" +
			"\n" +
			"package main\n" +
			"\n" +
			"import \"example.com/mk/lib\" //gomake:import\n" +
			"\n" +
			"var _ = lib.Build\n"
		oskit.Write(t, src, dir, "makefile.go")
		oskit.Write(t, src, dir, "makefile_"+runtime.GOOS+".go")

		// --- When ---
		have, err := NewMakefile(rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"build"}, have.Targets.Names())
	})

	t.Run("error - malformed import tag", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())

		dir := t.TempDir()
		oskit.Write(t, "module example.com/mk\n\ngo 1.26\n", dir, "go.mod")
		oskit.Write(t, "package lib\n", oskit.MkdirAll(t, dir, "lib"), "lib.go")
		src := "" +
			"//go:build gomake\n" +
			"\n" +
			"package main\n" +
			"\n" +
			"import \"example.com/mk/lib\" //gomake:import ns extra\n"
		oskit.Write(t, src, dir, "makefile.go")

		// --- When ---
		have, err := NewMakefile(rng, dir)

		// --- Then ---
		assert.ErrorIs(t, ErrImportTag, err)
		assert.ErrorContain(t, "example.com/mk/lib", err)
		assert.Nil(t, have)
	})

	t.Run("error - default names no target", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())

		dir := t.TempDir()
		oskit.Write(t, "module example.com/mk\n\ngo 1.26\n", dir, "go.mod")
		src := "" +
			"//go:build gomake\n" +
			"\n" +
			"package main\n" +
			"\n" +
			"var Default = Missing\n"
		oskit.Write(t, src, dir, "makefile.go")

		// --- When ---
		have, err := NewMakefile(rng, dir)

		// --- Then ---
		assert.ErrorIs(t, ErrNoDefault, err)
		assert.ErrorContain(t, `"Missing"`, err)
		assert.Nil(t, have)
	})

	t.Run("default in package sharing a name", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())

		dir := t.TempDir()
		oskit.Write(t, "module example.com/mk\n\ngo 1.26\n", dir, "go.mod")
		lib := "" +
			"package git\n" +
			"\n" +
			"import (\n" +
			"\t\"context\"\n" +
			"\n" +
			"\t\"github.com/ctx42/ring/pkg/ring\"\n" +
			")\n" +
			"\n" +
			"func Build(ctx context.Context, rng *ring.Ring) error {\n" +
			"\treturn nil\n" +
			"}\n"
		oskit.Write(t, lib, oskit.MkdirAll(t, dir, "one", "git"), "git.go")
		oskit.Write(t, lib, oskit.MkdirAll(t, dir, "two", "git"), "git.go")
		src := "" +
			"//go:build gomake\n" +
			"\n" +
			"package main\n" +
			"\n" +
			"import (\n" +
			"\ta \"example.com/mk/one/git\" //gomake:import one\n" +
			"\tb \"example.com/mk/two/git\" //gomake:import two\n" +
			")\n" +
			"\n" +
			"var Default = b.Build\n" +
			"\n" +
			"var _ = a.Build\n"
		oskit.Write(t, src, dir, "makefile.go")

		// --- When ---
		have, err := NewMakefile(rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "two:build", have.Default)
	})

	t.Run("default alias reused in another file", func(t *testing.T) {
		// --- Given ---
		rng := SetBuildTag(ring.New())

		dir := t.TempDir()
		oskit.Write(t, "module example.com/mk\n\ngo 1.26\n", dir, "go.mod")
		lib := "" +
			"package git\n" +
			"\n" +
			"import (\n" +
			"\t\"context\"\n" +
			"\n" +
			"\t\"github.com/ctx42/ring/pkg/ring\"\n" +
			")\n" +
			"\n" +
			"func Build(ctx context.Context, rng *ring.Ring) error {\n" +
			"\treturn nil\n" +
			"}\n"
		oskit.Write(t, lib, oskit.MkdirAll(t, dir, "one", "git"), "git.go")
		oskit.Write(t, lib, oskit.MkdirAll(t, dir, "two", "git"), "git.go")
		one := "" +
			"//go:build gomake\n" +
			"\n" +
			"package main\n" +
			"\n" +
			"import x \"example.com/mk/one/git\" //gomake:import one\n" +
			"\n" +
			"var _ = x.Build\n"
		oskit.Write(t, one, dir, "makefile.go")
		two := "" +
			"//go:build gomake\n" +
			"\n" +
			"package main\n" +
			"\n" +
			"import x \"example.com/mk/two/git\" //gomake:import two\n" +
			"\n" +
			"var Default = x.Build\n"
		oskit.Write(t, two, dir, "makefile_"+runtime.GOOS+".go")

		// --- When ---
		have, err := NewMakefile(rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "two:build", have.Default)
	})
}

func Test_MakefileFromPackage(t *testing.T) {
	t.Run("basic with requested GOOS", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("GOOS", "windows")

		relPath := "testdata/projects/showcase_targets/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		pkg := NewTestHelper(t, rng, prj.Root()).pkg

		// --- When ---
		have, err := MakefileFromPackage(rng, pkg)

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
		assert.Equal(t, want, have.Targets.Names())
	})

	t.Run("default as local function", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path("testdata/projects/default_local/project"))
		prj.GoModInit()
		prj.Close()

		pkg := NewTestHelper(t, rng, prj.Root()).pkg

		// --- When ---
		have, err := MakefileFromPackage(rng, pkg)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", have.Doc)
		assert.Equal(t, "hello", have.Default)
		assert.NotNil(t, have.Targets.Get("hello"))
		assert.NotNil(t, have.Targets.Get("bye"))
		assert.Len(t, 2, have.Targets.List())
	})

	t.Run("default as local namespace method", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		relPath := "testdata/projects/default_from_ns/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.Close()

		pkg := NewTestHelper(t, rng, prj.Root()).pkg

		// --- When ---
		have, err := MakefileFromPackage(rng, pkg)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", have.Doc)
		assert.Equal(t, "ns:hello", have.Default)
		assert.NotNil(t, have.Targets.Get("ns:hello"))
		assert.Len(t, 1, have.Targets.List())
	})

	t.Run("error - duplicated imported target", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path("testdata/projects/dup_imported/project"))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		pkg := NewTestHelper(t, rng, prj.Root()).pkg

		// --- When ---
		have, err := MakefileFromPackage(rng, pkg)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.ErrorContain(t, "PKG1, pkg1.Pkg1", err)
		assert.Nil(t, have)
	})

	t.Run("error - duplicated imported namespace", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		relPath := "testdata/projects/dup_imported_ns/project"
		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path(relPath))
		prj.GoModInit()
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.Close()

		pkg := NewTestHelper(t, rng, prj.Root()).pkg

		// --- When ---
		have, err := MakefileFromPackage(rng, pkg)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.ErrorContain(t, "NS.M0, pkg4.NS.M0", err)
		assert.Nil(t, have)
	})

	t.Run("error - duplicated local target", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path("testdata/projects/dup_local/project"))
		prj.GoModInit()
		prj.Close()

		pkg := NewTestHelper(t, rng, prj.Root()).pkg

		// --- When ---
		have, err := MakefileFromPackage(rng, pkg)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.ErrorContain(t, "HELLO, Hello", err)
		assert.Nil(t, have)
	})

	t.Run("error - duplicated namespaced targets", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		prj := gmt.NewProject(t)
		prj.ProjectFrom(modkit.Path("testdata/projects/dup_ns/project"))
		prj.GoModInit()
		prj.Close()

		pkg := NewTestHelper(t, rng, prj.Root()).pkg

		// --- When ---
		have, err := MakefileFromPackage(rng, pkg)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.ErrorContain(t, "NS1.HELLO, NS1.Hello", err)
		assert.Nil(t, have)
	})
}
