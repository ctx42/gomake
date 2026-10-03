// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/ctx42/ring/pkg/ring"
)

// binaryCacheDir returns the gomake binary cache directory, creating it
// if needed. XDG_CACHE_HOME is taken from rng when set; otherwise the
// platform user cache directory is used.
func binaryCacheDir(rng *ring.Ring) (string, error) {
	var base string
	if rng != nil {
		base = rng.EnvGet("XDG_CACHE_HOME")
	}
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
// key. gowork is the raw GOWORK env value. Toolchain variables
// (GOFLAGS, CGO_*, GOTOOLCHAIN) are read from rng, not the process
// environment.
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
		data, err := os.ReadFile(filepath.Join(srcDir, name)) //nolint:gosec
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
	for _, key := range []string{
		"GOFLAGS", "CGO_ENABLED", "CGO_CFLAGS", "CGO_LDFLAGS", "GOTOOLCHAIN",
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
	sep := string(filepath.Separator)
	return strings.HasPrefix(child, parent+sep)
}

// absWorkPaths returns the use and replace paths from the go.work file at
// goWorkPath, with relative paths joined to that file's directory. An empty
// path, a missing file, or a file with no local paths yields a nil slice.
func absWorkPaths(goWorkPath string) []string {
	if goWorkPath == "" {
		return nil
	}
	workDir := filepath.Dir(goWorkPath)
	raw := append(
		localPathsFromGoWork(goWorkPath),
		localPathsFromGoWorkReplace(goWorkPath)...,
	)
	if len(raw) == 0 {
		return nil
	}
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

// localPathsFromGoWork returns relative or absolute use paths from a go.work
// file. Missing or unreadable files yield a nil slice.
func localPathsFromGoWork(path string) []string {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil
	}
	return scanLocalUseOrReplace(string(data), "use")
}

// localPathsFromGoWorkReplace returns local replace targets from a go.work
// file (same grammar as go.mod replace).
func localPathsFromGoWorkReplace(path string) []string {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil
	}
	return scanLocalUseOrReplace(string(data), "replace")
}

// localPathsFromGoMod returns local filesystem replace targets from a go.mod
// file (paths that start with "." or are absolute). Versioned module replaces
// are ignored.
func localPathsFromGoMod(path string) []string {
	data, err := os.ReadFile(path) //nolint:gosec
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
		line = stripLineComment(line)
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
		tok := firstModuleToken(line)
		if tok == "" {
			return ""
		}
		return unquotePath(tok)
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
	tok := firstModuleToken(rhs)
	if tok == "" {
		return ""
	}
	pth := unquotePath(tok)
	if !isLocalDiskPath(pth) {
		return ""
	}
	return pth
}

// stripLineComment drops a // comment that sits outside a double-quoted
// string and trims the remainder.
func stripLineComment(line string) string {
	inQuote := false
	escaped := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inQuote = false
			}
			continue
		}
		if c == '"' {
			inQuote = true
			continue
		}
		if c == '/' && i+1 < len(line) && line[i+1] == '/' {
			return strings.TrimSpace(line[:i])
		}
	}
	return strings.TrimSpace(line)
}

// firstModuleToken returns the first go.mod token. A double-quoted token
// keeps its quotes and any spaces inside them.
func firstModuleToken(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if s[0] != '"' {
		if i := strings.IndexAny(s, " \t"); i >= 0 {
			return s[:i]
		}
		return s
	}
	escaped := false
	for i := 1; i < len(s); i++ {
		if escaped {
			escaped = false
			continue
		}
		if s[i] == '\\' {
			escaped = true
			continue
		}
		if s[i] == '"' {
			return s[:i+1]
		}
	}
	return s
}

