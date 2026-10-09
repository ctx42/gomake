// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package install

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/pathkit"
)

func Test_Main(t *testing.T) {
	t.Run("resolves GOBIN and installs", func(t *testing.T) {
		// --- Given ---
		// A minimal buildable module with no targets.yaml. Main resolves the
		// destination from GOBIN (pointed at a temp dir) and, with no imports,
		// builds straight from the working tree: no temp copy, no builtin
		// regeneration, source tree left untouched.
		src := t.TempDir()
		oskit.Write(t, "module example.test\n\ngo 1.24\n", src, "go.mod")
		oskit.MkdirAll(t, src, "cmd", "gomake")
		main := "package main\n\nfunc main() {}\n"
		oskit.Write(t, main, src, "cmd", "gomake", "main.go")
		t.Chdir(src)

		gobin := t.TempDir()
		rng := ringtest.New(t).Ring()
		rng.EnvSet("GOBIN", gobin)

		info, _ := debug.ReadBuildInfo()

		// --- When ---
		err := Main(t.Context(), rng, info, nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, oskit.PathExists(t, filepath.Join(gobin, "gomake")))
		// The source tree is unchanged: no targets.yaml was written and no
		// builtins were regenerated into it.
		assert.False(t, oskit.PathExists(t, filepath.Join(src, "targets.yaml")))
		gen := filepath.Join(src, "internal", "builtin", "targets.go")
		assert.False(t, oskit.PathExists(t, gen))
	})

	t.Run("error - GOBIN cannot be resolved", func(t *testing.T) {
		// --- Given ---
		// With no `go` on PATH, GoBinPath cannot resolve the destination, so
		// Main fails before doing any work.
		t.Chdir(t.TempDir())
		t.Setenv("PATH", t.TempDir())
		tst := ringtest.New(t)

		// --- When ---
		err := Main(t.Context(), tst.Ring(), nil, nil)

		// --- Then ---
		assert.ErrorContain(t, "cannot determine GOBIN/GOPATH", err)
	})
}

