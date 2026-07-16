// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/modkit"
)

func Test_astFiles(t *testing.T) {
	t.Run("empty directory", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/empty"
		absPath := modkit.Path(relPath)

		// --- When ---
		_, pkg, err := astFiles(absPath, nil)

		// --- Then ---
		assert.ErrorIs(t, ErrAstEmpty, err)
		assert.ErrorContain(t, absPath, err)
		assert.Nil(t, pkg)
	})

	t.Run("not existing directory", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/not_existing"
		absPath := modkit.Path(relPath)

		// --- When ---
		_, pkg, err := astFiles(absPath, nil)

		// --- Then ---
		assert.ErrorIs(t, errAstParse, err)
		assert.ErrorContain(t, absPath, err)
		assert.Nil(t, pkg)
	})

	t.Run("tagged files", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/simple_tagged/project"
		absPath := modkit.Path(relPath)

		// --- When ---
		_, pkg, err := astFiles(absPath, nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "main", pkgName(pkg))
		assert.Len(t, 1, pkg)

		fil := getFile(t, pkg, absPath, "makefile.go")
		assert.Equal(t, "main", fil.Name.Name)
	})

	t.Run("file list", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/simple_tagged/project"
		absPath := modkit.Path(relPath)
		files := []string{"makefile.go"}

		// --- When ---
		_, pkg, err := astFiles(absPath, files)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "main", pkgName(pkg))
		assert.Len(t, 1, pkg)

		fil := getFile(t, pkg, absPath, "makefile.go")
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
		_, pkg, err := astFiles(absPath, files)

		// --- Then ---
		assert.ErrorIs(t, ErrAstEmpty, err)
		assert.ErrorContain(t, absPath, err)
		assert.Nil(t, pkg)
	})

	t.Run("untagged files", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/simple_untagged/project"
		absPath := modkit.Path(relPath)

		// --- When ---
		_, pkg, err := astFiles(absPath, nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "main", pkgName(pkg))
		assert.Len(t, 1, pkg)

		fil := getFile(t, pkg, absPath, "makefile.go")
		assert.Equal(t, "main", fil.Name.Name)
	})

	t.Run("multiple ast packages error", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/packages/multi"
		absPath := modkit.Path(relPath)

		// --- When ---
		_, pkg, err := astFiles(absPath, nil)

		// --- Then ---
		assert.ErrorIs(t, errAstMultiPkg, err)
		assert.ErrorContain(t, absPath, err)
		assert.ErrorContain(t, "main, multi", err)
		assert.Nil(t, pkg)
	})
}

func Test_astAndDocPkg(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/simple_untagged/project"
		absPath := modkit.Path(relPath)

		// --- When ---
		astPkg, docPkg, err := astAndDocPkg(absPath, nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "main", pkgName(astPkg))
		assert.Len(t, 1, astPkg)
		getFile(t, astPkg, absPath, "makefile.go")
		assert.Equal(t, "main", docPkg.Name)
		assert.Equal(t, absPath, docPkg.ImportPath)
	})

	t.Run("error", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/packages/multi"
		absPath := modkit.Path(relPath)

		// --- When ---
		astPkg, docPkg, err := astAndDocPkg(absPath, nil)

		// --- Then ---
		assert.ErrorIs(t, errAstMultiPkg, err)
		assert.Nil(t, astPkg)
		assert.Nil(t, docPkg)
	})
}
