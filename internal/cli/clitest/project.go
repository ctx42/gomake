// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package clitest helps gomake integration tests set up temporary
// projects, copy makefiles, and align module dependencies with the tool
// under test.
package clitest

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"
)

// GmModName represents gomake import spec.
const GmModName = "github.com/ctx42/gomake"

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
// directory basename is "project" and the module path is [GmModName].
func NewProject(t tester.T, opts ...func(*prjkit.Project)) *Project {
	t.Helper()

	dir := oskit.MkdirAll(t, t.TempDir(), "project")
	prj := &Project{
		hidPrj: prjkit.New(t, dir, opts...),
		t:      t,
	}
	modPth := filepath.Join(modkit.Root(), "go.mod")
	prj.ringVer = must.Value(modkit.ModVer(modPth, "github.com/ctx42/ring"))
	prj.xflagVer = must.Value(modkit.ModVer(modPth, "github.com/ctx42/xflag"))
	return prj
}

// MakefilesFrom copies makefiles from src to project root. The `go:build
// gomake` directives if present in the makefiles will be removed. It can be
// called only once.
func (prj *Project) MakefilesFrom(src string) []string {
	prj.t.Helper()
	prj.CheckOpen()

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
			continue
		}
		switch {
		case bytes.HasPrefix(filData, tagLine):
			filData = bytes.TrimLeft(filData[len(tagLine):], "\n")
		default:
			block := append([]byte{'\n'}, append(tagLine, '\n')...)
			if i := bytes.Index(filData, block); i >= 0 {
				filData = append(filData[:i+1], filData[i+len(block):]...)
			}
		}
		dstPth := filepath.Join(prj.Root(), filepath.Base(srcPth))
		if err = os.WriteFile(dstPth, filData, 0600); err != nil {
			prj.t.Fatal(err)
		}
		copied = append(copied, dstPth)
	}
	return copied
}

// UseGomakeSrc edits test project's "go.mod" file and replaces
// "github.com/ctx42/gomake" imports with the source on the disk at src.
func (prj *Project) UseGomakeSrc(src string) {
	prj.t.Helper()
	prj.CheckOpen()

	gmPth := "github.com/ctx42/gomake@v0.0.0"
	ringPth := "github.com/ctx42/ring@" + prj.ringVer
	prj.Exe("go", "mod", "edit", "-require="+ringPth)
	prj.Exe("go", "mod", "edit", "-require="+gmPth)
	prj.Exe("go", "mod", "edit", "-replace="+gmPth+"="+src)
}

// RequireXflag adds the xflag module requirement and records its checksum so
// the generated makefile, which imports xflag, compiles in the test project.
// It mirrors what gomake injects into the build directory's "go.mod" and must
// be called after any "go mod tidy" that would otherwise drop the unused
// requirement.
func (prj *Project) RequireXflag() {
	prj.t.Helper()
	prj.CheckOpen()

	xflagPth := "github.com/ctx42/xflag@" + prj.xflagVer
	prj.Exe("go", "mod", "edit", "-require="+xflagPth)
	prj.Exe("go", "mod", "download", xflagPth)
}
