// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"bufio"
	"io"
	"os"
	"strings"
)

// PathExists returns true if the path exists.
func PathExists(pth string) bool {
	if _, err := os.Stat(pth); err != nil {
		return false
	}
	return true
}

// FileExists returns true if the path exists and is a file.
func FileExists(pth string) bool {
	fi, err := os.Stat(pth)
	if err != nil {
		return false
	}
	return !fi.IsDir()
}

// DirExists returns true if the path exists and is a directory.
func DirExists(pth string) bool {
	fi, err := os.Stat(pth)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// ReadFile is a wrapper around [os.ReadFile] returning string instead of byte
// slice.
func ReadFile(pth string) (string, error) {
	content, err := os.ReadFile(pth)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// ReadChar reads one rune from the reader and returns it as a string. It
// returns [io.EOF] when the reader is empty.
func ReadChar(r io.Reader) (string, error) {
	char, _, err := bufio.NewReader(r).ReadRune()
	if err != nil {
		return "", err
	}
	return string(char), nil
}

// ReadLine reads line delimited by "\n" from the reader. The prefix and
// postfix whitespace are trimmed from the returned line.
func ReadLine(r io.Reader) (string, error) {
	txt, err := bufio.NewReader(r).ReadString('\n')
	if err != nil {
		return txt, err
	}
	return strings.TrimSpace(txt), nil
}
