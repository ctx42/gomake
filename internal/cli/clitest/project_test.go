// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package clitest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"
)

// wantMkfContent is the content of a testdata makefile after
// [Project.MakefilesFrom] strips the "go:build gomake" directive.
const wantMkfContent = "" +
	"// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac\n" +
	"// SPDX-License-Identifier: MIT\n" +
	"\n" +
	"package testdata\n"

func Test_NewProject(t *testing.T) {
	t.Run("no options", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.Close()

		// --- When ---
		have := NewProject(tspy)

		// --- Then ---
		assert.NotNil(t, have.hidPrj)
		assert.NotEmpty(t, have.ringVer)
		assert.Same(t, tspy, have.t)

		have.Close() // Must close to prevent error.
	})

	t.Run("error - not closed at test end", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.ExpectError()
		tspy.ExpectLogEqual("expected instance to be closed at the test end")
		tspy.Close()

		// --- When ---
		have := NewProject(tspy)

		// --- Then ---
		assert.NotNil(t, have)

		tspy.Finish()
	})
}

func Test_Project_MakefilesFrom(t *testing.T) {
	t.Run("on opened", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.Close()

		prj := NewProject(tspy)

		testdata := "testdata"

		// --- When ---
		prj.MakefilesFrom(testdata)

		// --- Then ---
		wantLs := []string{"makefile.go", "makefile_helpers.go"}
		assert.Equal(t, wantLs, oskit.List(t, prj.Root()))

		haveContent := oskit.ReadFileStr(t, prj.Root(), "makefile.go")
		assert.Equal(t, wantMkfContent, haveContent)

		haveContent = oskit.ReadFileStr(t, prj.Root(), "makefile_helpers.go")

		assert.Equal(t, wantMkfContent, haveContent)

		prj.Close() // Must close to prevent error.
	})

	t.Run("overwritten", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.Close()

		prj := NewProject(tspy)
		oskit.Write(t, "not go code", prj.Root(), "makefile.go")
		oskit.Write(t, "not go code", prj.Root(), "makefile_helpers.go")

		testdata := "testdata"

		// --- When ---
		have := prj.MakefilesFrom(testdata)

		// --- Then ---
		wantLs := []string{
			"makefile.go",
			"makefile_helpers.go",
		}
		assert.Equal(t, wantLs, oskit.List(t, prj.Root()))

		wantLs = []string{
			filepath.Join(prj.Root(), "makefile.go"),
			filepath.Join(prj.Root(), "makefile_helpers.go"),
		}

		assert.Equal(t, wantLs, have)

		haveContent := oskit.ReadFileStr(t, prj.Root(), "makefile.go")
		assert.Equal(t, wantMkfContent, haveContent)

		haveContent = oskit.ReadFileStr(t, prj.Root(), "makefile_helpers.go")

		assert.Equal(t, wantMkfContent, haveContent)

		prj.Close() // Must close to prevent error.
	})

	t.Run("no blank line after tag", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.Close()

		prj := NewProject(tspy)

		src := oskit.MkdirAll(t, t.TempDir(), "src")

		body := "" +
			"// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac\n" +
			"// SPDX-License-Identifier: MIT\n" +
			"\n" +
			"//go:build gomake\n" +
			"package testdata\n"
		oskit.Write(t, body, src, "makefile.go")

		// --- When ---
		prj.MakefilesFrom(src)

		// --- Then ---
		text := oskit.ReadFileStr(t, prj.Root(), "makefile.go")
		assert.Equal(t, wantMkfContent, text)

		prj.Close() // Must close to prevent error.
	})

	t.Run("error - used more than once", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.ExpectError()
		tspy.ExpectFatal()

		wMsg := "" +
			"the MakefilesFrom method can be used only once\n" +
			"expected instance to be closed at the test end"
		tspy.ExpectLogEqual(wMsg)

		tspy.Close()

		prj := NewProject(tspy)
		prj.MakefilesFrom("testdata")

		fn := func() { prj.MakefilesFrom("testdata") }

		// --- When ---
		assert.Panic(t, fn)
	})

	t.Run("error - on closed", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.ExpectFatal()
		tspy.ExpectLogEqual("expected test project instance to be open")
		tspy.Close()

		prj := NewProject(tspy)
		prj.Close()

		fn := func() { prj.MakefilesFrom("testdata") }

		// --- When ---
		assert.Panic(t, fn)
	})

	t.Run("error - unreadable makefile source", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.ExpectFatal()
		tspy.ExpectLogContain("no such file or directory")
		tspy.Close()

		prj := NewProject(tspy)

		src := oskit.MkdirAll(t, t.TempDir(), "src")

		dst := filepath.Join(src, "makefile.go")
		must.Nil(os.Symlink(filepath.Join(src, "missing.go"), dst))

		fn := func() { prj.MakefilesFrom(src) }

		// --- When ---
		assert.Panic(t, fn)

		// --- Then ---
		assert.NoFileExist(t, filepath.Join(prj.Root(), "makefile.go"))

		prj.Close() // Must close to prevent error.
	})
}

func Test_Project_UseGomakeSrc(t *testing.T) {
	t.Run("on opened", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.Close()

		prj := NewProject(tspy)
		prj.GoModInit()

		selfDir := modkit.Root()

		// --- When ---
		prj.UseGomakeSrc(selfDir)

		// --- Then ---
		want := "github.com/ctx42/gomake v0.0.0"
		pth := filepath.Join(prj.Root(), "go.mod")
		assert.FileContain(t, want, pth)

		want = "github.com/ctx42/ring " + prj.ringVer

		assert.FileContain(t, want, pth)

		want = "replace github.com/ctx42/gomake v0.0.0 => " + selfDir

		assert.FileContain(t, want, pth)

		prj.Close() // Must close to prevent error.
	})

	t.Run("error - on closed", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectTempDir(1)
		tspy.ExpectCleanups(1)
		tspy.ExpectFatal()
		tspy.ExpectLogEqual("expected test project instance to be open")
		tspy.Close()

		prj := NewProject(tspy)
		prj.Close()

		fn := func() { prj.UseGomakeSrc("/dir/a/b") }

		// --- When ---
		assert.Panic(t, fn)
	})
}

func Test_Project_RequireXflag(t *testing.T) {
	t.Run("on opened", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.Close()

		prj := NewProject(tspy)
		prj.GoModInit()

		// --- When ---
		prj.RequireXflag()

		// --- Then ---
		want := "github.com/ctx42/xflag " + prj.xflagVer
		pth := filepath.Join(prj.Root(), "go.mod")
		assert.FileContain(t, want, pth)

		prj.Close() // Must close to prevent error.
	})

	t.Run("error - on closed", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectTempDir(1)
		tspy.ExpectCleanups(1)
		tspy.ExpectFatal()
		tspy.ExpectLogEqual("expected test project instance to be open")
		tspy.Close()

		prj := NewProject(tspy)
		prj.Close()

		fn := func() { prj.RequireXflag() }

		// --- When ---
		assert.Panic(t, fn)
	})
}