func Test_installTo(t *testing.T) {
	// publishedInfo returns a *debug.BuildInfo that simulates a
	// published-version build using github.com/ctx42/ring@v0.7.0, which is
	// already in the local module cache and does not require network access.
	publishedInfo := &debug.BuildInfo{
		Main: debug.Module{Path: "github.com/ctx42/ring", Version: "v0.7.0"},
	}
	develInfo := &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}

	t.Run("error - build info unavailable", func(t *testing.T) {
		// --- Given ---
		t.Chdir(t.TempDir())

		rng := ringtest.New(t).Ring()
		rng.EnvSet("GOWORK", "keep")

		// --- When ---
		err := installTo(t.Context(), rng, nil, t.TempDir(), nil)

		// --- Then ---
		assert.ErrorIs(t, ErrNoBuildInfo, err)
		work, _ := rng.EnvLookup("GOWORK")
		assert.Equal(t, "keep", work)
	})

	t.Run("GOWORK restored", func(t *testing.T) {
		// --- Given ---
		t.Chdir(t.TempDir()) // No go.mod, so the source root lookup fails.

		rng := ringtest.New(t).Ring()
		rng.EnvSet("GOWORK", "keep")

		// --- When ---
		err := installTo(t.Context(), rng, develInfo, t.TempDir(), nil)

		// --- Then ---
		assert.Error(t, err)
		work, _ := rng.EnvLookup("GOWORK")
		assert.Equal(t, "keep", work)
	})

	t.Run("GOWORK unset again", func(t *testing.T) {
		// --- Given ---
		t.Chdir(t.TempDir())

		rng := ringtest.New(t).Ring()
		rng.EnvUnset("GOWORK")

		// --- When ---
		err := installTo(t.Context(), rng, develInfo, t.TempDir(), nil)

		// --- Then ---
		assert.Error(t, err)
		_, set := rng.EnvLookup("GOWORK")
		assert.False(t, set)
	})

	t.Run("error - nonexistent targets file", func(t *testing.T) {
		// --- Given ---
		// Devel install resolves the module root via go.mod (not CWD alone).
		// Use a temp module so nothing lands in the real project tree.
		src := t.TempDir()
		oskit.Write(t, "module example.test\n\ngo 1.24\n", src, "go.mod")
		t.Chdir(src)

		tst := ringtest.New(t)
		pth := []string{filepath.Join(t.TempDir(), "missing.yaml")}
		info, _ := debug.ReadBuildInfo()

		// --- When ---
		err := installTo(t.Context(), tst.Ring(), info, t.TempDir(), pth)

		// --- Then ---
		assert.ErrorContain(t, "read targets", err)
	})

	t.Run("error - imports route through full path", func(t *testing.T) {
		// --- Given ---
		// A --targets file with an import must take the full path: the
		// effective targets.yaml is written (inline os.WriteFile) and
		// PrepareTargets (go get + regen) runs before the build. No require
		// path in go.mod makes go get fail before touching the network,
		// proving the fast path was not taken.
		src := t.TempDir()
		oskit.Write(t, "module example.test\n\ngo 1.24\n", src, "go.mod")
		t.Chdir(src)

		tst := ringtest.New(t)

		content := "imports:\n  - import: example.com/pkg@v1.0.0\n"
		tgs := []string{oskit.Write(t, content, t.TempDir(), "targets.yaml")}

		info, _ := debug.ReadBuildInfo()

		// --- When ---
		err := installTo(t.Context(), tst.Ring(), info, t.TempDir(), tgs)

		// --- Then ---
		// go get runs only on the full path (fast path skips it). Snapshot
		// restore removes the temporary targets.yaml written for the attempt.
		assert.ErrorContain(t, "go get example.com/pkg@v1.0.0", err)
		assert.NoFileExist(t, filepath.Join(src, "targets.yaml"))
	})

	t.Run("error - targets file cannot be written", func(t *testing.T) {
		// --- Given ---
		// A read-only module root causes os.WriteFile to fail when installTo
		// writes the effective targets.yaml into the build tree.
		src := t.TempDir()
		oskit.Write(t, "module example.test\n\ngo 1.24\n", src, "go.mod")
		if err := os.Chmod(src, 0o555); err != nil {
			t.Skip("cannot make dir read-only:", err)
		}
		t.Cleanup(func() { _ = os.Chmod(src, 0o755) })
		t.Chdir(src)

		tst := ringtest.New(t)

		content := "imports:\n  - import: example.com/pkg@v1.0.0\n"
		tgs := []string{oskit.Write(t, content, t.TempDir(), "targets.yaml")}

		info, _ := debug.ReadBuildInfo()

		// --- When ---
		err := installTo(t.Context(), tst.Ring(), info, t.TempDir(), tgs)

		// --- Then ---
		assert.ErrorContain(t, "write targets", err)
	})

	t.Run("error - non-devel module download", func(t *testing.T) {
		// --- Given ---
		// A published-version build info with an unknown module causes
		// moduleCacheDir to fail; installTo must propagate the error.
		t.Chdir(t.TempDir())
		tst := ringtest.New(t)
		info := &debug.BuildInfo{
			Main: debug.Module{Path: "example.invalid/mod", Version: "v0.0.1"},
		}

		// --- When ---
		err := installTo(t.Context(), tst.Ring(), info, t.TempDir(), nil)

		// --- Then ---
		assert.ErrorContain(t, "module download example.invalid/mod", err)
	})

	t.Run("error - module download temp dir", func(t *testing.T) {
		// --- Given ---
		// A published-version build first downloads its module from a
		// neutral temp dir. Setting the ring's TMPDIR to a non-existent path
		// makes that os.MkdirTemp fail, which installTo must propagate.
		t.Chdir(t.TempDir())
		rng := ringtest.New(t).Ring()
		rng.EnvSet("TMPDIR", filepath.Join(t.TempDir(), "nonexistent"))
		dst := t.TempDir()

		tgs := filepath.Join(t.TempDir(), "targets.yaml")
		oskit.Write(t, "imports:\n  - import: example.com/pkg@v1.0.0\n", tgs)

		// --- When ---
		err := installTo(t.Context(), rng, publishedInfo, dst, []string{tgs})

		// --- Then ---
		assert.ErrorIs(t, os.ErrNotExist, err)
		assert.ErrorContain(t, "module download temp", err)
	})

	t.Run("error - non-devel full path copy then regen", func(t *testing.T) {
		// --- Given ---
		// A published-version build with imports copies the module source to a
		// temp dir (covering the copyToTemp success path), then PrepareTargets
		// fails because go get cannot resolve the fake import.
		t.Chdir(t.TempDir())
		tst := ringtest.New(t)

		tgs := filepath.Join(t.TempDir(), "targets.yaml")
		oskit.Write(t, "imports:\n  - import: example.com/pkg@v1.0.0\n", tgs)

		// --- When ---
		err := installTo(
			t.Context(),
			tst.Ring(),
			publishedInfo,
			t.TempDir(),
			[]string{tgs},
		)

		// --- Then ---
		assert.ErrorContain(t, "go get example.com/pkg@v1.0.0", err)
	})

	t.Run("full path build runs after PrepareTargets", func(t *testing.T) {
		// --- Given ---
		// A Go module whose targets.yaml lists a locally-replaced fake
		// package. go get resolves the replace without network; go list
		// finds the package; MakefileFromPackage accepts packages with no
		// gomake targets (ErrAstEmpty path). GenMain writes into
		// internal/builtin and internal/builtin/data, which must already
		// exist. The final build call compiles cmd/gomake.
		//
		// The generated targets.go imports the gomake-internal mkf package,
		// so a fake external module cannot compile it into its own binary;
		// this case therefore asserts the generated code lands in the same
		// internal/builtin package the binary compiles, not the pre-refactor
		// pkg/builtin. That the compiled binary truly exposes an external
		// target is covered by the "compiles external target" case below.
		src := t.TempDir()
		const gomod = "module test.example.com\n\ngo 1.24\n\n" +
			"replace example.com/fakepkg => ./fakepkg\n"
		oskit.Write(t, gomod, src, "go.mod")
		oskit.MkdirAll(t, src, "fakepkg")
		mod := "module example.com/fakepkg\n\ngo 1.24\n"
		oskit.Write(t, mod, src, "fakepkg", "go.mod")
		oskit.Write(t, "package fakepkg\n", src, "fakepkg", "fakepkg.go")
		oskit.MkdirAll(t, src, "internal", "builtin", "data")
		oskit.MkdirAll(t, src, "cmd", "gomake")
		main := "package main\n\nfunc main() {}\n"
		oskit.Write(t, main, src, "cmd", "gomake", "main.go")
		t.Chdir(src)

		dst := t.TempDir()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		tgs := filepath.Join(t.TempDir(), "targets.yaml")
		oskit.Write(t, "imports:\n  - import: example.com/fakepkg\n", tgs)

		info, _ := debug.ReadBuildInfo()

		// --- When ---
		err := installTo(t.Context(), rng, info, dst, []string{tgs})

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, oskit.PathExists(t, filepath.Join(dst, "gomake")))
		want := "adding external target example.com/fakepkg"
		assert.Contain(t, want, tst.Stderr())
		// Snapshot restore leaves the working tree clean: regenerated
		// targets.go is removed when it did not exist before the install.
		gen := filepath.Join(src, "internal", "builtin", "targets.go")
		assert.False(t, oskit.PathExists(t, gen))
	})
	t.Run("fast path restores go.sum", func(t *testing.T) {
		// --- Given ---
		info, ok := debug.ReadBuildInfo()
		if !ok || info.Main.Version != "(devel)" {
			t.Skip("fast path restore runs on a devel build")
		}

		src := t.TempDir()
		gomod := "" +
			"module example.test\n" +
			"\n" +
			"go 1.24\n" +
			"\n" +
			"require github.com/ctx42/ring v0.7.0\n"
		oskit.Write(t, gomod, src, "go.mod")
		oskit.MkdirAll(t, src, "cmd", "gomake")
		mainSrc := "" +
			"package main\n" +
			"\n" +
			"import _ \"github.com/ctx42/ring/pkg/ring\"\n" +
			"\n" +
			"func main() {}\n"
		oskit.Write(t, mainSrc, src, "cmd", "gomake", "main.go")
		t.Chdir(src)

		rng := ringtest.New(t).Ring()
		rng.EnvSet("GOFLAGS", "-mod=mod")

		// --- When ---
		err := installTo(t.Context(), rng, info, t.TempDir(), nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.NoFileExist(t, filepath.Join(src, "go.sum"))
	})

	t.Run("error - fast path restore", func(t *testing.T) {
		// --- Given ---
		info, ok := debug.ReadBuildInfo()
		if !ok || info.Main.Version != "(devel)" {
			t.Skip("fast path restore runs on a devel build")
		}
		if os.Geteuid() == 0 {
			t.Skip("root ignores file permissions")
		}

		// No cmd/gomake makes the build fail; a read-only go.mod makes the
		// restore fail.
		src := t.TempDir()
		mod := "module example.test\n\ngo 1.24\n"
		gomod := oskit.Write(t, mod, src, "go.mod")
		must.Nil(os.Chmod(gomod, 0o444))
		t.Chdir(src)
		rng := ringtest.New(t).Ring()

		// --- When ---
		err := installTo(t.Context(), rng, info, t.TempDir(), nil)

		// --- Then ---
		assert.ErrorRegexp(t, "(?s)cmd/gomake.*go.mod: permission denied", err)
	})

	t.Run("compiles external target", func(t *testing.T) {
		// End-to-end: install the real gomake module (a copy, so the working
		// tree is left untouched) with a local external-target module wired via
		// replace, then run the built binary and confirm the external target is
		// compiled in. This is the only case that drives the real internal/mkf
		// types through the generated builtins; the fake-module cases above
		// cannot, because a foreign module may not import gomake internals.
		if testing.Short() {
			t.Skip("slow: copies the module and runs go build")
		}

		// --- Given ---
		root := goListDir(t, "-m", "-f", "{{.Dir}}", "github.com/ctx42/gomake")
		build := t.TempDir()
		copyTree(t, root, build)

		// A local module exposing one gomake target, wired via a replace so go
		// get resolves it without the network.
		oskit.MkdirAll(t, build, "exttgt")
		ext := filepath.Join(build, "exttgt")
		extMod := "module example.com/exttgt\n\ngo 1.26\n\n" +
			"require github.com/ctx42/ring v0.7.0\n"
		oskit.Write(t, extMod, ext, "go.mod")
		target := "" +
			"package exttgt\n\n" +
			"import (\n" +
			"\t\"context\"\n\n" +
			"\t\"github.com/ctx42/ring/pkg/ring\"\n" +
			")\n\n" +
			"// Pingxyzzy prints a sentinel.\n" +
			"func Pingxyzzy(_ context.Context, rng *ring.Ring) error {\n" +
			"\t_, _ = rng.Stdout().Write([]byte(\"pong\"))\n" +
			"\treturn nil\n" +
			"}\n"
		oskit.Write(t, target, ext, "target.go")
		goModEdit(t, build, "-replace", "example.com/exttgt=./exttgt")

		tgs := filepath.Join(t.TempDir(), "targets.yaml")
		oskit.Write(t, "imports:\n  - import: example.com/exttgt\n", tgs)

		t.Chdir(build)
		dst := t.TempDir()
		tst := ringtest.New(t).WetStderr()
		info := &debug.BuildInfo{Main: debug.Module{
			Path:    "github.com/ctx42/gomake",
			Version: "(devel)",
		}}

		// --- When ---
		err := installTo(t.Context(), tst.Ring(), info, dst, []string{tgs})

		// --- Then ---
		assert.NoError(t, err)

		assert.Contain(
			t,
			"adding external target example.com/exttgt",
			tst.Stderr(),
		)
		bin := filepath.Join(dst, "gomake")
		assert.True(t, oskit.PathExists(t, bin))

		// The freshly built binary lists the compiled-in external target.
		assert.Contain(t, "pingxyzzy", runBin(t, bin, "--list"))
	})

	t.Run("local targets via workspace", func(t *testing.T) {
		// Option A end-to-end: a local --targets file inside a Go module is
		// resolved from disk via a temporary Go workspace, with no replace and
		// no go get. The in-source build's go.mod is left byte-identical, and
		// the freshly built binary still exposes the compiled-in external
		// target.
		if testing.Short() {
			t.Skip("slow: copies the module and runs go build")
		}

		// --- Given ---
		root := goListDir(t, "-m", "-f", "{{.Dir}}", "github.com/ctx42/gomake")
		build := t.TempDir()
		copyTree(t, root, build)
		goBefore := oskit.ReadFileStr(t, build, "go.mod")
		genBefore := oskit.ReadFileStr(
			t,
			build,
			"internal", "builtin", "targets.go",
		)

		// A separate local module exposing one gomake target. It is wired only
		// through the workspace, never a replace, so go.mod must stay
		// untouched.
		ext := t.TempDir()
		extMod := "module example.com/wstgt\n\ngo 1.26\n\n" +
			"require github.com/ctx42/ring v0.7.0\n"
		oskit.Write(t, extMod, ext, "go.mod")
		target := "" +
			"package wstgt\n\n" +
			"import (\n" +
			"\t\"context\"\n\n" +
			"\t\"github.com/ctx42/ring/pkg/ring\"\n" +
			")\n\n" +
			"// Wsxyzzy prints a sentinel.\n" +
			"func Wsxyzzy(_ context.Context, rng *ring.Ring) error {\n" +
			"\t_, _ = rng.Stdout().Write([]byte(\"pong\"))\n" +
			"\treturn nil\n" +
			"}\n"
		oskit.Write(t, target, ext, "target.go")
		tgs := filepath.Join(ext, "targets.yaml")
		oskit.Write(t, "imports:\n  - import: example.com/wstgt\n", tgs)

		t.Chdir(build)
		dst := t.TempDir()
		tst := ringtest.New(t).WetStderr()
		info := &debug.BuildInfo{Main: debug.Module{
			Path:    "github.com/ctx42/gomake",
			Version: "(devel)",
		}}

		// --- When ---
		err := installTo(t.Context(), tst.Ring(), info, dst, []string{tgs})

		// --- Then ---
		assert.NoError(t, err)

		assert.Contain(
			t,
			"adding external target example.com/wstgt",
			tst.Stderr(),
		)

		// go.mod was never touched: no replace, no require, no go get.
		assert.Equal(t, goBefore, oskit.ReadFileStr(t, build, "go.mod"))

		// The generated targets.go was restored, so the tree still compiles
		// without the workspace: the install is safely re-runnable.
		gen := filepath.Join(build, "internal", "builtin", "targets.go")
		genAfter := oskit.ReadFileStr(t, gen)
		assert.Equal(t, genBefore, genAfter)

		bin := filepath.Join(dst, "gomake")
		assert.True(t, oskit.PathExists(t, bin))
		assert.Contain(t, "wsxyzzy", runBin(t, bin, "--list"))
	})
}

