// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"bytes"
	"io"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
)

func Test_PathExists(t *testing.T) {
	t.Run("path does not exist", func(t *testing.T) {
		// --- When ---
		exists := PathExists("testdata/not-existing")

		// --- Then ---
		assert.False(t, exists)
	})

	t.Run("file", func(t *testing.T) {
		// --- When ---
		exists := PathExists("testdata/file0.txt")

		// --- Then ---
		assert.True(t, exists)
	})

	t.Run("link", func(t *testing.T) {
		// --- When ---
		exists := PathExists("testdata/file0_link.txt")

		// --- Then ---
		assert.True(t, exists)
	})

	t.Run("directory", func(t *testing.T) {
		// --- When ---
		exists := PathExists("testdata/dir")

		// --- Then ---
		assert.True(t, exists)
	})
}

func Test_FileExists(t *testing.T) {
	t.Run("file does not exist", func(t *testing.T) {
		// --- When ---
		exists := FileExists("testdata/not-existing")

		// --- Then ---
		assert.False(t, exists)
	})

	t.Run("file", func(t *testing.T) {
		// --- When ---
		exists := FileExists("testdata/file0.txt")

		// --- Then ---
		assert.True(t, exists)
	})

	t.Run("link", func(t *testing.T) {
		// --- When ---
		exists := FileExists("testdata/file0_link.txt")

		// --- Then ---
		assert.True(t, exists)
	})

	t.Run("directory", func(t *testing.T) {
		// --- When ---
		exists := FileExists("testdata/dir")

		// --- Then ---
		assert.False(t, exists)
	})
}

func Test_DirExists(t *testing.T) {
	t.Run("directory does not exist", func(t *testing.T) {
		// --- When ---
		exists := DirExists("testdata/not-existing")

		// --- Then ---
		assert.False(t, exists)
	})

	t.Run("file", func(t *testing.T) {
		// --- When ---
		exists := DirExists("testdata/file0.txt")

		// --- Then ---
		assert.False(t, exists)
	})

	t.Run("link", func(t *testing.T) {
		// --- When ---
		exists := DirExists("testdata/file0_link.txt")

		// --- Then ---
		assert.False(t, exists)
	})

	t.Run("directory", func(t *testing.T) {
		// --- When ---
		exists := DirExists("testdata/dir")

		// --- Then ---
		assert.True(t, exists)
	})
}

func Test_ReadFile(t *testing.T) {
	t.Run("does not exist", func(t *testing.T) {
		// --- When ---
		content, err := ReadFile("testdata/not-existing")

		// --- Then ---
		assert.Error(t, err)
		assert.Equal(t, "", content)
	})

	t.Run("file", func(t *testing.T) {
		// --- When ---
		content, err := ReadFile("testdata/file0.txt")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "content", content)

	})

	t.Run("link", func(t *testing.T) {
		// --- When ---
		content, err := ReadFile("testdata/file0_link.txt")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "content", content)
	})

	t.Run("directory", func(t *testing.T) {
		// --- When ---
		content, err := ReadFile("testdata/dir")

		// --- Then ---
		assert.Error(t, err)
		assert.Equal(t, "", content)
	})
}

func Test_ReadChar(t *testing.T) {
	t.Run("empty reader", func(t *testing.T) {
		// --- Given ---
		r := bytes.NewReader(nil)

		// --- When ---
		have, err := ReadChar(r)

		// --- Then ---
		assert.ErrorIs(t, io.EOF, err)
		assert.Equal(t, "", have)
	})

	t.Run("read ok", func(t *testing.T) {
		// --- Given ---
		r := bytes.NewReader([]byte{'a', 'b', 'c'})

		// --- When ---
		have, err := ReadChar(r)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "a", have)
	})
}

func Test_ReadLine(t *testing.T) {
	t.Run("empty reader", func(t *testing.T) {
		// --- Given ---
		rdr := bytes.NewBufferString("")

		// --- When ---
		have, err := ReadLine(rdr)

		// --- Then ---
		assert.ErrorIs(t, io.EOF, err)
		assert.Equal(t, "", have)
	})
}

func Test_ReadLine_tabular(t *testing.T) {
	tt := []struct {
		testN string

		in   string
		want string
	}{
		{"1", "abc\n", "abc"},
		{"2", "abc\n\n", "abc"},
		{"3", " abc\n\n", "abc"},
		{"4", "\n abc\n", ""},
		{"5", "abc\ndef\n", "abc"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			rdr := bytes.NewBufferString(tc.in)

			// --- When ---
			have, err := ReadLine(rdr)

			// --- Then ---
			assert.NoError(t, err)
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_ReadLine_eofWithoutNewline(t *testing.T) {
	// --- Given ---
	rdr := bytes.NewBufferString("  last  ")

	// --- When ---
	have, err := ReadLine(rdr)

	// --- Then ---
	assert.ErrorIs(t, io.EOF, err)
	assert.Equal(t, "last", have)
}
