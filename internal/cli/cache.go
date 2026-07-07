// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// binaryCacheDir returns the gomake binary cache directory, creating it
// if needed.
func binaryCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "gomake", "bin")
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// binaryCacheKey returns a hex-encoded SHA-256 hash over the given makefile
// source files in srcDir, the project's go.sum, gomake version, GOOS, and
// GOARCH. mkfNames are the validated makefile base names actually compiled, so
// the key matches the build exactly (ignored makefile_* files do not affect
// it).
func binaryCacheKey(
	srcDir string,
	mkfNames []string,
	version, goos, goarch string,
) (string, error) {

	h := sha256.New()

	// Hash makefile contents in sorted order for determinism.
	names := append([]string(nil), mkfNames...)
	sort.Strings(names)
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(h, "f:%s\n", name)
		_, _ = h.Write(data)
	}

	// Hash go.sum to catch dependency changes.
	if goSum := findGoSum(srcDir); goSum != "" {
		if data, err := os.ReadFile(goSum); err == nil {
			_, _ = fmt.Fprint(h, "go.sum\n")
			_, _ = h.Write(data)
		}
	}

	_, _ = fmt.Fprintf(h, "ver:%s\ngoos:%s\ngoarch:%s\n", version, goos, goarch)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// findGoSum returns the path to go.sum by locating go.mod walking up from
// srcDir. Returns empty string when not found.
func findGoSum(srcDir string) string {
	dir := srcDir
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			goSum := filepath.Join(dir, "go.sum")
			if _, err = os.Stat(goSum); err == nil {
				return goSum
			}
			return ""
		}
		dir = parent
	}
}

// lookupBinaryCache returns the path to a cached binary and true when a valid
// cached binary exists for the given source directory, version, and platform.
func lookupBinaryCache(
	srcDir string,
	mkfNames []string,
	version, goos, goarch string,
) (string, bool) {

	key, err := binaryCacheKey(srcDir, mkfNames, version, goos, goarch)
	if err != nil {
		return "", false
	}
	dir, err := binaryCacheDir()
	if err != nil {
		return "", false
	}
	name := key
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	pth := filepath.Join(dir, name)
	if _, err = os.Stat(pth); err != nil {
		return "", false
	}
	return pth, true
}

// storeBinaryCache copies the compiled binary at binPath into the cache.
// Errors are silently ignored because the cache is advisory.
func storeBinaryCache(
	binPath, srcDir string,
	mkfNames []string,
	version, goos, goarch string,
) {

	key, err := binaryCacheKey(srcDir, mkfNames, version, goos, goarch)
	if err != nil {
		return
	}
	dir, err := binaryCacheDir()
	if err != nil {
		return
	}
	name := key
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	_ = copyFile(binPath, filepath.Join(dir, name))
}