func Test_effectiveImports(t *testing.T) {
	t.Run("no flag loads source targets.yaml", func(t *testing.T) {
		// --- Given ---
		srcDir := t.TempDir()
		content := "imports:\n  - import: a.com/x\n"
		oskit.Write(t, content, srcDir, "targets.yaml")

		// --- When ---
		have, err := effectiveImports(t.Context(), ring.New(), srcDir, nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"a.com/x"}, have.Paths())
	})

	t.Run("no flag and absent source", func(t *testing.T) {
		// --- When ---
		have, err := effectiveImports(t.Context(), ring.New(), t.TempDir(), nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 0, len(have.Imports()))
	})

	t.Run("flag file overrides and exposes raw bytes", func(t *testing.T) {
		// --- Given ---
		srcDir := t.TempDir()
		content := "imports:\n  - import: src.com/x\n"
		oskit.Write(t, content, srcDir, "targets.yaml")

		flagFile := filepath.Join(t.TempDir(), "flag.yaml")
		oskit.Write(t, "imports:\n  - import: flag.com/y\n", flagFile)
		tgs := []string{flagFile}

		// --- When ---
		have, err := effectiveImports(t.Context(), ring.New(), srcDir, tgs)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(
			t,
			"imports:\n  - import: flag.com/y\n",
			string(have.Raw()),
		)
		assert.Equal(t, []string{"flag.com/y"}, have.Paths())
	})

	t.Run("flag files are merged", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		f0 := oskit.Write(t, "imports:\n  - import: a.com/x\n", dir, "f0.yaml")
		content := "imports:\n  - import: a.com/x\n  - import: b.com/y\n"
		f1 := oskit.Write(t, content, dir, "f1.yaml")
		tgs := []string{f0, f1}

		// --- When ---
		have, err := effectiveImports(t.Context(), ring.New(), dir, tgs)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"a.com/x", "b.com/y"}, have.Paths())
	})

	t.Run("error - conflicting flag files", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		content := "imports:\n  - import: a.com/x@v1.0.0\n"
		f0 := oskit.Write(t, content, dir, "f0.yaml")
		content = "imports:\n  - import: a.com/x@v2.0.0\n"
		f1 := oskit.Write(t, content, dir, "f1.yaml")
		tgs := []string{f0, f1}

		// --- When ---
		have, err := effectiveImports(t.Context(), ring.New(), dir, tgs)

		// --- Then ---
		want := "conflicting import: a.com/x in " + f0 + " and " + f1
		assert.ErrorContain(t, want, err)
		assert.Nil(t, have)
	})

	t.Run("error - one of flag files missing", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		f0 := oskit.Write(t, "imports:\n  - import: a.com/x\n", dir, "f0.yaml")
		tgs := []string{f0, filepath.Join(dir, "missing.yaml")}

		// --- When ---
		have, err := effectiveImports(t.Context(), ring.New(), dir, tgs)

		// --- Then ---
		assert.ErrorContain(t, "missing.yaml", err)
		assert.Nil(t, have)
	})

	t.Run("error - missing flag file", func(t *testing.T) {
		// --- Given ---
		flagFile := filepath.Join(t.TempDir(), "missing.yaml")

		// --- When ---
		_, err := effectiveImports(
			t.Context(),
			ring.New(),
			t.TempDir(),
			[]string{flagFile},
		)

		// --- Then ---
		assert.ErrorContain(t, "read targets", err)
	})

	t.Run("error - invalid flag file", func(t *testing.T) {
		// --- Given ---
		flagFile := filepath.Join(t.TempDir(), "bad.yaml")
		oskit.Write(t, "{", flagFile)

		// --- When ---
		_, err := effectiveImports(
			t.Context(),
			ring.New(),
			t.TempDir(),
			[]string{flagFile},
		)

		// --- Then ---
		assert.ErrorContain(t, "invalid external targets config", err)
	})

	t.Run("error - canceled context", func(t *testing.T) {
		// --- Given ---
		ctx, cxl := context.WithCancel(t.Context())
		cxl()

		// --- When ---
		_, err := effectiveImports(
			ctx,
			ring.New(),
			t.TempDir(),
			[]string{"https://example.com/targets.yaml"},
		)

		// --- Then ---
		assert.ErrorIs(t, context.Canceled, err)
	})
}

