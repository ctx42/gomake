// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package clitest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"
)

type hidPrj = prjkit.Project // Hide embedded field.

// Project represents test gomake projects.
type Project struct {
	*hidPrj           // Embed project helper.
	mkfFrom  string   // Absolute path to makefile sources.
	ringVer  string   // The ring module version used by gomake.
	xflagVer string   // The xflag module version used by gomake.
	t        tester.T // Test manager.
}

// NewProject creates a temporary directory for a project. By default the
// directory basename is "project" and the module path is
// "example.com/comp/project". It fails the test at once when the test has
// already failed, since no project can be created then.
func NewProject(t tester.T, opts ...func(*prjkit.Project)) *Project {
	t.Helper()

	modPth := filepath.Join(modkit.Root(), "go.mod")
	ringVer, err := modkit.ModVer(modPth, "github.com/ctx42/ring")
	if err != nil {
		t.Fatal(err)
		return nil
	}
	xflagVer, err := modkit.ModVer(modPth, "github.com/ctx42/xflag")
	if err != nil {
		t.Fatal(err)
		return nil
	}

	dir := oskit.MkdirAll(t, t.TempDir(), "project")
	hid := prjkit.New(t, dir, opts...)
	if hid == nil { // prjkit.New refuses to run in a failed test.
		t.Fatal("cannot create a project in a failed test")
		return nil
	}
	return &Project{hidPrj: hid, ringVer: ringVer, xflagVer: xflagVer, t: t}
}

// MakefilesFrom copies makefiles from src to the project root, removing the
// `go:build gomake` directive, if present, from each. It can be called only
// once.
func (prj *Project) MakefilesFrom(src string) []string {
	prj.t.Helper()
	_ = prj.CheckOpen()

	if prj.mkfFrom != "" {
		prj.t.Fatal("the MakefilesFrom method can be used only once")
		return nil
	}
	prj.mkfFrom = src
	var err error
	tagLine := []byte("//go:build gomake\n")
	copied := make([]string, 0, 10)
	for _, srcPth := range findMakefiles(prj.t, prj.mkfFrom) {
		var filData []byte
		srcPth = filepath.Join(prj.mkfFrom, srcPth)
		if filData, err = os.ReadFile(srcPth); err != nil {
			prj.t.Fatal(err)
			return nil
		}
		// Normalize CRLF so LF-only tag matching works on Windows sources.
		filData = bytes.ReplaceAll(filData, []byte("\r\n"), []byte("\n"))
		switch {
		case bytes.HasPrefix(filData, tagLine):
			filData = bytes.TrimLeft(filData[len(tagLine):], "\n")

		default:
			// Drop the tag line. A following blank line is kept as one
			// newline so the result matches a tag that already had one.
			// Build constraints precede the package clause, so the search
			// stops there and never touches the same text in the body.
			line := append([]byte{'\n'}, tagLine...)
			header := filData
			nlData := append([]byte{'\n'}, filData...)
			if end := bytes.Index(nlData, []byte("\npackage ")); end >= 0 {
				header = filData[:end]
			}
			i := bytes.Index(header, line)
			if i < 0 {
				break
			}
			rest := filData[i+len(line):]
			if len(rest) > 0 && rest[0] == '\n' {
				rest = rest[1:]
			}
			filData = append(filData[:i+1], rest...)
		}
		dstPth := filepath.Join(prj.Root(), filepath.Base(srcPth))
		if err = os.WriteFile(dstPth, filData, 0600); err != nil {
			prj.t.Fatal(err)
			return nil
		}
		copied = append(copied, dstPth)
	}
	return copied
}

// UseGomakeSrc edits test project's "go.mod" file and replaces
// "github.com/ctx42/gomake" imports with the source on the disk at src, an
// absolute path. Call it after [prjkit.Project.GoModInit]; a failed edit
// fails the test at once.
func (prj *Project) UseGomakeSrc(src string) {
	prj.t.Helper()
	_ = prj.CheckOpen()

	gmPth := "github.com/ctx42/gomake@v0.0.0"
	ringPth := "github.com/ctx42/ring@" + prj.ringVer
	prj.mustExe("go", "mod", "edit", "-require="+ringPth)
	prj.mustExe("go", "mod", "edit", "-require="+gmPth)
	prj.mustExe("go", "mod", "edit", "-replace="+gmPth+"="+src)
}

// RequireXflag adds the xflag module requirement and records its checksum so
// the generated makefile, which imports xflag, compiles in the test project.
// It mirrors what gomake injects into the build directory's "go.mod" and must
// be called after any "go mod tidy" that would otherwise drop the unused
// requirement. A failed command fails the test at once.
func (prj *Project) RequireXflag() {
	prj.t.Helper()
	_ = prj.CheckOpen()

	xflagPth := "github.com/ctx42/xflag@" + prj.xflagVer
	prj.mustExe("go", "mod", "edit", "-require="+xflagPth)
	prj.mustExe("go", "mod", "download", xflagPth)
}

// mustExe runs cmd in the project and fails the test at once when the
// command fails, so later steps never run against a half-edited go.mod.
func (prj *Project) mustExe(cmd string, args ...string) {
	prj.t.Helper()
	failed := prj.t.Failed()
	_, _ = prj.Exe(cmd, args...)
	if !failed && prj.t.Failed() {
		line := strings.Join(append([]string{cmd}, args...), " ")
		prj.t.Fatal("command failed: " + line)
	}
}
