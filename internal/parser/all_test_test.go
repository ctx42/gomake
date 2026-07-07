// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"go/ast"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/tester"
)

func Test_getFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t, 1)
		tspy.Close()

		fls := map[string]*ast.File{
			"dir/file0.go": {FileStart: 0},
			"dir/file1.go": {FileStart: 1},
		}

		// --- When ---
		have := getFile(tspy, fls, "dir", "file1.go")

		// --- Then ---
		assert.Equal(t, 1, int(have.FileStart))
	})

	t.Run("failure", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t, 1)
		tspy.ExpectError()
		wMsg := "" +
			"expected map to have a file:\n" +
			"  path: dir/file2.go"
		tspy.ExpectLogContain(wMsg)
		tspy.Close()

		fls := map[string]*ast.File{
			"dir/file0.go": {FileStart: 0},
			"dir/file1.go": {FileStart: 1},
		}

		// --- When ---
		have := getFile(tspy, fls, "dir", "file2.go")

		// --- Then ---
		assert.Nil(t, have)
	})
}
