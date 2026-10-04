// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/ctx42/ring/pkg/ring"
	"golang.org/x/mod/modfile"
)

// binaryCacheDir returns the gomake binary cache directory, creating it
// if needed. XDG_CACHE_HOME is taken from rng when set; otherwise the
// platform user cache directory is used.
func binaryCacheDir(rng *ring.Ring) (string, error) {
	base := rng.EnvGet("XDG_CACHE_HOME")
	var err error
	if base == "" {
		base, err = os.UserCacheDir()
		if err != nil {
			return "", err
		}
	}
	dir := filepath.Join(base, "gomake", "bin")
	if err = os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	return dir, nil
}

// binaryCacheKey returns a hex-encoded SHA-256 hash over the inputs that
// affect the compiled makefile binary: the makefile sources actually built,
// the module's go.mod / go.sum, the effective go.work (same resolution as
// findGoWorkValue, including GOWORK=off), every non-test .go file under the
// module root (skipping vendor and VCS dirs) and the files those sources
// name with //go:embed, local use/replace trees outside the module, and the
// gomake version plus GOOS/GOARCH. mkfNames are the validated makefile base
// names actually compiled, so ignored makefile_* files do not affect the
// key. gowork is the raw GOWORK env value. Toolchain variables (GOFLAGS,
// CGO_*, GOTOOLCHAIN, GOEXPERIMENT, and the architecture feature levels such
// as GOAMD64) are read from rng, not the process environment.
//
//nolint:cyclop,gocognit
func binaryCacheKey(
	rng *ring.Ring,
	srcDir string,
	mkfNames []string,
	version, goos, goarch, gowork string,
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
		gomod := filepath.Join(modRoot, "go.mod")
		if err := hashFile(h, "go.mod", gomod); err != nil {
			return "", err
		}
		// Optional go.sum at module root. A stat failure other than a
		// missing file is returned so the cache is not keyed as if the
		// checksums were absent.
		sumPath := filepath.Join(modRoot, "go.sum")
		if _, err := os.Stat(sumPath); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return "", err
			}
		} else if err := hashFile(h, "go.sum", sumPath); err != nil {
			return "", err
		}
		// Effective workspace (parent walk / GOWORK / off).
		// Missing GOWORK is not fatal for the cache key: hash the raw value
		// and proceed without external trees so keys stay deterministic.
		goWorkPath, _ := findGoWorkValue(gowork, modRoot)
		_, _ = fmt.Fprintf(h, "gowork:%s\n", gowork)
		if goWorkPath != "" {
			if err := hashFile(h, "go.work", goWorkPath); err != nil {
				return "", err
			}
			sumBeside := goWorkPath + ".sum"
			if _, err := os.Stat(sumBeside); err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					return "", err
				}
			} else if err := hashFile(h, "go.work.sum", sumBeside); err != nil {
				return "", err
			}
		}
		if err := hashModuleGoFiles(h, modRoot); err != nil {
			return "", err
		}
		if err := hashExternalModuleTrees(h, modRoot, goWorkPath); err != nil {
			return "", err
		}
	}

	// Toolchain / flag inputs that change the compiled binary.
	_, _ = fmt.Fprintf(h, "ver:%s\ngoos:%s\ngoarch:%s\n", version, goos, goarch)
	_, _ = fmt.Fprintf(h, "go:%s\n", runtime.Version())
	// GOEXPERIMENT and the per-architecture feature levels select build
	// tags and code generation.
	for _, key := range []string{
		"GOFLAGS", "CGO_ENABLED", "CGO_CFLAGS", "CGO_LDFLAGS", "GOTOOLCHAIN",
		"GOEXPERIMENT", "GO386", "GOAMD64", "GOARM", "GOARM64", "GOMIPS",
		"GOMIPS64", "GOPPC64", "GORISCV64", "GOWASM",
	} {
		if val := rng.EnvGet(key); val != "" {
			_, _ = fmt.Fprintf(h, "%s:%s\n", key, val)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashExternalModuleTrees hashes non-test .go files (and go.mod when present)
// under local go.work use directories and go.mod replace targets that lie
// outside modRoot, so workspace siblings and out-of-tree replaces invalidate
// the binary cache. goWorkPath is the effective workspace file (may be empty).
func hashExternalModuleTrees(
	h interface{ Write([]byte) (int, error) },
	modRoot, goWorkPath string,
) error {

	workRels := absWorkPaths(goWorkPath)
	workRels = append(
		workRels,
		localPathsFromGoMod(filepath.Join(modRoot, "go.mod"))...,
	)
	if len(workRels) == 0 {
		return nil
	}

	seen := map[string]struct{}{filepath.Clean(modRoot): {}}
	var roots []string
	for _, rel := range workRels {
		abs := rel
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(modRoot, rel)
		}
		abs = filepath.Clean(abs)
		if _, ok := seen[abs]; ok {
			continue
		}
		if isSubpath(modRoot, abs) {
			continue
		}
		if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
			continue
		}
		seen[abs] = struct{}{}
		roots = append(roots, abs)
	}
	sort.Strings(roots)

	for _, root := range roots {
		modPath := filepath.Join(root, "go.mod")
		if _, err := os.Stat(modPath); err == nil {
			label := "extmod:" + filepath.ToSlash(root)
			if err := hashFile(h, label, modPath); err != nil {
				return err
			}
		}
		if err := hashModuleGoFiles(h, root); err != nil {
			return err
		}
	}
	return nil
}

