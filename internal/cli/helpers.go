// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/build"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/internal/osarch"
	"github.com/ctx42/gomake/internal/parser"
	"github.com/ctx42/gomake/internal/version"
	"github.com/ctx42/gomake/pkg/gomake"
)

// isCoreCmd returns true if the target name is a core target (":"-prefixed),
// mirroring [mkf.Target.IsCore].
func isCoreCmd(tgtName string) bool { return strings.HasPrefix(tgtName, ":") }

// fail writes err to stderr decorated for the user. It is the single place
// controlling how command errors are presented.
func fail(rng *ring.Ring, err error) {
	_, _ = fmt.Fprintf(rng.Stderr(), "%s: %s\n", binName, err)
}

// failCode writes err to stderr with [fail] and returns the conventional exit
// code for err from [mkf.ExitCode].
func failCode(rng *ring.Ring, err error) int {
	fail(rng, err)
	return mkf.ExitCode(err)
}

// errCompile represents a makefile compilation failure, including captured
// stdout and stderr from the Go toolchain when available.
type errCompile struct {
	error        // Compilation error.
	sout  string // Contents of standard output.
	eout  string // Contents of standard error.
}

func (e *errCompile) Unwrap() error { return e.error }

func (e *errCompile) Error() string {
	var msg string
	if e.error != nil {
		msg = e.error.Error()
	}
	if e.eout != "" {
		return e.eout + "\n" + msg
	}
	if e.sout != "" {
		return e.sout + "\n" + msg
	}
	return msg
}

// makefiles returns a list of "makefile*.go" files in the import path. The
// list depends on the environment (GOOS, ...). It returns [errNoMakefile]
// if there are no makefiles.
//
// Function does not check if "makefile.go" is on the list.
func makefiles(rng *ring.Ring, impPath string) ([]string, error) {
	fn := gmFiles
	if bt := parser.GetBuildTag(rng); bt == "" {
		fn = goFiles
	}
	files, err := fn(rng, impPath)
	if err != nil {
		return nil, err
	}
	files = filterMkf(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("%w in %s", errNoMakefile, impPath)
	}
	return files, nil
}

// filterMkf filters out filenames that are not starting with "makefile" or
// "makefile_".
func filterMkf(fls []string) []string {
	var ret []string
	for _, fil := range fls {
		name := filepath.Base(fil)
		if strings.HasPrefix(name, "makefile.") ||
			strings.HasPrefix(name, "makefile_") {
			ret = append(ret, fil)
		}
	}
	return ret
}

// validMakefileName reports whether name is an allowed makefile filename. Only
// the main makefile and its GOOS/GOARCH variants are accepted, mirroring Go's
// own filename build constraints:
//
//   - makefile.go
//   - makefile_<GOOS>.go
//   - makefile_<GOARCH>.go
//   - makefile_<GOOS>_<GOARCH>.go (GOOS before GOARCH)
//
// GOOS/GOARCH validity is checked against [osarch]. Matching is case-sensitive.
// Any other suffix (custom names, the dot form makefile.<x>.go, reversed order,
// or extra tokens) is rejected.
func validMakefileName(name string) bool {
	if name == mkf.MakefileMain {
		return true
	}
	mid, ok := strings.CutPrefix(name, "makefile_")
	if !ok {
		return false
	}
	mid, ok = strings.CutSuffix(mid, ".go")
	if !ok {
		return false
	}
	switch parts := strings.Split(mid, "_"); len(parts) {
	case 1:
		return osarch.IsGOOS(parts[0]) || osarch.IsGOARCH(parts[0])
	case 2:
		return osarch.IsGOOS(parts[0]) && osarch.IsGOARCH(parts[1])
	default:
		return false
	}
}

// selectMakefiles splits files into the makefiles to compile (keep) and the
// makefile-looking files rejected by [validMakefileName] (ignored, as base
// names). Files that do not look like makefiles at all are dropped silently, as
// before. Input order is preserved.
func selectMakefiles(files []string) (keep, ignored []string) {
	for _, fil := range files {
		name := filepath.Base(fil)
		switch {
		case validMakefileName(name):
			keep = append(keep, fil)
		case strings.HasPrefix(name, "makefile.") ||
			strings.HasPrefix(name, "makefile_"):
			ignored = append(ignored, name)
		}
	}
	return keep, ignored
}

