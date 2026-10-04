// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/modkit"
)

func Test_astFiles(t *testing.T) {
	t.Run("error - empty directory", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/empty"
		absPath := modkit.Path(relPath)

		// --- When ---
		_, have, err := astFiles(absPath, goFilesIn(absPath))

		// --- Then ---
		assert.ErrorIs(t, ErrAstEmpty, err)
		assert.ErrorContain(t, absPath, err)
		assert.Nil(t, have)
	})

	t.Run("error - missing file", func(t *testing.T) {
		// --- Given ---
		absPath := modkit.Path("testdata/projects/not_existing")

		// --- When ---
		_, have, err := astFiles(absPath, []string{"makefile.go"})

		// --- Then ---
		assert.ErrorIs(t, errAstParse, err)
		assert.ErrorContain(t, absPath, err)
		assert.Nil(t, have)
	})

	t.Run("tagged files", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/simple_tagged/project"
		absPath := modkit.Path(relPath)

		// --- When ---
		_, have, err := astFiles(absPath, goFilesIn(absPath))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "main", pkgName(have))
		assert.Len(t, 1, have)
		fil := getFile(t, have, absPath, "makefile.go")
		assert.Equal(t, "main", fil.Name.Name)
	})

	t.Run("file list", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/simple_tagged/project"
		absPath := modkit.Path(relPath)
		files := []string{"makefile.go"}

		// --- When ---
		_, have, err := astFiles(absPath, files)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "main", pkgName(have))
		assert.Len(t, 1, have)
		fil := getFile(t, have, absPath, "makefile.go")
		assert.Equal(t, "main", fil.Name.Name)
	})

	t.Run("error - empty file list does not scan dir", func(t *testing.T) {
		// --- Given ---
		// Directory has makefile.go; an explicit empty list must not fall
		// back to scanning the directory (build-tag / go-list fidelity).
		relPath := "testdata/projects/simple_untagged/project"
		absPath := modkit.Path(relPath)
		files := []string{}

		// --- When ---
		_, have, err := astFiles(absPath, files)

		// --- Then ---
		assert.ErrorIs(t, ErrAstEmpty, err)
		assert.ErrorContain(t, absPath, err)
		assert.Nil(t, have)
	})

	t.Run("untagged files", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/simple_untagged/project"
		absPath := modkit.Path(relPath)

		// --- When ---
		_, have, err := astFiles(absPath, goFilesIn(absPath))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "main", pkgName(have))
		assert.Len(t, 1, have)
		fil := getFile(t, have, absPath, "makefile.go")
		assert.Equal(t, "main", fil.Name.Name)
	})

	t.Run("multiple ast packages error", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/packages/multi"
		absPath := modkit.Path(relPath)

		// --- When ---
		_, have, err := astFiles(absPath, goFilesIn(absPath))

		// --- Then ---
		assert.ErrorIs(t, errAstMultiPkg, err)
		assert.ErrorContain(t, absPath, err)
		assert.ErrorContain(t, "main, multi", err)
		assert.Nil(t, have)
	})
}

func Test_astAndDocPkg(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/simple_untagged/project"
		absPath := modkit.Path(relPath)

		// --- When ---
		hAstPkg, hDocPkg, err := astAndDocPkg(absPath, goFilesIn(absPath))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "main", pkgName(hAstPkg))
		assert.Len(t, 1, hAstPkg)

		getFile(t, hAstPkg, absPath, "makefile.go")

		assert.Equal(t, "main", hDocPkg.Name)
		assert.Equal(t, absPath, hDocPkg.ImportPath)
	})

	t.Run("error", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/packages/multi"
		absPath := modkit.Path(relPath)

		// --- When ---
		hAstPkg, hDocPkg, err := astAndDocPkg(absPath, goFilesIn(absPath))

		// --- Then ---
		assert.ErrorIs(t, errAstMultiPkg, err)
		assert.Nil(t, hAstPkg)
		assert.Nil(t, hDocPkg)
	})
}