// unquotePath decodes a double-quoted go.mod path. An unquoted token is
// returned unchanged.
func unquotePath(s string) string {
	if len(s) >= 2 && s[0] == '"' {
		if u, err := strconv.Unquote(s); err == nil {
			return u
		}
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
func hashFile(
	h interface{ Write([]byte) (int, error) },
	label, pth string,
) error {

	data, err := os.ReadFile(pth) //nolint:gosec
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(h, "%s\n", label)
	_, _ = h.Write(data)
	return nil
}

// hashModuleGoFiles hashes every non-test .go file under modRoot, in sorted
// relative-path order, and the files those sources name with //go:embed.
// Skips vendor, module cache, and VCS directories so the key tracks local
// package sources the makefile may import via replace.
func hashModuleGoFiles(
	h interface{ Write([]byte) (int, error) },
	modRoot string,
) error {

	var paths []string
	err := filepath.WalkDir(modRoot, func(
		pth string,
		d fs.DirEntry,
		err error,
	) error {

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
		if !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") {
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
		if err = hashGoEmbeds(h, modRoot, pth); err != nil {
			return err
		}
	}
	return nil
}

// hashGoEmbeds hashes the files named by //go:embed directives in goFile.
// A pattern the compiler would reject, and a target that is not on disk, is
// skipped. modRoot is the root the cache labels are relative to.
func hashGoEmbeds(
	h interface{ Write([]byte) (int, error) },
	modRoot string,
	goFile string,
) error {

	data, err := os.ReadFile(goFile) //nolint:gosec
	if err != nil {
		return err
	}
	patterns := goEmbedPatterns(data)
	if len(patterns) == 0 {
		return nil
	}
	dir := filepath.Dir(goFile)
	var files []string
	for _, pat := range patterns {
		matched, err := matchEmbedPattern(dir, pat)
		if err != nil {
			return err
		}
		files = append(files, matched...)
	}
	sort.Strings(files)
	prev := ""
	for _, pth := range files {
		if pth == prev {
			continue
		}
		prev = pth
		rel, err := filepath.Rel(modRoot, pth)
		if err != nil {
			rel = pth
		}
		if err = hashFile(h, "embed:"+filepath.ToSlash(rel), pth); err != nil {
			return err
		}
	}
	return nil
}

// goEmbedPatterns returns the //go:embed arguments in src. A directive that
// does not parse is skipped; the compiler reports that on its own.
func goEmbedPatterns(src []byte) []string {
	var out []string
	for _, line := range bytes.Split(src, []byte("\n")) {
		s := strings.TrimSuffix(string(line), "\r")
		if !strings.HasPrefix(s, "//go:embed") {
			continue
		}
		dir, ok := ast.ParseDirective(token.NoPos, s)
		if !ok || dir.Tool != "go" || dir.Name != "embed" {
			continue
		}
		args, err := dir.ParseArgs()
		if err != nil {
			continue
		}
		for _, arg := range args {
			if arg.Arg != "" {
				out = append(out, arg.Arg)
			}
		}
	}
	return out
}

// matchEmbedPattern returns the regular files a //go:embed pattern names
// under dir. all: keeps names that start with "." or "_".
func matchEmbedPattern(dir, pattern string) ([]string, error) {
	all := false
	if rest, ok := strings.CutPrefix(pattern, "all:"); ok {
		pattern = rest
		all = true
	}
	if !fs.ValidPath(pattern) {
		return nil, nil
	}
	if strings.ContainsAny(pattern, "*?[") {
		return globEmbed(dir, pattern, all)
	}
	target := filepath.Join(dir, filepath.FromSlash(pattern))
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if info.IsDir() {
		return walkEmbedDir(target, all)
	}
	if !info.Mode().IsRegular() {
		return nil, nil
	}
	return []string{target}, nil
}

// globEmbed expands pattern relative to dir and returns the regular files it
// names. A matched directory contributes the files under it.
func globEmbed(dir, pattern string, all bool) ([]string, error) {
	glob := filepath.Join(dir, filepath.FromSlash(pattern))
	matches, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, match := range matches {
		name := filepath.Base(match)
		if !all && hiddenEmbedName(name) {
			continue
		}
		info, err := os.Lstat(match)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			files, err := walkEmbedDir(match, all)
			if err != nil {
				return nil, err
			}
			out = append(out, files...)
			continue
		}
		if info.Mode().IsRegular() {
			out = append(out, match)
		}
	}
	return out, nil
}

// walkEmbedDir returns regular files under root. Names that start with "."
// or "_" are skipped unless all is set, matching //go:embed.
func walkEmbedDir(root string, all bool) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(
		pth string,
		d fs.DirEntry,
		err error,
	) error {

		if err != nil {
			return err
		}
		name := d.Name()
		if pth != root && !all && hiddenEmbedName(name) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		out = append(out, pth)
		return nil
	})
	return out, err
}

// hiddenEmbedName reports whether //go:embed skips name unless all: is set.
func hiddenEmbedName(name string) bool {
	return name != "" && (name[0] == '.' || name[0] == '_')
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
	rng *ring.Ring,
	srcDir string,
	mkfNames []string,
	version, goos, goarch, gowork string,
) (string, bool) {

	key, err := binaryCacheKey(
		rng, srcDir, mkfNames, version, goos, goarch, gowork,
	)
	if err != nil {
		return "", false
	}
	dir, err := binaryCacheDir(rng)
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
	rng *ring.Ring,
	binPath, srcDir string,
	mkfNames []string,
	version, goos, goarch, gowork string,
) {

	key, err := binaryCacheKey(
		rng, srcDir, mkfNames, version, goos, goarch, gowork,
	)
	if err != nil {
		return
	}
	dir, err := binaryCacheDir(rng)
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