// ignoreWarning formats the warning gomake prints for a makefile-looking
// source file rejected by [validMakefileName].
func ignoreWarning(name string) string {
	format := "gomake: ignoring %q: only makefile.go and " +
		"makefile_<GOOS>.go / makefile_<GOARCH>.go / " +
		"makefile_<GOOS>_<GOARCH>.go are allowed\n"
	return fmt.Sprintf(format, name)
}

// compUnit captures information about single makefile compilation unit.
type compUnit struct {
	// Absolute path to the makefile sources.
	SourceDir string

	// The root directory where BuildDir is created.
	BuildRootDir string

	// Absolute path to out of source build directory.
	//
	// The out of source build directory contains all makefiles copied from
	// SourceDir along with generated makefile [mkf.MakefileGen] and
	// compiled makefile [mkf.MakefileBin].
	//
	// In general this directory (and its contents) is removed after gomake
	// executes the makefile binary.
	BuildDir string

	// Absolute path to the generated makefile.
	//
	// Generated makefile is the code gomake generates to be able to execute
	// the user defined targets.
	MainGen string

	// Absolute path to main makefile binary.
	//
	// The binary is the result of compiling user provided code
	// [mkf.MakefileMain] and gomake-generated [mkf.MakefileGen] and
	// [mkf.MakefileUser] (and built-in targets when enabled).
	MainBin string

	// Absolute path to definition of user targets.
	MainUser string

	// Absolute path to source module "go.mod" file.
	GoModSrc string

	// Absolute path to "go.mod" file in build dir.
	GoModDst string

	// Absolute paths to all the files to compile to get MainBin.
	Files []string

	// Source makefile base names selected for compilation (makefile.go and its
	// GOOS/GOARCH variants). Used for the binary cache key.
	MkfNames []string

	// Base names of makefile-looking sources rejected by [validMakefileName].
	// The entry point reports these to the user as a warning.
	Ignored []string

	// Environment to use when preparing build directory.
	Ring *ring.Ring

	// Source package info returned by go list on SourceDir.
	SrcPkg *parser.Package
}

// srcDirMustContain is the error format for files that must not appear in the
// source directory. These errors are informational; callers are not expected to
// inspect them with errors.Is or errors.As.
const srcDirMustContain = "source directory must not contain %q file"