func Test_moduleCacheDir(t *testing.T) {
	t.Run("returns directory for a cached module", func(t *testing.T) {
		// --- When ---
		have, err := moduleCacheDir(ring.New(), "github.com/ctx42/ring@v0.7.0")

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(
			t,
			"github.com/ctx42/ring@v0.7.0",
			filepath.ToSlash(have),
		)
		assert.True(t, oskit.PathExists(t, have))
	})

	t.Run("error - ring TMPDIR missing", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("TMPDIR", filepath.Join(t.TempDir(), "nonexistent"))

		// --- When ---
		_, err := moduleCacheDir(rng, "github.com/ctx42/ring@v0.7.0")

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.ErrorContain(t, "module download temp", err)
	})

	t.Run("ignores the working directory", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.Write(t, "not a module\n", dir, "go.mod")
		t.Chdir(dir)

		// --- When ---
		have, err := moduleCacheDir(ring.New(), "github.com/ctx42/ring@v0.7.0")

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, filepath.IsAbs(have))
	})

	t.Run("error - invalid module", func(t *testing.T) {
		// --- When ---
		_, err := moduleCacheDir(ring.New(), "not.a.module@!!!")

		// --- Then ---
		// Pin the offending module so the assertion fails if a sibling
		// path (e.g. a missing go binary) produced the error instead.
		assert.ErrorContain(t, "module download not.a.module@!!!", err)
	})

	t.Run("error - go binary not in PATH", func(t *testing.T) {
		// --- Given ---
		// exec.Command resolves "go" via the process PATH (LookPath), not
		// cmd.Env. Clear PATH before constructing the ring and the command.
		empty := t.TempDir()
		t.Setenv("PATH", empty)
		rng := ring.New()
		rng.EnvSet("PATH", empty)

		// --- When ---
		_, err := moduleCacheDir(rng, "example.com/mod@v1.0.0")

		// --- Then ---
		// The exec-not-found cause is unique to this path; a plain
		// "module download" wrapper is shared with the ExitError branch.
		assert.ErrorContain(t, "executable file not found", err)
	})
}

