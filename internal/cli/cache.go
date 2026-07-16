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
// the module's go.mod / go.sum, the effective go.work (same resolution as
// findGoWorkValue, including GOWORK=off), every non-test .go file under the
// module root (skipping vendor and VCS dirs), local use/replace trees outside
// the module, and the gomake version plus GOOS/GOARCH. mkfNames are the
// validated makefile base names actually compiled, so ignored makefile_*
// files do not affect the key. gowork is the raw GOWORK env value.
func binaryCacheKey(
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
		if err := hashFile(h, "go.mod", filepath.Join(modRoot, "go.mod")); err != nil {
			return "", err
		}
		// Optional go.sum at module root.
		sumPath := filepath.Join(modRoot, "go.sum")
		if _, err := os.Stat(sumPath); err == nil {
			if err := hashFile(h, "go.sum", sumPath); err != nil {
				return "", err
			}
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
			if _, err := os.Stat(sumBeside); err == nil {
				if err := hashFile(h, "go.work.sum", sumBeside); err != nil {
					return "", err
				}
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
	for _, key := range []string{
		"GOFLAGS", "CGO_ENABLED", "CGO_CFLAGS", "CGO_LDFLAGS", "GOTOOLCHAIN",
	} {
		if val := os.Getenv(key); val != "" {
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

	var workRels []string
	if goWorkPath != "" {
		// use and replace from go.work; resolve relatives against workDir.
		workDir := filepath.Dir(goWorkPath)
		raw := append(
			localPathsFromGoWork(goWorkPath),
			localPathsFromGoWorkReplace(goWorkPath)...,
		)
		workRels = make([]string, 0, len(raw))
		for _, rel := range raw {
			if filepath.IsAbs(rel) {
				workRels = append(workRels, rel)
				continue
			}
			workRels = append(workRels, filepath.Join(workDir, rel))
		}
	}
	rels := append(workRels, localPathsFromGoMod(filepath.Join(modRoot, "go.mod"))...)
	if len(rels) == 0 {
		return nil
	}

	seen := map[string]struct{}{filepath.Clean(modRoot): {}}
	var roots []string
	for _, rel := range rels {
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
	sep := string(filepath.Separator)
	return strings.HasPrefix(child, parent+sep)
}

// localPathsFromGoWork returns relative or absolute use paths from a go.work
// file. Missing or unreadable files yield a nil slice.
func localPathsFromGoWork(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return scanLocalUseOrReplace(string(data), "use")
}

// localPathsFromGoWorkReplace returns local replace targets from a go.work
// file (same grammar as go.mod replace).
func localPathsFromGoWorkReplace(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return scanLocalUseOrReplace(string(data), "replace")
}

// localPathsFromGoMod returns local filesystem replace targets from a go.mod
// file (paths that start with "." or are absolute). Versioned module replaces
// are ignored.
func localPathsFromGoMod(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return scanLocalUseOrReplace(string(data), "replace")
}

// scanLocalUseOrReplace extracts local disk paths from go.work "use" or
// go.mod "replace" directives, including parenthesized multi-line forms.
func scanLocalUseOrReplace(src, keyword string) []string {
	var out []string
	inBlock := false
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if i := strings.Index(line, "//"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		if inBlock {
			if line == ")" {
				inBlock = false
				continue
			}
			if pth := localPathFromDirective(line, keyword, true); pth != "" {
				out = append(out, pth)
			}
			continue
		}
		if !strings.HasPrefix(line, keyword) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, keyword))
		if rest == "(" {
			inBlock = true
			continue
		}
		if pth := localPathFromDirective(rest, keyword, false); pth != "" {
			out = append(out, pth)
		}
	}
	return out
}

// localPathFromDirective returns a local path from a single use/replace line
// body. inBlock is true when the line is inside a parenthesized block (no
// leading keyword).
func localPathFromDirective(line, keyword string, inBlock bool) string {
	line = strings.TrimSpace(line)
	if line == "" || line == ")" {
		return ""
	}
	if keyword == "use" {
		// use ./foo  or  use "../foo"
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return ""
		}
		return unquotePath(fields[0])
	}
	// replace: "path [version] => local" (keyword already stripped when not
	// in block? when not inBlock, line is full after "replace").
	if !inBlock && strings.HasPrefix(line, "replace") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "replace"))
	}
	idx := strings.Index(line, "=>")
	if idx < 0 {
		return ""
	}
	rhs := strings.TrimSpace(line[idx+2:])
	fields := strings.Fields(rhs)
	if len(fields) == 0 {
		return ""
	}
	pth := unquotePath(fields[0])
	if !isLocalDiskPath(pth) {
		return ""
	}
	return pth
}

// unquotePath strips optional surrounding double quotes from a path token.
func unquotePath(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
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
// gowork is the raw GOWORK env value (same as findGoWorkValue).
func lookupBinaryCache(
	srcDir string,
	mkfNames []string,
	version, goos, goarch, gowork string,
) (string, bool) {

	key, err := binaryCacheKey(srcDir, mkfNames, version, goos, goarch, gowork)
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
// Errors are silently ignored because the cache is advisory. The write is
// rename-atomic so concurrent lookups never see a partial binary.
func storeBinaryCache(
	binPath, srcDir string,
	mkfNames []string,
	version, goos, goarch, gowork string,
) {

	key, err := binaryCacheKey(srcDir, mkfNames, version, goos, goarch, gowork)
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
	dst := filepath.Join(dir, name)
	// Unique temp so concurrent same-key stores cannot clobber each other.
	// CreateTemp only reserves a path; remove it so copyFile can create with
	// executable mode (OpenFile ignores mode when the file already exists).
	tmp, err := os.CreateTemp(dir, name+".*.tmp")
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