// prepare creates build directory with random name in tmp directory and moves
// all makefile*.go files from src (source directory) to it. It also makes sure
// files with restricted names are not in src directory:
//
//   - [mkf.MakefileGen]
//   - [mkf.MakefileBin]
//   - [mkf.MakefileUser]
//
// All the makefiles in the src directory may or may not be tagged with the
// [parser.BuildTag]; do not mix tagged and untagged makefile sources in src.
//
//nolint:cyclop,gocognit
func prepare(rng *ring.Ring, tmp, src string) (cu *compUnit, err error) {
	// Before we do anything we must be in a Go project (the go.mod file
	// exists).
	if _, err = gomake.Root(src); err != nil {
		return nil, err
	}

	var buildDir string
	var buildDirCreated bool
	defer func() {
		if err != nil && buildDirCreated {
			_ = os.RemoveAll(buildDir)
		}
	}()

	// Source must contain [mkf.MakefileMain] file.
	mkfMain := filepath.Join(src, mkf.MakefileMain)
	if _, err = os.Stat(mkfMain); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w in %s", errNoMakefile, src)
		}
		return nil, fmt.Errorf("stat %s: %w", mkfMain, err)
	}

	// Source directory must not contain [mkf.MakefileGen] file.
	mkfGen := filepath.Join(src, mkf.MakefileGen)
	if _, err = os.Stat(mkfGen); err == nil {
		return nil, fmt.Errorf(srcDirMustContain, mkf.MakefileGen)
	}

	// Source directory must not contain [mkf.MakefileBin] file.
	mkfBin := filepath.Join(src, mkf.MakefileBin)
	if _, err = os.Stat(mkfBin); err == nil {
		return nil, fmt.Errorf(srcDirMustContain, mkf.MakefileBin)
	}

	// Source directory must not contain [mkf.MakefileUser] file.
	mkfUser := filepath.Join(src, mkf.MakefileUser)
	if _, err = os.Stat(mkfUser); err == nil {
		return nil, fmt.Errorf(srcDirMustContain, mkf.MakefileUser)
	}

	// Get package with user-defined targets.
	var pkg *parser.Package
	if pkg, err = parser.NewPackage(rng, src); err != nil {
		return nil, err
	}

	// Discover makefiles: makefile.go plus only the GOOS/GOARCH filename
	// variants. Other makefile_* files are ignored; the entry point warns
	// about the names collected here.
	mkfFiles, ignored := selectMakefiles(pkg.Files)

	// MakefileMain must always be copied. go list may place it in
	// InvalidGoFiles (e.g. stale build cache after a permission change),
	// omitting it from pkg.Files. We prepend it here so that the copy loop
	// below surfaces a real error if the file is actually unreadable.
	hasMkfMain := slices.Contains(mkfFiles, mkf.MakefileMain)
	if !hasMkfMain {
		mkfFiles = append([]string{mkf.MakefileMain}, mkfFiles...)
	}

	// Create out of source compilation directory.
	if buildDir, err = os.MkdirTemp(tmp, "gomake-*"); err != nil {
		return nil, err
	}
	buildDirCreated = true

	// Copy all makefiles to destination and remove BuildTagLine if present.
	tagLine := []byte(buildTagLine)
	copied := make([]string, 0, len(mkfFiles))
	for _, srcPth := range mkfFiles {
		var filData []byte
		srcPth = filepath.Join(src, srcPth)
		if filData, err = os.ReadFile(srcPth); err != nil {
			return nil, err
		}
		filData = stripBuildTag(filData, tagLine)
		dstPth := filepath.Join(buildDir, filepath.Base(srcPth))
		if err = os.WriteFile(dstPth, filData, 0600); err != nil {
			return nil, err
		}
		copied = append(copied, dstPth)
	}

	// Add empty user targets, later we can override it once we have them.
	mkfUser = filepath.Join(buildDir, mkf.MakefileUser)
	err = os.WriteFile(mkfUser, parser.TgsUserEmptySrc, 0600)
	if err != nil {
		return nil, err
	}

	var content []byte
	mod := pkg.Module

	// Copy "go.mod" file.
	if content, err = os.ReadFile(mod.ModPath); err != nil {
		return nil, err
	}
	goModDst := filepath.Join(buildDir, "go.mod")
	if err = os.WriteFile(goModDst, content, 0600); err != nil {
		return nil, err
	}

	// Copy "go.work" file (module root, parent walk, or GOWORK).
	modRoot := filepath.Dir(mod.ModPath)
	goWorkSrc, err := findGoWork(rng, modRoot)
	if err != nil {
		return nil, err
	}
	if goWorkSrc != "" {
		if content, err = os.ReadFile(goWorkSrc); err != nil {
			return nil, err
		}
		goWorkDst := filepath.Join(buildDir, "go.work")
		if err = os.WriteFile(goWorkDst, content, 0600); err != nil {
			return nil, err
		}
		if err = editGoWork(rng, goWorkSrc, buildDir, modRoot); err != nil {
			return nil, err
		}

		// Sibling sum next to the discovered go.work, when present.
		goWorkSumSrc := goWorkSrc + ".sum"
		if gomake.FileExists(goWorkSumSrc) {
			if content, err = os.ReadFile(goWorkSumSrc); err != nil {
				return nil, err
			}
			sumDst := filepath.Join(buildDir, "go.work.sum")
			if err = os.WriteFile(sumDst, content, 0600); err != nil {
				return nil, err
			}
		}
	}

	// Copy "go.sum" file before editing go.mod: editGoMod records the xflag
	// checksum by appending to this file, so it must exist first.
	goSum := filepath.Join(filepath.Dir(mod.ModPath), "go.sum")
	if content, err = os.ReadFile(goSum); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		goSumDst := filepath.Join(buildDir, "go.sum")
		if err = os.WriteFile(goSumDst, content, 0600); err != nil {
			return nil, err
		}
	}

	if err = editGoMod(
		rng, goModDst, mod.ImpSpec, mod.ImpPath, modRoot,
	); err != nil {
		return nil, err
	}

	cu = &compUnit{
		SourceDir:    src,
		BuildRootDir: tmp,
		BuildDir:     buildDir,
		MainGen:      filepath.Join(buildDir, mkf.MakefileGen),
		MainBin:      filepath.Join(buildDir, mkf.MakefileBin),
		MainUser:     mkfUser,
		GoModSrc:     mod.ModPath,
		GoModDst:     goModDst,
		Files:        copied,
		MkfNames:     mkfFiles,
		Ignored:      ignored,
		Ring:         rng,
		SrcPkg:       pkg,
	}
	cu.Files = append(cu.Files, cu.MainGen, cu.MainUser)
	return cu, nil
}