func Test_setupWorkspace(t *testing.T) {
	t.Run("empty targets is a no-op", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		buildDir := t.TempDir()

		// --- When ---
		hMod, hCleanup, err := setupWorkspace(rng, buildDir, nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, hMod)
		_, set := rng.EnvLookup("GOWORK")
		assert.False(t, set)

		hCleanup()
	})

	t.Run("URL targets is a no-op", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		buildDir := t.TempDir()
		tgs := "https://example.com/targets.yaml"

		// --- When ---
		hMod, hCleanup, err := setupWorkspace(rng, buildDir, []string{tgs})

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, hMod)
		_, set := rng.EnvLookup("GOWORK")
		assert.False(t, set)

		hCleanup()
	})

	t.Run("targets outside a module is a no-op", func(t *testing.T) {
		// --- Given ---
		// The targets file sits in a bare directory with no go.mod, so no
		// module can be resolved and the caller falls back to go get.
		rng := ringtest.New(t).Ring()
		buildDir := t.TempDir()
		tgs := filepath.Join(t.TempDir(), "targets.yaml")

		// --- When ---
		hMod, hCleanup, err := setupWorkspace(rng, buildDir, []string{tgs})

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, hMod)
		_, set := rng.EnvLookup("GOWORK")
		assert.False(t, set)

		hCleanup()
	})

	t.Run("local module wires GOWORK", func(t *testing.T) {
		// --- Given ---
		// The build tree and a separate module tree each carry a go.mod; the
		// targets file lives at the module root.
		rng := ringtest.New(t).Ring()

		buildDir := t.TempDir()
		bmod := "module example.com/build\n\ngo 1.24\n"
		oskit.Write(t, bmod, buildDir, "go.mod")

		modDir := t.TempDir()
		oskit.Write(t, "module example.com/tgt\n\ngo 1.24\n", modDir, "go.mod")
		tgs := filepath.Join(modDir, "targets.yaml")
		oskit.Write(t, "imports:\n", tgs)

		// --- When ---
		hMod, hCleanup, err := setupWorkspace(rng, buildDir, []string{tgs})

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"example.com/tgt"}, hMod)
		work, set := rng.EnvLookup("GOWORK")
		assert.True(t, set)
		assert.True(t, oskit.PathExists(t, work))
		assert.FileContain(t, "use", work)

		// Cleanup restores GOWORK=off and removes the workspace file.
		hCleanup()
		workVal, set := rng.EnvLookup("GOWORK")
		assert.True(t, set)
		assert.Equal(t, "off", workVal)
		assert.False(t, oskit.PathExists(t, work))
	})

	t.Run("two local modules", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		buildDir := t.TempDir()
		bmod := "module example.com/build\n\ngo 1.24\n"
		oskit.Write(t, bmod, buildDir, "go.mod")

		mod0 := t.TempDir()
		oskit.Write(t, "module example.com/a\n\ngo 1.24\n", mod0, "go.mod")
		mod1 := t.TempDir()
		oskit.Write(t, "module example.com/b\n\ngo 1.24\n", mod1, "go.mod")
		tgs := []string{
			oskit.Write(t, "imports:\n", mod0, "targets.yaml"),
			oskit.Write(t, "imports:\n", mod1, "targets.yaml"),
		}

		// --- When ---
		have, hCleanup, err := setupWorkspace(rng, buildDir, tgs)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"example.com/a", "example.com/b"}, have)
		work := rng.EnvGet("GOWORK")
		assert.FileContain(t, filepath.Base(mod0), work)
		assert.FileContain(t, filepath.Base(mod1), work)

		hCleanup()
	})

	t.Run("GOFLAGS -mod=mod", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet("GOFLAGS", "-mod=mod -trimpath")

		buildDir := t.TempDir()
		bmod := "module example.com/build\n\ngo 1.24\n"
		oskit.Write(t, bmod, buildDir, "go.mod")

		modDir := t.TempDir()
		oskit.Write(t, "module example.com/tgt\n\ngo 1.24\n", modDir, "go.mod")
		tgs := oskit.Write(t, "imports:\n", modDir, "targets.yaml")

		// --- When ---
		have, hCleanup, err := setupWorkspace(rng, buildDir, []string{tgs})

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"example.com/tgt"}, have)
		cmd := exec.CommandContext(t.Context(), "go", "list", "-m")
		cmd.Env = rng.EnvAll()
		cmd.Dir = buildDir
		out, lerr := cmd.CombinedOutput()
		assert.NoError(t, lerr, string(out))

		hCleanup()
		assert.Equal(t, "-mod=mod -trimpath", rng.EnvGet("GOFLAGS"))
	})

	t.Run("home relative targets", func(t *testing.T) {
		// --- Given ---
		modDir := t.TempDir()
		oskit.Write(t, "module example.com/tgt\n\ngo 1.24\n", modDir, "go.mod")
		oskit.Write(t, "imports:\n", modDir, "targets.yaml")

		rng := ringtest.New(t).Ring()
		rng.EnvSet("HOME", modDir)

		buildDir := t.TempDir()
		bmod := "module example.com/build\n\ngo 1.24\n"
		oskit.Write(t, bmod, buildDir, "go.mod")
		tgs := []string{"~/targets.yaml"}

		// --- When ---
		have, hCleanup, err := setupWorkspace(rng, buildDir, tgs)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"example.com/tgt"}, have)

		hCleanup()
	})

	t.Run("error - workspace temp", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet("TMPDIR", filepath.Join(t.TempDir(), "nonexistent"))

		modDir := t.TempDir()
		oskit.Write(t, "module example.com/tgt\n\ngo 1.24\n", modDir, "go.mod")
		tgs := oskit.Write(t, "imports:\n", modDir, "targets.yaml")

		// --- When ---
		have, hCleanup, err := setupWorkspace(rng, t.TempDir(), []string{tgs})

		// --- Then ---
		assert.ErrorContain(t, "workspace temp", err)
		assert.Nil(t, have)
		_, set := rng.EnvLookup("GOWORK")
		assert.False(t, set)

		hCleanup()
	})

	t.Run("error - go work init", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		modDir := t.TempDir()
		oskit.Write(t, "module example.com/tgt\n\ngo 1.24\n", modDir, "go.mod")
		tgs := oskit.Write(t, "imports:\n", modDir, "targets.yaml")

		missing := filepath.Join(t.TempDir(), "missing")

		// --- When ---
		have, hCleanup, err := setupWorkspace(rng, missing, []string{tgs})

		// --- Then ---
		assert.ErrorContain(t, "go work init", err)
		assert.Nil(t, have)
		assert.Equal(t, "off", rng.EnvGet("GOWORK"))

		hCleanup()
	})
}

