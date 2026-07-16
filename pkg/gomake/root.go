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

// ErrNoGoMod is returned when [Root] cannot find the project root directory.
var ErrNoGoMod = errors.New("cannot find \"go.mod\" file")

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
		_, err := os.Stat(filepath.Join(pth, "go.mod"))
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("gomake: root: %w", err)
		}
		parent := filepath.Dir(pth)
		if parent == pth {
			return "", fmt.Errorf("%w starting at %s", ErrNoGoMod, start)
		}
		pth = parent
	}
	return filepath.Join(append([]string{pth}, elem...)...), nil
}