// stripBuildTag removes build-constraint lines that mention the gomake tag so
// the out-of-source build can compile without -tags=gomake. CRLF is normalized
// first. Matches exact BuildTagLine and compound lines such as
// //go:build gomake && linux (and legacy // +build forms that include gomake).
func stripBuildTag(filData, tagLine []byte) []byte {
	_ = tagLine // retained for call-site compatibility
	filData = bytes.ReplaceAll(filData, []byte("\r\n"), []byte("\n"))
	lines := bytes.Split(filData, []byte("\n"))
	out := make([][]byte, 0, len(lines))
	skipNextBlank := false
	for _, line := range lines {
		if isGomakeConstraintLine(bytes.TrimSpace(line)) {
			skipNextBlank = true
			continue
		}
		if skipNextBlank {
			skipNextBlank = false
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
		}
		out = append(out, line)
	}
	return bytes.Join(out, []byte("\n"))
}

// isGomakeConstraintLine reports whether line is a //go:build or // +build
// constraint that mentions the gomake tag.
func isGomakeConstraintLine(line []byte) bool {
	s := string(line)
	if !strings.HasPrefix(s, "//") {
		return false
	}
	body := strings.TrimSpace(strings.TrimPrefix(s, "//"))
	if strings.HasPrefix(body, "go:build ") {
		return strings.Contains(body, "gomake")
	}
	if strings.HasPrefix(body, "+build ") {
		return strings.Contains(body, "gomake")
	}
	return false
}

// findGoWork returns the path to the go.work file that applies to modRoot.
// It honors GOWORK when set (including "off"), otherwise walks from modRoot
// toward the filesystem root like the Go toolchain. Empty path means none.
// A set, non-off GOWORK that does not exist returns an error (no silent
// fallthrough to module-only mode).
func findGoWork(env ring.Environ, modRoot string) (string, error) {
	var gowork string
	if env != nil {
		gowork = env.EnvGet("GOWORK")
	}
	return findGoWorkValue(gowork, modRoot)
}