func Test_localModules(t *testing.T) {
	t.Run("skips URLs and files outside a module", func(t *testing.T) {
		// --- Given ---
		tgs := []string{
			"https://example.com/targets.yaml",
			filepath.Join(t.TempDir(), "targets.yaml"),
		}

		// --- When ---
		hMods, hRoots, err := localModules(ring.New(), tgs)

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, hMods)
		assert.Nil(t, hRoots)
	})

	t.Run("one module listed once", func(t *testing.T) {
		// --- Given ---
		modDir := t.TempDir()
		oskit.Write(t, "module example.com/a\n\ngo 1.24\n", modDir, "go.mod")
		oskit.MkdirAll(t, modDir, "sub")
		tgs := []string{
			oskit.Write(t, "imports:\n", modDir, "f0.yaml"),
			oskit.Write(t, "imports:\n", modDir, "sub", "f1.yaml"),
		}

		// --- When ---
		hMods, hRoots, err := localModules(ring.New(), tgs)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"example.com/a"}, hMods)
		wRoots := []string{pathkit.EvalSymlinks(t, modDir)}
		assert.Equal(t, wRoots, hRoots)
	})

	t.Run("error - home not set", func(t *testing.T) {
		// --- Given ---
		rng := ring.New(ring.WithEnv([]string{}))

		// --- When ---
		hMods, hRoots, err := localModules(rng, []string{"~/targets.yaml"})

		// --- Then ---
		assert.Error(t, err)
		assert.Nil(t, hMods)
		assert.Nil(t, hRoots)
	})
}

