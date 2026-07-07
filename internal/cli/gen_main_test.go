// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/goldy"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/exekit"
	"github.com/ctx42/testkit/pkg/modkit"

	gmt "github.com/ctx42/gomake/internal/cli/clitest"
	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/internal/parser"
)

func Test_genMain(t *testing.T) {
	// setup creates test project based on makefiles in src directory than
	// generates [mkf.MakefileUser] file and returns the [gmt.Project]
	// instance for the created project.
	setup := func(t tester.T, src string) *gmt.Project {
		t.Helper()

		prj := gmt.NewProject(t)
		prj.GoModInit()
		prj.MakefilesFrom(src)
		prj.UseGomakeSrc(modkit.Root())
		prj.GoModTidy()
		prj.RequireXflag()
		prj.Close()

		rng := parser.SetBuildTag(ring.New())
		_, err := parser.GenMakefileUserAndSave(
			rng,
			prj.Root(),
			prj.Path(mkf.MakefileUser),
		)
		assert.NoError(t, err)
		return prj
	}

	t.Run("no targets", func(t *testing.T) {
		// --- Given ---
		relPth := "testdata/projects/no_targets/project"
		absPth := modkit.Path(relPth)
		prj := setup(t, absPth)

		// --- When ---
		err := genMain(prj.Path(mkf.MakefileGen), "1.2.3")

		// --- Then ---
		assert.NoError(t, err)
		prj.Exe("go", "mod", "tidy")
		want := "pick a target to execute\n"
		have := exekit.New(t, exekit.WithExitCode(126)).ExeStderr(prj.Compile())
		assert.Equal(t, want, have)
	})

	t.Run("no targets list targets", func(t *testing.T) {
		// --- Given ---
		relPth := "testdata/projects/no_targets/project"
		absPth := modkit.Path(relPth)
		prj := setup(t, absPth)

		// --- When ---
		err := genMain(prj.Path(mkf.MakefileGen), "1.2.3")

		// --- Then ---
		assert.NoError(t, err)
		want := "no targets\n"
		have := exekit.New(t).ExeStderr(prj.Compile(), "--list")
		assert.Equal(t, want, have)
	})

	t.Run("no targets print version", func(t *testing.T) {
		// --- Given ---
		relPth := "testdata/projects/no_targets/project"
		absPth := modkit.Path(relPth)
		prj := setup(t, absPth)

		// --- When ---
		err := genMain(prj.Path(mkf.MakefileGen), "1.2.3")

		// --- Then ---
		assert.NoError(t, err)
		want := "1.2.3\n"
		have := exekit.New(t).ExeStderr(prj.Compile(), "--version")
		assert.Equal(t, want, have)
	})

	t.Run("makefile without targets print help", func(t *testing.T) {
		// --- Given ---
		relPth := "testdata/projects/no_targets/project"
		absPth := modkit.Path(relPth)
		prj := setup(t, absPth)

		// --- When ---
		err := genMain(prj.Path(mkf.MakefileGen), "1.2.3")

		// --- Then ---
		assert.NoError(t, err)
		gfp := filepath.Join(absPth, "help_without_builtin.no_trim.gld")
		gld := goldy.Open(t, gfp)
		have := exekit.New(t).ExeStderr(prj.Compile(), "--help")
		assert.Equal(t, gld.String(), have)
	})

	t.Run("list targets", func(t *testing.T) {
		// --- Given ---
		relPth := "testdata/projects/showcase_imports/project"
		absPth := modkit.Path(relPth)
		prj := setup(t, absPth)

		// --- When ---
		err := genMain(prj.Path(mkf.MakefileGen), "1.2.3")

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"abc:ns0:hello\n" +
			"abc:ns0:ns1:hello\n" +
			"abc:ns0:ns1:ns2:hello-hello\n" +
			"imported\n" +
			"local\n" +
			"mx:panic-string\n" +
			"mx:print                       " +
			"is a test target with help message\n" +
			"ns:pkg1\n" +
			"pkg0                           is an example target\n"
		have := exekit.New(t).ExeStderr(prj.Compile(), "--list")
		assert.Equal(t, want, have)
	})

	t.Run("run local target", func(t *testing.T) {
		// --- Given ---
		relPth := "testdata/projects/showcase_imports/project"
		absPth := modkit.Path(relPth)
		prj := setup(t, absPth)

		// --- When ---
		err := genMain(prj.Path(mkf.MakefileGen), "1.2.3")

		// --- Then ---
		assert.NoError(t, err)
		want := "local target says hello"
		have := exekit.New(t).ExeStdout(prj.Compile(), "local")
		assert.Equal(t, want, have)
	})

	t.Run("run local target using an import", func(t *testing.T) {
		// --- Given ---
		relPth := "testdata/projects/showcase_imports/project"
		absPth := modkit.Path(relPth)
		prj := setup(t, absPth)

		// --- When ---
		err := genMain(prj.Path(mkf.MakefileGen), "1.2.3")

		// --- Then ---
		assert.NoError(t, err)
		want := "pkg8.Pkg8F1"
		have := exekit.New(t).ExeStdout(prj.Compile(), "imported")
		assert.Equal(t, want, have)
	})

	t.Run("run imported not namespaced target", func(t *testing.T) {
		// --- Given ---
		relPth := "testdata/projects/showcase_imports/project"
		absPth := modkit.Path(relPth)
		prj := setup(t, absPth)

		// --- When ---
		err := genMain(prj.Path(mkf.MakefileGen), "1.2.3")

		// --- Then ---
		assert.NoError(t, err)
		want := "pkg0.Pkg0"
		have := exekit.New(t).ExeStdout(prj.Compile(), "pkg0")
		assert.Equal(t, want, have)
	})

	t.Run("run imported namespaced target", func(t *testing.T) {
		// --- Given ---
		relPth := "testdata/projects/showcase_imports/project"
		absPth := modkit.Path(relPth)
		prj := setup(t, absPth)

		// --- When ---
		err := genMain(prj.Path(mkf.MakefileGen), "1.2.3")

		// --- Then ---
		assert.NoError(t, err)
		want := "pkg1.Pkg1"
		have := exekit.New(t).ExeStdout(prj.Compile(), "ns:pkg1")
		assert.Equal(t, want, have)
	})

	t.Run("run default local target", func(t *testing.T) {
		// --- Given ---
		relPth := "testdata/projects/default_local/project"
		absPth := modkit.Path(relPth)
		prj := setup(t, absPth)

		// --- When ---
		err := genMain(prj.Path(mkf.MakefileGen), "1.2.3")

		// --- Then ---
		assert.NoError(t, err)
		want := "gomake says hello"
		have := exekit.New(t).ExeStdout(prj.Compile())
		assert.Equal(t, want, have)
	})

	t.Run("run default local namespaced target", func(t *testing.T) {
		// --- Given ---
		relPth := "testdata/projects/default_from_ns/project"
		absPth := modkit.Path(relPth)
		prj := setup(t, absPth)

		// --- When ---
		err := genMain(prj.Path(mkf.MakefileGen), "1.2.3")

		// --- Then ---
		assert.NoError(t, err)
		want := "NS says hello"
		have := exekit.New(t).ExeStdout(prj.Compile())
		assert.Equal(t, want, have)
	})

	t.Run("run target from OS related file", func(t *testing.T) {
		// --- Given ---
		relPth := "testdata/projects/arch_os_build_tag/project"
		absPth := modkit.Path(relPth)
		prj := setup(t, absPth)

		// --- When ---
		err := genMain(prj.Path(mkf.MakefileGen), "1.2.3")

		// --- Then ---
		assert.NoError(t, err)
		want := fmt.Sprintf("TargetOS=os:%s", runtime.GOOS)
		have := exekit.New(t).ExeStdout(prj.Compile(), "target-os")
		assert.Equal(t, want, have)
	})

	t.Run("run target from not tagged makefiles", func(t *testing.T) {
		// --- Given ---
		relPth := "testdata/projects/arch_os/project"
		absPth := modkit.Path(relPth)
		prj := setup(t, absPth)

		// --- When ---
		err := genMain(prj.Path(mkf.MakefileGen), "1.2.3")

		// --- Then ---
		assert.NoError(t, err)
		want := fmt.Sprintf("TargetOS=os:%s", runtime.GOOS)
		have := exekit.New(t).ExeStdout(prj.Compile(), "target-os")
		assert.Equal(t, want, have)
	})

	t.Run("target can read its own name via gomake.TargetName",
		func(t *testing.T) {
			// --- Given ---
			relPth := "testdata/projects/showcase_targets/project"
			absPth := modkit.Path(relPth)
			prj := setup(t, absPth)

			// --- When ---
			err := genMain(prj.Path(mkf.MakefileGen), "1.2.3")

			// --- Then ---
			assert.NoError(t, err)
			want := "my name is ns0:ns1:ns2:say-my-name\n"
			have := exekit.New(t).
				ExeStdout(prj.Compile(), "ns0:ns1:ns2:say-my-name")
			assert.Equal(t, want, have)
		})

	t.Run("cannot create main file error", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "not_existing", mkf.MakefileMain)

		// --- When ---
		err := genMain(pth, "1.2.3")

		// --- Then ---
		var e *fs.PathError
		assert.ErrorAs(t, &e, err)
		assert.Equal(t, "open", e.Op)
		assert.Equal(t, syscall.ENOENT, e.Err)
	})
}