// findGoWorkValue is the env-free core of findGoWork. Relative GOWORK paths
// are resolved against the process working directory (like the Go toolchain),
// not against modRoot. The second result is non-nil only when GOWORK is set
// to a missing path.
func findGoWorkValue(gowork, modRoot string) (string, error) {
	if gowork != "" {
		if gowork == "off" {
			return "", nil
		}
		pth := gowork
		if !filepath.IsAbs(pth) {
			abs, err := filepath.Abs(pth)
			if err != nil {
				return "", fmt.Errorf("GOWORK %s: %w", gowork, err)
			}
			pth = abs
		}
		pth = filepath.Clean(pth)
		if gomake.FileExists(pth) {
			return pth, nil
		}
		return "", fmt.Errorf("GOWORK %s: no such file", pth)
	}
	dir := modRoot
	for {
		pth := filepath.Join(dir, "go.work")
		if _, err := os.Stat(pth); err == nil {
			return pth, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

// editGoWork rewrites relative use paths in the go.work file already copied
// to dst. srcWork is the absolute path of the original workspace file (used
// only to resolve relative use entries). Paths that resolve to modRoot become
// "." so the build directory remains the workspace's main module; other
// relative paths become absolute. GOWORK is pinned per command so ambient
// GOWORK never mutates the caller's workfile.
func editGoWork(env ring.Environ, srcWork, dst, modRoot string) error {
	workDir := filepath.Dir(srcWork)
	base := env.EnvAll()
	srcEnv := ring.EnvSet(base, "GOWORK", srcWork)

	out := &bytes.Buffer{}
	cmd := exec.Command("go", "work", "edit", "-json")
	cmd.Env = srcEnv
	cmd.Dir = workDir
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return goEditErr(errGoWorkEdit, srcWork, out.String(), err)
	}

	var result struct {
		Use []struct {
			DiskPath string `json:"DiskPath"`
		} `json:"Use"`
		Replace []struct {
			Old struct {
				Path    string `json:"Path"`
				Version string `json:"Version"`
			} `json:"Old"`
			New struct {
				Path    string `json:"Path"`
				Version string `json:"Version"`
			} `json:"New"`
		} `json:"Replace"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return goEditErr(errGoWorkEdit, srcWork, out.String(), err)
	}

	modRoot = filepath.Clean(modRoot)
	workDir = filepath.Clean(workDir)
	replace := make(map[string]string, len(result.Use))
	for _, val := range result.Use {
		dp := val.DiskPath
		var pth string
		switch {
		case filepath.IsAbs(dp):
			pth = filepath.Clean(dp)
		case filepath.Clean(dp) == ".":
			// "." is the workspace root (workDir), not the build dir.
			pth = workDir
		default:
			pth = filepath.Clean(filepath.Join(workDir, dp))
		}
		// Makefile module becomes the build-dir main module.
		if modRoot != "" && pth == modRoot {
			if filepath.Clean(dp) != "." {
				replace[dp] = "."
			}
			continue
		}
		// Parent monorepo: keep an absolute path to the original workDir.
		if filepath.Clean(dp) == "." {
			replace[dp] = pth
			continue
		}
		if filepath.IsAbs(dp) {
			continue
		}
		replace[dp] = pth
	}

	dstWork := filepath.Join(dst, "go.work")
	dstEnv := ring.EnvSet(base, "GOWORK", dstWork)
	for from, to := range replace {
		out.Reset()
		cmd = exec.Command("go", "work", "edit", "-dropuse", from)
		cmd.Env = dstEnv
		cmd.Dir = dst
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Run(); err != nil {
			return goEditErr(errGoWorkEdit, dstWork, out.String(), err)
		}

		out.Reset()
		cmd = exec.Command("go", "work", "edit", "-use", to)
		cmd.Env = dstEnv
		cmd.Dir = dst
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Run(); err != nil {
			return goEditErr(errGoWorkEdit, dstWork, out.String(), err)
		}
	}

	// Absolutize local replace targets against the original workDir.
	for _, rpl := range result.Replace {
		newPath := rpl.New.Path
		if !isLocalDiskPath(newPath) {
			continue
		}
		abs := newPath
		if !filepath.IsAbs(abs) {
			abs = filepath.Clean(filepath.Join(workDir, abs))
		}
		oldSpec := rpl.Old.Path
		if rpl.Old.Version != "" {
			oldSpec = rpl.Old.Path + "@" + rpl.Old.Version
		}
		out.Reset()
		cmd = exec.Command(
			"go", "work", "edit",
			"-dropreplace="+oldSpec,
			"-replace="+oldSpec+"="+abs,
		)
		cmd.Env = dstEnv
		cmd.Dir = dst
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Run(); err != nil {
			return goEditErr(errGoWorkEdit, dstWork, out.String(), err)
		}
	}
	return nil
}

// editGoMod edits "go.mod" pointed by absolute path pth. It sets the module
// name to "makefile", requires pkgImpSpec and replaces it with pkgPath. It also
// requires the xflag module and records its checksum, because the generated
// makefile imports xflag while user projects do not. srcModDir is the original
// module root used to absolutize local replace paths after the copy.
func editGoMod(
	env ring.Environ,
	pth, pkgImpSpec, pkgPath, srcModDir string,
) error {

	dir := filepath.Dir(pth)
	xflagReq := xflagModPath + "@" + xflagVersion()
	// Pin GOWORK like compile so ambient workspace does not affect go mod.
	modEnv := pinBuildGOWORK(env.EnvAll(), dir)

	cmd := exec.Command(
		"go", "mod", "edit",
		"-module", "makefile",
		"-require="+pkgImpSpec+"@v0.0.0",
		"-replace="+pkgImpSpec+"@v0.0.0="+pkgPath,
		"-require="+xflagReq,
	)
	cmd.Env = modEnv
	cmd.Dir = dir
	out := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return goEditErr(errGoModEdit, pth, out.String(), err)
	}

	if err := absolutizeGoModReplaces(modEnv, dir, srcModDir); err != nil {
		return err
	}

	// The copied "go.sum" lacks xflag, so populate it from the module cache
	// (gomake was built with the same version) before the build runs.
	out.Reset()
	cmd = exec.Command("go", "mod", "download", xflagReq)
	cmd.Env = modEnv
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return goEditErr(errGoModEdit, pth, out.String(), err)
	}
	return nil
}

// absolutizeGoModReplaces rewrites relative local replace targets in the
// build-dir go.mod so they resolve against the original source module root.
func absolutizeGoModReplaces(env []string, buildDir, srcModDir string) error {
	out := &bytes.Buffer{}
	cmd := exec.Command("go", "mod", "edit", "-json")
	cmd.Env = env
	cmd.Dir = buildDir
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return goEditErr(errGoModEdit, buildDir, out.String(), err)
	}
	var result struct {
		Replace []struct {
			Old struct {
				Path    string `json:"Path"`
				Version string `json:"Version"`
			} `json:"Old"`
			New struct {
				Path    string `json:"Path"`
				Version string `json:"Version"`
			} `json:"New"`
		} `json:"Replace"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return goEditErr(errGoModEdit, buildDir, out.String(), err)
	}
	srcModDir = filepath.Clean(srcModDir)
	for _, rpl := range result.Replace {
		newPath := rpl.New.Path
		if !isLocalDiskPath(newPath) {
			continue
		}
		abs := newPath
		if !filepath.IsAbs(abs) {
			abs = filepath.Clean(filepath.Join(srcModDir, abs))
		}
		if abs == newPath {
			continue
		}
		oldSpec := rpl.Old.Path
		if rpl.Old.Version != "" {
			oldSpec = rpl.Old.Path + "@" + rpl.Old.Version
		}
		out.Reset()
		cmd = exec.Command(
			"go", "mod", "edit",
			"-dropreplace="+oldSpec,
			"-replace="+oldSpec+"="+abs,
		)
		cmd.Env = env
		cmd.Dir = buildDir
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Run(); err != nil {
			return goEditErr(errGoModEdit, buildDir, out.String(), err)
		}
	}
	return nil
}

// xflagModPath is the module path of the xflag package inlined into generated
// makefiles.
const xflagModPath = "github.com/ctx42/xflag"

// xflagFallbackVer pins the xflag version used when build information is
// unavailable at runtime.
const xflagFallbackVer = "v0.10.0"

// xflagVersion returns the xflag module version gomake was built with, so the
// generated makefile pins the same version. It falls back to [xflagFallbackVer]
// when build information is unavailable.
func xflagVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return xflagFallbackVer
	}
	for _, dep := range bi.Deps {
		if dep.Path != xflagModPath {
			continue
		}
		if dep.Replace != nil {
			dep = dep.Replace
		}
		if dep.Version != "" {
			return dep.Version
		}
	}
	return xflagFallbackVer
}