// isSubpath reports whether child is the same as parent or a path under it.
func isSubpath(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if parent == child {
		return true
	}
	// A root such as "/" already ends with the separator.
	prefix := parent
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(child, prefix)
}

// absWorkPaths returns the local use and replace paths from the go.work file
// at goWorkPath, with relative paths joined to that file's directory. An
// empty path, a missing or malformed file, or a file with no local paths
// yields a nil slice.
func absWorkPaths(goWorkPath string) []string {
	if goWorkPath == "" {
		return nil
	}
	data, err := os.ReadFile(goWorkPath)
	if err != nil {
		return nil
	}
	wf, err := modfile.ParseWork(goWorkPath, data, nil)
	if err != nil {
		return nil
	}
	raw := make([]string, 0, len(wf.Use)+len(wf.Replace))
	for _, use := range wf.Use {
		raw = append(raw, use.Path)
	}
	raw = append(raw, localReplacePaths(wf.Replace)...)
	if len(raw) == 0 {
		return nil
	}
	workDir := filepath.Dir(goWorkPath)
	paths := make([]string, 0, len(raw))
	for _, rel := range raw {
		if filepath.IsAbs(rel) {
			paths = append(paths, rel)
			continue
		}
		paths = append(paths, filepath.Join(workDir, rel))
	}
	return paths
}

// localPathsFromGoMod returns local filesystem replace targets from a go.mod
// file (paths that start with "." or are absolute). Versioned module replaces
// are ignored. A missing or malformed file yields a nil slice.
func localPathsFromGoMod(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	mf, err := modfile.Parse(path, data, nil)
	if err != nil {
		return nil
	}
	return localReplacePaths(mf.Replace)
}

// localReplacePaths returns the replacement paths of rpls that are local
// disk paths.
func localReplacePaths(rpls []*modfile.Replace) []string {
	var out []string
	for _, rpl := range rpls {
		if isLocalDiskPath(rpl.New.Path) {
			out = append(out, rpl.New.Path)
		}
	}
	return out
}

// isLocalDiskPath reports whether pth is a filesystem path rather than a
// module path (relative with "."/".." or absolute).
func isLocalDiskPath(pth string) bool {
	if pth == "" {
		return false
	}
	if filepath.IsAbs(pth) {
		return true
	}
	// Relative disk paths start with "." (covers "./", "../", ".").
	return strings.HasPrefix(pth, ".")
}

