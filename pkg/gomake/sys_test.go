// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
)

func Test_PathExists(t *testing.T) {
	t.Run("path does not exist", func(t *testing.T) {
		// --- When ---
		have := PathExists("testdata/not-existing")

		// --- Then ---
		assert.False(t, have)
	})

	t.Run("file", func(t *testing.T) {
		// --- When ---
		have := PathExists("testdata/file0.txt")

		// --- Then ---
		assert.True(t, have)
	})

	t.Run("link", func(t *testing.T) {
		// --- When ---
		have := PathExists("testdata/file0_link.txt")

		// --- Then ---
		assert.True(t, have)
	})

	t.Run("directory", func(t *testing.T) {
		// --- When ---
		have := PathExists("testdata/dir")

		// --- Then ---
		assert.True(t, have)
	})
}

func Test_FileExists(t *testing.T) {
	t.Run("file does not exist", func(t *testing.T) {
		// --- When ---
		have := FileExists("testdata/not-existing")

		// --- Then ---
		assert.False(t, have)
	})

	t.Run("file", func(t *testing.T) {
		// --- When ---
		have := FileExists("testdata/file0.txt")

		// --- Then ---
		assert.True(t, have)
	})

	t.Run("link", func(t *testing.T) {
		// --- When ---
		have := FileExists("testdata/file0_link.txt")

		// --- Then ---
		assert.True(t, have)
	})

	t.Run("directory", func(t *testing.T) {
		// --- When ---
		have := FileExists("testdata/dir")

		// --- Then ---
		assert.False(t, have)
	})

	t.Run("socket", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		pth := filepath.Join(dir, "sock")
		ln := must.Value(net.Listen("unix", pth))
		t.Cleanup(func() { _ = ln.Close() })

		// --- When ---
		have := FileExists(pth)

		// --- Then ---
		assert.False(t, have)
	})
}

func Test_DirExists(t *testing.T) {
	t.Run("directory does not exist", func(t *testing.T) {
		// --- When ---
		have := DirExists("testdata/not-existing")

		// --- Then ---
		assert.False(t, have)
	})

	t.Run("file", func(t *testing.T) {
		// --- When ---
		have := DirExists("testdata/file0.txt")

		// --- Then ---
		assert.False(t, have)
	})

	t.Run("link", func(t *testing.T) {
		// --- When ---
		have := DirExists("testdata/file0_link.txt")

		// --- Then ---
		assert.False(t, have)
	})

	t.Run("directory", func(t *testing.T) {
		// --- When ---
		have := DirExists("testdata/dir")

		// --- Then ---
		assert.True(t, have)
	})
}

func Test_ReadFile(t *testing.T) {
	t.Run("does not exist", func(t *testing.T) {
		// --- When ---
		have, err := ReadFile("testdata/not-existing")

		// --- Then ---
		assert.ErrorIs(t, os.ErrNotExist, err)
		assert.Equal(t, "", have)
	})

	t.Run("file", func(t *testing.T) {
		// --- When ---
		have, err := ReadFile("testdata/file0.txt")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "content", have)
	})

	t.Run("link", func(t *testing.T) {
		// --- When ---
		have, err := ReadFile("testdata/file0_link.txt")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "content", have)
	})

	t.Run("directory", func(t *testing.T) {
		// --- When ---
		have, err := ReadFile("testdata/dir")

		// --- Then ---
		assert.ErrorContain(t, "directory", err)
		assert.Equal(t, "", have)
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

	t.Run("second call keeps the next rune", func(t *testing.T) {
		// --- Given ---
		r := bytes.NewReader([]byte("ab"))
		must.Value(ReadChar(r))

		// --- When ---
		have, err := ReadChar(r)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "b", have)
	})

	t.Run("multi-byte rune", func(t *testing.T) {
		// --- Given ---
		r := bytes.NewReader([]byte("é"))

		// --- When ---
		have, err := ReadChar(r)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "é", have)
	})

	t.Run("invalid continuation", func(t *testing.T) {
		// --- When ---
		have, err := ReadChar(bytes.NewReader([]byte("\xC3A")))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "\xC3A", have)
	})

	t.Run("invalid lead byte", func(t *testing.T) {
		// --- When ---
		have, err := ReadChar(bytes.NewReader([]byte("\x80b")))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "\x80", have)
	})

	t.Run("error - truncated rune", func(t *testing.T) {
		// --- When ---
		have, err := ReadChar(bytes.NewReader([]byte("\xC3")))

		// --- Then ---
		assert.ErrorIs(t, io.ErrUnexpectedEOF, err)
		assert.Equal(t, "", have)
	})

	t.Run("error - reader", func(t *testing.T) {
		// --- When ---
		have, err := ReadChar(iotest.ErrReader(errTest))

		// --- Then ---
		assert.ErrorIs(t, errTest, err)
		assert.ErrorContain(t, "read rune: ", err)
		assert.Equal(t, "", have)
	})

	t.Run("error - reader inside rune", func(t *testing.T) {
		// --- Given ---
		rdr := io.MultiReader(
			bytes.NewReader([]byte("\xC3")),
			iotest.ErrReader(errTest),
		)

		// --- When ---
		have, err := ReadChar(rdr)

		// --- Then ---
		assert.ErrorIs(t, errTest, err)
		assert.Equal(t, "", have)
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

	t.Run("second call keeps the next line", func(t *testing.T) {
		// --- Given ---
		rdr := bytes.NewBufferString("one\ntwo\n")
		must.Value(ReadLine(rdr))

		// --- When ---
		have, err := ReadLine(rdr)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "two", have)
	})

	t.Run("error - eof without newline", func(t *testing.T) {
		// --- Given ---
		rdr := bytes.NewBufferString("  last  ")

		// --- When ---
		have, err := ReadLine(rdr)

		// --- Then ---
		assert.ErrorIs(t, io.EOF, err)
		assert.Equal(t, "last", have)
	})

	t.Run("error - reader", func(t *testing.T) {
		// --- Given ---
		rdr := io.MultiReader(
			bytes.NewReader([]byte(" ab")),
			iotest.ErrReader(errTest),
		)

		// --- When ---
		have, err := ReadLine(rdr)

		// --- Then ---
		assert.ErrorIs(t, errTest, err)
		assert.ErrorContain(t, "read line: ", err)
		assert.Equal(t, "ab", have)
	})
}

func Test_ReadLine_tabular(t *testing.T) {
	tt := []struct {
		testN string

		in   string
		want string
	}{
		{"newline", "abc\n", "abc"},
		{"trailing blank line", "abc\n\n", "abc"},
		{"leading space", " abc\n\n", "abc"},
		{"leading blank line", "\n abc\n", ""},
		{"first line only", "abc\ndef\n", "abc"},
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