// goEditErr wraps a failed "go mod/work edit" with sentinel and the location,
// appending the captured toolchain output or, when empty, the raw error.
func goEditErr(sentinel error, where, out string, err error) error {
	detail := strings.TrimSpace(out)
	if detail == "" {
		return fmt.Errorf("%w at %s: %w", sentinel, where, err)
	}
	return fmt.Errorf("%w at %s: %s: %w", sentinel, where, detail, err)
}

// compile compiles binary from given files by calling "go build" with given
// environment and in given working directory. The compiled makefile will be
// put in path defined by out. The returned error will always be of ErrCompile
// type. GOWORK is pinned to wd/go.work when present, otherwise "off", so the
// build never follows the caller's ambient workspace.
func compile(
	ctx context.Context,
	env []string,
	wd, out string,
	files ...string,
) error {

	args := []string{
		"build",
		"-ldflags",
		version.LDFlags(),
		"-o",
		out,
	}
	args = append(args, files...)

	env = pinBuildGOWORK(env, wd)

	sout := &bytes.Buffer{}
	eout := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Stdout = sout
	cmd.Stderr = eout
	cmd.Env = env
	cmd.Dir = wd
	if err := cmd.Run(); err != nil {
		err = &errCompile{
			error: err,
			sout:  sout.String(),
			eout:  eout.String(),
		}
		return err
	}
	return nil
}