// hashFile writes a labeled file into h. Returns an error when the file
// cannot be read, a missing file included.
func hashFile(
	h interface{ Write([]byte) (int, error) },
	label, pth string,
) error {

	data, err := os.ReadFile(pth)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(h, "%s\n", label)
	_, _ = h.Write(data)
	return nil
}

// hashModuleGoFiles hashes every non-test .go file under modRoot, in sorted
// relative-path order. A directory holding a .go file with a //go:embed
// directive also has every file under it hashed, which covers whatever the
// directive names. Skips vendor, module cache, and VCS directories so the key
// tracks local package sources the makefile may import via replace.
func hashModuleGoFiles(
	h interface{ Write([]byte) (int, error) },
	modRoot string,
) error {

	paths, err := walkCacheFiles(modRoot, func(name string) bool {
		return strings.HasSuffix(name, ".go") &&
			!strings.HasSuffix(name, "_test.go")
	})
	if err != nil {
		return err
	}
	var embedDirs []string
	for _, pth := range paths {
		var data []byte
		data, err = os.ReadFile(pth)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(h, "go:%s\n", cacheLabel(modRoot, pth))
		_, _ = h.Write(data)
		if bytes.Contains(data, []byte("//go:embed")) {
			embedDirs = append(embedDirs, filepath.Dir(pth))
		}
	}

	embeds := make(map[string]struct{})
	for _, dir := range embedDirs {
		var files []string
		files, err = walkCacheFiles(dir, func(name string) bool {
			return !strings.HasSuffix(name, ".go")
		})
		if err != nil {
			return err
		}
		for _, pth := range files {
			embeds[pth] = struct{}{}
		}
	}
	for _, pth := range slices.Sorted(maps.Keys(embeds)) {
		label := "embed:" + cacheLabel(modRoot, pth)
		if err = hashFile(h, label, pth); err != nil {
			return err
		}
	}
	return nil
}

// walkCacheFiles returns the sorted paths of files under root whose base
// name keep accepts, skipping directories shouldSkipCacheDir rejects.
func walkCacheFiles(
	root string,
	keep func(name string) bool,
) ([]string, error) {

	var paths []string
	err := filepath.WalkDir(root, func(
		pth string,
		d fs.DirEntry,
		err error,
	) error {

		if err != nil {
			return err
		}
		if d.IsDir() {
			if pth != root && shouldSkipCacheDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if keep(d.Name()) {
			paths = append(paths, pth)
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

// cacheLabel returns pth relative to root in slash form, or pth itself when
// it has no relative form.
func cacheLabel(root, pth string) string {
	rel, err := filepath.Rel(root, pth)
	if err != nil {
		return pth
	}
	return filepath.ToSlash(rel)
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

// binaryCachePath returns the cache path of the binary compiled from the
// given source directory, version, and platform. gowork is the raw GOWORK env
// value (same as findGoWorkValue). Compute it once, before compiling, so a
// source edited during the build cannot file the binary under the new key.
func binaryCachePath(
	rng *ring.Ring,
	srcDir string,
	mkfNames []string,
	version, goos, goarch, gowork string,
) (string, error) {

	key, err := binaryCacheKey(
		rng, srcDir, mkfNames, version, goos, goarch, gowork,
	)
	if err != nil {
		return "", err
	}
	dir, err := binaryCacheDir(rng)
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		key += ".exe"
	}
	return filepath.Join(dir, key), nil
}

// storeBinaryCache copies the compiled binary at binPath to the cache path
// dst. Errors are silently ignored because the cache is advisory. The write is
// rename-atomic so concurrent lookups never see a partial binary.
func storeBinaryCache(binPath, dst string) {
	// Unique temp so concurrent same-key stores cannot clobber each other.
	// CreateTemp only reserves a path; remove it so copyFile can create with
	// executable mode (OpenFile ignores mode when the file already exists).
	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".*.tmp")
	if err != nil {
		return
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(tmpPath)
	if err = copyFile(binPath, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return
	}
	if err = os.Rename(tmpPath, dst); err != nil {
		_ = os.Remove(tmpPath)
	}
}
