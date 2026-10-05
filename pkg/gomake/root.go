// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Root returns the absolute path to a module root directory, found by walking
// up from pth until a "go.mod" file is located; the optional elem segments are
// appended to the result. On failure it returns an empty string and an error,
// which wraps [ErrNoGoMod] when no "go.mod" file exists above pth.
func Root(pth string, elem ...string) (string, error) {
	pth, err := filepath.Abs(pth)
	if err != nil {
		return "", fmt.Errorf("gomake: root: %w", err)
	}
	start := pth
	for {
		// Only a regular file counts; a directory named go.mod does not.
		fi, err := os.Stat(filepath.Join(pth, "go.mod"))
		if err == nil && fi.Mode().IsRegular() {
			break
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("gomake: root: %w", err)
		}
		parent := filepath.Dir(pth)
		if parent == pth {
			format := "gomake: root: %w starting at %s"
			return "", fmt.Errorf(format, ErrNoGoMod, start)
		}
		pth = parent
	}
	return filepath.Join(append([]string{pth}, elem...)...), nil
}