func Test_withoutModMod_tabular(t *testing.T) {
	tt := []struct {
		testN string

		flags string
		want  string
	}{
		{"empty", "", ""},
		{"only mod", "-mod=mod", ""},
		{"double dash", "--mod=mod -v", "-v"},
		{"kept", "-mod=mod -trimpath -tags=x", "-trimpath -tags=x"},
		{"readonly kept", "-mod=readonly", "-mod=readonly"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := withoutModMod(tc.flags)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_moduleAt(t *testing.T) {
	t.Run("reports module path and root", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		dir := t.TempDir()
		oskit.Write(t, "module example.com/mod\n\ngo 1.24\n", dir, "go.mod")

		// --- When ---
		hMod, hRoot, hOk, err := moduleAt(rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, hOk)
		assert.Equal(t, "example.com/mod", hMod)
		assert.Equal(t, dir, hRoot)
	})

	t.Run("not a module", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		// --- When ---
		hMod, hRoot, hOk, err := moduleAt(rng, t.TempDir())

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, hOk)
		assert.Equal(t, "", hMod)
		assert.Equal(t, "", hRoot)
	})

	t.Run("error - go binary not in PATH", func(t *testing.T) {
		// --- Given ---
		empty := t.TempDir()
		t.Setenv("PATH", empty)
		rng := ringtest.New(t).Ring()
		rng.EnvSet("PATH", empty)

		// --- When ---
		hMod, hRoot, hOk, err := moduleAt(rng, t.TempDir())

		// --- Then ---
		assert.ErrorContain(t, "executable file not found", err)
		assert.False(t, hOk)
		assert.Equal(t, "", hMod)
		assert.Equal(t, "", hRoot)
	})
}

func Test_notModule_tabular(t *testing.T) {
	tt := []struct {
		testN string

		err  error
		want bool
	}{
		{
			"go.mod not found",
			&exec.ExitError{Stderr: []byte("go: go.mod file not found")},
			true,
		},
		{
			"main module",
			&exec.ExitError{Stderr: []byte("go: cannot find main module")},
			true,
		},
		{"other exit", &exec.ExitError{Stderr: []byte("go: boom")}, false},
		{"not an exit error", errors.New("go: go.mod file not found"), false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := notModule(tc.err)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_goWorkInit(t *testing.T) {
	t.Run("writes a workspace listing both modules", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		buildDir := t.TempDir()
		bmod := "module example.com/build\n\ngo 1.24\n"
		oskit.Write(t, bmod, buildDir, "go.mod")

		modDir := t.TempDir()
		oskit.Write(t, "module example.com/mod\n\ngo 1.24\n", modDir, "go.mod")

		wsDir := t.TempDir()

		// --- When ---
		err := goWorkInit(rng, wsDir, buildDir, modDir)

		// --- Then ---
		assert.NoError(t, err)
		work := filepath.Join(wsDir, "go.work")
		assert.FileContain(t, buildDir, work)
		assert.FileContain(t, modDir, work)
	})

	t.Run("error - workspace directory does not exist", func(t *testing.T) {
		// --- Given ---
		// A non-existent wsDir makes the go subprocess fail to chdir before it
		// runs, surfacing the wrapped error.
		rng := ringtest.New(t).Ring()
		wsDir := filepath.Join(t.TempDir(), "nonexistent")

		// --- When ---
		err := goWorkInit(rng, wsDir, t.TempDir(), t.TempDir())

		// --- Then ---
		assert.ErrorContain(t, "go work init", err)
	})
}

func Test_restoreEnv(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("KEY", "old")
		restore := restoreEnv(rng, "KEY")
		rng.EnvSet("KEY", "new")

		// --- When ---
		restore()

		// --- Then ---
		assert.Equal(t, "old", rng.EnvGet("KEY"))
	})

	t.Run("unset", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvUnset("KEY")
		restore := restoreEnv(rng, "KEY")
		rng.EnvSet("KEY", "new")

		// --- When ---
		restore()

		// --- Then ---
		_, set := rng.EnvLookup("KEY")
		assert.False(t, set)
	})
}

func Test_joinRestore(t *testing.T) {
	t.Run("both succeed", func(t *testing.T) {
		// --- When ---
		err := joinRestore(nil, func() error { return nil })

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error - run failed", func(t *testing.T) {
		// --- Given ---
		runErr := errors.New("run")

		// --- When ---
		err := joinRestore(runErr, func() error { return nil })

		// --- Then ---
		assert.Same(t, runErr, err)
	})

	t.Run("error - restore failed", func(t *testing.T) {
		// --- Given ---
		rstErr := errors.New("restore")

		// --- When ---
		err := joinRestore(nil, func() error { return rstErr })

		// --- Then ---
		assert.ErrorIs(t, rstErr, err)
		assert.ErrorEqual(t, "gomake: restore generated: restore", err)
	})

	t.Run("error - both failed", func(t *testing.T) {
		// --- Given ---
		runErr := errors.New("run")
		rstErr := errors.New("restore")

		// --- When ---
		err := joinRestore(runErr, func() error { return rstErr })

		// --- Then ---
		assert.ErrorIs(t, runErr, err)
		assert.ErrorIs(t, rstErr, err)
	})
}

