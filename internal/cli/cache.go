// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
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

// binaryCacheKey returns a hex-encoded SHA-256 hash over the inputs that
// affect the compiled makefile binary: the makefile sources actually built,
// the module's go.mod / go.sum / go.work / go.work.sum when present, every
// non-test .go file under the module root (skipping vendor and VCS dirs),
// and the gomake version plus GOOS/GOARCH. mkfNames are the validated
// makefile base names actually compiled, so ignored makefile_* files do not
// affect the key.
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

	modRoot := findModuleRoot(srcDir)
	if modRoot != "" {
		if err := hashFile(h, "go.mod", filepath.Join(modRoot, "go.mod")); err != nil {
			return "", err
		}
		// Optional files: missing is fine; read errors are not.
		for _, name := range []string{"go.sum", "go.work", "go.work.sum"} {
			pth := filepath.Join(modRoot, name)
			if _, err := os.Stat(pth); err != nil {
				continue
			}
			if err := hashFile(h, name, pth); err != nil {
				return "", err
			}
		}
		if err := hashModuleGoFiles(h, modRoot); err != nil {
			return "", err
		}
	}

	_, _ = fmt.Fprintf(h, "ver:%s\ngoos:%s\ngoarch:%s\n", version, goos, goarch)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashFile writes a labeled file into h. Returns an error only when the file
// exists but cannot be read.
func hashFile(h interface{ Write([]byte) (int, error) }, label, pth string) error {
	data, err := os.ReadFile(pth)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(h, "%s\n", label)
	_, _ = h.Write(data)
	return nil
}

// hashModuleGoFiles hashes every non-test .go file under modRoot, in sorted
// relative-path order. Skips vendor, module cache, and VCS directories so the
// key tracks local package sources the makefile may import via replace.
func hashModuleGoFiles(h interface{ Write([]byte) (int, error) }, modRoot string) error {
	var paths []string
	err := filepath.WalkDir(modRoot, func(pth string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if shouldSkipCacheDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		paths = append(paths, pth)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(paths)
	for _, pth := range paths {
		rel, err := filepath.Rel(modRoot, pth)
		if err != nil {
			rel = pth
		}
		if err = hashFile(h, "go:"+filepath.ToSlash(rel), pth); err != nil {
			return err
		}
	}
	return nil
}

// shouldSkipCacheDir reports whether a directory name should be excluded from
// the binary-cache module walk.
func shouldSkipCacheDir(name string) bool {
	switch name {
	case "vendor", "node_modules", ".git", ".hg", ".svn", ".bzr":
		return true
	default:
		return false
	}
}

// findModuleRoot walks up from srcDir and returns the directory containing
// go.mod, or empty string when none is found.
func findModuleRoot(srcDir string) string {
	dir := srcDir
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
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
