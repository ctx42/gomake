// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// PathExists reports whether the path exists. Any Stat error, a permission
// error included, yields false.
func PathExists(pth string) bool {
	if _, err := os.Stat(pth); err != nil {
		return false
	}
	return true
}

// FileExists reports whether the path exists and is a regular file. A
// directory, socket, fifo, or device is not a regular file. Any Stat error
// yields false.
func FileExists(pth string) bool {
	fi, err := os.Stat(pth)
	if err != nil {
		return false
	}
	return fi.Mode().IsRegular()
}

// DirExists reports whether the path exists and is a directory. Any Stat
// error yields false.
func DirExists(pth string) bool {
	fi, err := os.Stat(pth)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// ReadFile is a wrapper around [os.ReadFile] returning a string instead of a
// byte slice.
func ReadFile(pth string) (string, error) {
	content, err := os.ReadFile(pth)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", pth, err)
	}
	return string(content), nil
}

// ReadChar reads one rune from the reader and returns it as a string. It
// returns [io.EOF] when the reader is empty. A second call continues at the
// next rune. Bytes that do not form a valid UTF-8 sequence are returned as
// read, so no input is lost: an invalid lead byte alone, and a lead byte with
// a malformed continuation together with the bytes read after it. A reader
// ending inside a multi-byte rune yields [io.ErrUnexpectedEOF] and drops the
// partial rune.
func ReadChar(r io.Reader) (string, error) {
	var buf [utf8.UTFMax]byte
	if _, err := io.ReadFull(r, buf[:1]); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return "", io.EOF
		}
		return "", fmt.Errorf("read rune: %w", err)
	}
	need := runeSize(buf[0])
	if need > 1 {
		_, err := io.ReadFull(r, buf[1:need])
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return "", io.ErrUnexpectedEOF
			}
			return "", fmt.Errorf("read rune: %w", err)
		}
	}
	return string(buf[:need]), nil
}

// ReadLine reads a line delimited by "\n" from the reader. Leading and
// trailing whitespace are trimmed from a successfully read line. On error,
// any partial content is also trimmed so EOF after a final line without a
// newline still yields the line text with [io.EOF]. A second call continues
// at the next line.
func ReadLine(r io.Reader) (string, error) {
	var buf bytes.Buffer
	tmp := make([]byte, 1)
	for {
		_, err := io.ReadFull(r, tmp)
		if err != nil {
			txt := strings.TrimSpace(buf.String())
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return txt, io.EOF
			}
			return txt, fmt.Errorf("read line: %w", err)
		}
		buf.WriteByte(tmp[0])
		if tmp[0] == '\n' {
			return strings.TrimSpace(buf.String()), nil
		}
	}
}