func Test_snapshotGenerated(t *testing.T) {
	t.Run("restores overwritten generated files", func(t *testing.T) {
		// --- Given ---
		build := t.TempDir()
		dir := oskit.MkdirAll(t, build, "internal", "builtin")
		data := oskit.MkdirAll(t, dir, "data")
		oskit.Write(t, "imports: original\n", build, "targets.yaml")
		oskit.Write(t, "package builtin // original\n", dir, "targets.go")
		oskit.Write(t, "// original main\n", data, "targets_main.go_")

		restore := must.Value(snapshotGenerated(build))

		// The build overwrites the generated files.
		oskit.Write(t, "imports: changed\n", build, "targets.yaml")
		oskit.Write(t, "package builtin // changed\n", dir, "targets.go")

		// --- When ---
		err := restore()

		// --- Then ---
		assert.NoError(t, err)
		have := oskit.ReadFileStr(t, build, "targets.yaml")
		assert.Equal(t, "imports: original\n", have)
		have = oskit.ReadFileStr(t, dir, "targets.go")
		assert.Equal(t, "package builtin // original\n", have)
	})

	t.Run("restores go.mod and go.sum", func(t *testing.T) {
		// --- Given ---
		// The devel install runs go get in the working tree, mutating go.mod
		// and go.sum; restore must return both to their pre-build contents.
		build := t.TempDir()
		oskit.MkdirAll(t, build, "internal", "builtin", "data")
		oskit.Write(t, "module example.test\n\ngo 1.24\n", build, "go.mod")
		oskit.Write(t, "h1:original\n", build, "go.sum")

		restore := must.Value(snapshotGenerated(build))

		// go get rewrites both files during the build.
		mod := "module example.test\n\ngo 1.24\n\nrequire x v1\n"
		oskit.Write(t, mod, build, "go.mod")
		oskit.Write(t, "h1:changed\n", build, "go.sum")

		// --- When ---
		err := restore()

		// --- Then ---
		assert.NoError(t, err)
		have := oskit.ReadFileStr(t, build, "go.mod")
		assert.Equal(t, "module example.test\n\ngo 1.24\n", have)
		assert.Equal(t, "h1:original\n", oskit.ReadFileStr(t, build, "go.sum"))
	})

	t.Run("removes a file created after snapshot", func(t *testing.T) {
		// --- Given ---
		// targets.go is absent at snapshot time; the build then creates it.
		// Restore must delete it so the tree stays clean.
		build := t.TempDir()
		dir := oskit.MkdirAll(t, build, "internal", "builtin", "data")

		restore := must.Value(snapshotGenerated(build))

		gen := filepath.Join(filepath.Dir(dir), "targets.go")
		oskit.Write(t, "package builtin\n", gen)

		// --- When ---
		err := restore()

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, oskit.PathExists(t, gen))
	})

	t.Run("error - a generated path cannot be read", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root ignores file permissions")
		}

		// --- Given ---
		build := t.TempDir()
		pth := oskit.Write(t, "imports:\n", build, "targets.yaml")
		must.Nil(os.Chmod(pth, 0o000))
		t.Cleanup(func() { _ = os.Chmod(pth, 0o644) })

		// --- When ---
		_, err := snapshotGenerated(build)

		// --- Then ---
		assert.ErrorIs(t, os.ErrPermission, err)
	})
}

func Test_copyToTemp(t *testing.T) {
	t.Run("copies source and returns cleanup", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		oskit.Write(t, "hello", src, "a.txt")

		// --- When ---
		hDst, hCleanup, err := copyToTemp(ring.New(), src)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileContain(t, "hello", filepath.Join(hDst, "a.txt"))

		hCleanup()

		assert.False(t, oskit.PathExists(t, hDst))
	})

	t.Run("error - missing source", func(t *testing.T) {
		// --- Given ---
		src := filepath.Join(t.TempDir(), "missing")

		// --- When ---
		_, _, err := copyToTemp(ring.New(), src)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
	})

	t.Run("error - temp dir cannot be created", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		rng := ring.New()
		rng.EnvSet("TMPDIR", filepath.Join(t.TempDir(), "nonexistent"))

		// --- When ---
		_, _, err := copyToTemp(rng, src)

		// --- Then ---
		assert.ErrorIs(t, os.ErrNotExist, err)
	})
}

func Test_tempRoot(t *testing.T) {
	t.Run("ring value", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("TMPDIR", "/ring/tmp")
		rng.EnvSet("TMP", "/ring/tmp")
		rng.EnvSet("TEMP", "/ring/tmp")

		// --- When ---
		have := tempRoot(rng)

		// --- Then ---
		assert.Equal(t, "/ring/tmp", have)
	})

	t.Run("unset", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvUnset("TMPDIR")
		rng.EnvUnset("TMP")
		rng.EnvUnset("TEMP")

		// --- When ---
		have := tempRoot(rng)

		// --- Then ---
		assert.Equal(t, os.TempDir(), have)
	})
}

func Test_copyDir(t *testing.T) {
	t.Run("copies files and makes them writable", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		oskit.Write(t, "hello", src, "a.txt")
		oskit.MkdirAll(t, src, "sub")
		oskit.Write(t, "world", src, "sub", "b.txt")

		dst := t.TempDir()

		// --- When ---
		err := copyDir(src, dst)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileContain(t, "hello", filepath.Join(dst, "a.txt"))
		assert.FileContain(t, "world", filepath.Join(dst, "sub", "b.txt"))
		assert.Equal(t, os.FileMode(0o600), oskit.Stat(t, dst, "a.txt").Mode())
	})

	t.Run("error - missing source", func(t *testing.T) {
		// --- Given ---
		src := filepath.Join(t.TempDir(), "missing")
		dst := t.TempDir()

		// --- When ---
		err := copyDir(src, dst)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
	})

	t.Run("error - source file not readable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root ignores file permissions")
		}

		// --- Given ---
		src := t.TempDir()
		pth := oskit.Write(t, "data", src, "a.txt")
		must.Nil(os.Chmod(pth, 0o000))
		t.Cleanup(func() { _ = os.Chmod(pth, 0o644) })

		dst := t.TempDir()

		// --- When ---
		err := copyDir(src, dst)

		// --- Then ---
		assert.ErrorIs(t, os.ErrPermission, err)
	})
}