// pinBuildGOWORK sets GOWORK to buildDir/go.work when that file exists,
// otherwise to "off", so go tool invocations use only the prepared workspace.
func pinBuildGOWORK(env []string, buildDir string) []string {
	work := filepath.Join(buildDir, "go.work")
	if _, err := os.Stat(work); err == nil {
		return ring.EnvSet(env, "GOWORK", work)
	}
	return ring.EnvSet(env, "GOWORK", "off")
}

// gmFiles returns the list of files which have buildTag ("go:build") for the
// given operating system and architecture in the given impPath directory. The
// impPath must be an absolute path.
//
// Example:
//
//	gmFiles(env, "/dir/package")
func gmFiles(rng *ring.Ring, impPath string) ([]string, error) {
	if bt := parser.GetBuildTag(rng); bt == "" {
		return nil, errors.New("build tag must be provided")
	}

	// Get all the files, including those with the buildTag.
	fls, err := goFiles(rng, impPath)
	if err != nil {
		return nil, fmt.Errorf("listing build tag files: %w", err)
	}

	// Get all the files without the buildTag.
	rng = parser.RemBuildTag(rng)
	without, err := goFiles(rng, impPath)
	if err != nil {
		return nil, fmt.Errorf("listing all files: %w", err)
	}

	// Remove files that are not tagged with buildTag.
	for i := range without {
		for it, fil := range fls {
			if fil == without[i] {
				fls = append(fls[:it], fls[it+1:]...)
				break
			}
		}
	}
	return fls, nil
}

// goFiles returns a list of all Go files in a given impPath directory for
// the given operating system and architecture, if buildTag is set to not empty
// string it will also return files which have buildTag in "go:build".
//
// Returns empty slice and no error if there are no Go files in the directory.
//
// Example:
//
//	goFiles(env, "/dir/package")
func goFiles(rng *ring.Ring, impPath string) ([]string, error) {
	bctx := build.Default
	if bt := parser.GetBuildTag(rng); bt != "" {
		bctx.BuildTags = []string{bt}
	}
	bctx.GOOS = rng.EnvGet("GOOS")
	bctx.GOARCH = rng.EnvGet("GOARCH")

	pkg, err := bctx.Import(".", impPath, build.IgnoreVendor)
	if err != nil {
		if _, ok := errors.AsType[*build.NoGoError](err); ok {
			return []string{}, nil
		}
		// Allow multiple packages in the same directory.
		if _, ok := errors.AsType[*build.MultiplePackageError](err); !ok {
			return nil, fmt.Errorf("listing Go source files: %w", err)
		}
	}

	fls := make([]string, len(pkg.GoFiles))
	for i := range pkg.GoFiles {
		fls[i] = filepath.Join(impPath, pkg.GoFiles[i])
	}
	return fls, nil
}

// allTargets returns built-in (and pulled) targets from stock, plus makefile
// targets when a makefile is present in the project.
//
// For listing and help, targets are parsed directly from source without
// creating a build directory.
func allTargets(
	rng *ring.Ring,
	cfg *config,
	stock []*mkf.Target,
) ([]*mkf.Target, error) {

	combined := append([]*mkf.Target(nil), stock...)
	if cfg.bin != "" {
		return combined, nil
	}

	// Inexpensive pre-checks before invoking go list.
	if _, err := gomake.Root(cfg.src); err != nil {
		return combined, nil
	}
	if _, err := os.Stat(filepath.Join(cfg.src, mkf.MakefileMain)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return combined, nil
		}
		return nil, fmt.Errorf("stat %s: %w", mkf.MakefileMain, err)
	}

	// Parse targets directly from source — no build dir, no `go mod` edit.
	// Restrict package files to valid makefile names so list/help match
	// what prepare compiles.
	parser.SetBuildTag(rng)
	var err error
	var pmf *parser.Makefile
	analyzeAct := func() error {
		var pkg *parser.Package
		pkg, err = parser.NewPackage(rng, cfg.src)
		if err != nil {
			return err
		}
		keep, _ := selectMakefiles(pkg.Files)
		pkg.Files = keep
		pmf, err = parser.MakefileFromPackage(rng, pkg)
		return err
	}
	err = withProgress(rng.Stderr(), "Analyzing sources...", analyzeAct)
	if err != nil {
		if errors.Is(err, parser.ErrAstEmpty) {
			return combined, nil
		}
		return nil, err
	}

	return append(combined, pmf.Targets.List()...), nil
}
