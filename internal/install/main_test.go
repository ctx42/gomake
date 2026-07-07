// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package install

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
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
		err := Main(rng, info, "")

		// --- Then ---
		assert.NoError(t, err)
		pth := filepath.Join(gobin, "gomake")
		assert.True(t, oskit.PathExists(t, pth))
		// The source tree is unchanged: no targets.yaml was written and no
		// builtins were regenerated into it.
		assert.False(t, oskit.PathExists(t, filepath.Join(src, "targets.yaml")))
		assert.False(t, oskit.PathExists(t, filepath.Join(src, "pkg")))
	})

	t.Run("error - GOBIN cannot be resolved", func(t *testing.T) {
		// --- Given ---
		// With no `go` on PATH, GoBinPath cannot resolve the destination, so
		// Main fails before doing any work.
		t.Chdir(t.TempDir())
		t.Setenv("PATH", t.TempDir())
		rng := ringtest.New(t)

		// --- When ---
		err := Main(rng.Ring(), nil, "")

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

	t.Run("error - build info unavailable", func(t *testing.T) {
		// --- Given ---
		t.Chdir(t.TempDir())
		rng := ringtest.New(t)

		// --- When ---
		err := installTo(rng.Ring(), nil, t.TempDir(), "")

		// --- Then ---
		assert.ErrorContain(t, "build info unavailable", err)
	})

	t.Run("error - nonexistent targets file", func(t *testing.T) {
		// --- Given ---
		// The devel install path builds from the current working directory
		// and writes artifacts (targets.yaml, regenerated builtins) into it.
		// Run inside a temp dir so nothing lands in the real project tree.
		t.Chdir(t.TempDir())
		rng := ringtest.New(t)
		pth := filepath.Join(t.TempDir(), "missing.yaml")
		info, _ := debug.ReadBuildInfo()

		// --- When ---
		err := installTo(rng.Ring(), info, t.TempDir(), pth)

		// --- Then ---
		assert.ErrorContain(t, "read targets", err)
	})

	t.Run("error - imports route through full path", func(t *testing.T) {
		// --- Given ---
		// A --targets file with an import must take the full path: the
		// effective targets.yaml is written (inline os.WriteFile) and
		// PrepareTargets (go get + regen) runs before the build. No go.mod in
		// the tree makes go get fail before touching the network, proving the
		// fast path was not taken (it would skip PrepareTargets entirely).
		src := t.TempDir()
		t.Chdir(src)
		rng := ringtest.New(t)
		tgs := filepath.Join(t.TempDir(), "targets.yaml")
		oskit.Write(t, "imports:\n  - import: example.com/pkg@v1.0.0\n", tgs)
		info, _ := debug.ReadBuildInfo()

		// --- When ---
		err := installTo(rng.Ring(), info, t.TempDir(), tgs)

		// --- Then ---
		assert.ErrorContain(t, "go get example.com/pkg@v1.0.0", err)
		// The effective targets.yaml was written into the build tree.
		pth := filepath.Join(src, "targets.yaml")
		assert.FileContain(t, "example.com/pkg", pth)
	})

	t.Run("error - targets file cannot be written", func(t *testing.T) {
		// --- Given ---
		// A read-only working directory causes os.WriteFile to fail when
		// installTo writes the effective targets.yaml into the build tree.
		src := t.TempDir()
		if err := os.Chmod(src, 0o555); err != nil {
			t.Skip("cannot make dir read-only:", err)
		}
		t.Cleanup(func() { _ = os.Chmod(src, 0o755) })
		t.Chdir(src)
		rng := ringtest.New(t)
		tgs := filepath.Join(t.TempDir(), "targets.yaml")
		oskit.Write(t, "imports:\n  - import: example.com/pkg@v1.0.0\n", tgs)
		info, _ := debug.ReadBuildInfo()

		// --- When ---
		err := installTo(rng.Ring(), info, t.TempDir(), tgs)

		// --- Then ---
		assert.ErrorContain(t, "write targets", err)
	})

	t.Run("error - non-devel module download", func(t *testing.T) {
		// --- Given ---
		// A published-version build info with an unknown module causes
		// moduleCacheDir to fail; installTo must propagate the error.
		t.Chdir(t.TempDir())
		rng := ringtest.New(t)
		info := &debug.BuildInfo{
			Main: debug.Module{Path: "example.invalid/mod", Version: "v0.0.1"},
		}

		// --- When ---
		err := installTo(rng.Ring(), info, t.TempDir(), "")

		// --- Then ---
		assert.ErrorContain(t, "module download example.invalid/mod", err)
	})

	t.Run("error - non-devel temp dir cannot be created", func(t *testing.T) {
		// --- Given ---
		// A published-version build with imports causes copyToTemp to run.
		// Setting TMPDIR to a non-existent path makes os.MkdirTemp fail, which
		// installTo must propagate. All temp dirs are resolved before TMPDIR is
		// changed so the test's own scratch space is unaffected.
		t.Chdir(t.TempDir())
		rng := ringtest.New(t)
		dst := t.TempDir()
		tgs := filepath.Join(t.TempDir(), "targets.yaml")
		oskit.Write(t, "imports:\n  - import: example.com/pkg@v1.0.0\n", tgs)
		t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "nonexistent"))

		// --- When ---
		err := installTo(rng.Ring(), publishedInfo, dst, tgs)

		// --- Then ---
		assert.Error(t, err)
	})

	t.Run("error - non-devel full path copy then regen", func(t *testing.T) {
		// --- Given ---
		// A published-version build with imports copies the module source to a
		// temp dir (covering the copyToTemp success path), then PrepareTargets
		// fails because go get cannot resolve the fake import.
		t.Chdir(t.TempDir())
		rng := ringtest.New(t)
		tgs := filepath.Join(t.TempDir(), "targets.yaml")
		oskit.Write(t, "imports:\n  - import: example.com/pkg@v1.0.0\n", tgs)

		// --- When ---
		err := installTo(rng.Ring(), publishedInfo, t.TempDir(), tgs)

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
		// target is covered by Test_regenBuiltins_writesToCompiledPackage.
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
		err := installTo(rng, info, dst, tgs)

		// --- Then ---
		assert.NoError(t, err)
		pth := filepath.Join(dst, "gomake")
		assert.True(t, oskit.PathExists(t, pth))
		assert.Contain(
			t,
			"adding external target example.com/fakepkg",
			tst.Stderr(),
		)
		// The regenerated targets landed in the package the binary compiles,
		// not the pre-refactor pkg/builtin.
		gen := filepath.Join(src, "internal", "builtin", "targets.go")
		assert.True(t, oskit.PathExists(t, gen))
		assert.False(t, oskit.PathExists(t, filepath.Join(src, "pkg")))
	})
}

func Test_installTo_compilesExternalTargetIntoBinary(t *testing.T) {
	// End-to-end: install the real gomake module (a copy, so the working tree
	// is left untouched) with a local external-target module wired via replace,
	// then run the built binary and confirm the external target is compiled in.
	// This is the only case that drives the real internal/mkf types through the
	// generated builtins; the fake-module cases above cannot, because a foreign
	// module may not import gomake internals.
	if testing.Short() {
		t.Skip("slow: copies the module and runs go build")
	}

	// --- Given ---
	root := goListDir(t, "-m", "-f", "{{.Dir}}", "github.com/ctx42/gomake")
	build := t.TempDir()
	copyTree(t, root, build)

	// A local module exposing one gomake target, wired via a replace so go get
	// resolves it without the network.
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
	err := installTo(tst.Ring(), info, dst, tgs)

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
	out := runBin(t, bin, "--list")
	assert.Contain(t, "pingxyzzy", out)
}

func Test_effectiveImports(t *testing.T) {
	t.Run("no flag loads source targets.yaml", func(t *testing.T) {
		// --- Given ---
		srcDir := t.TempDir()
		content := "imports:\n  - import: a.com/x\n"
		oskit.Write(t, content, srcDir, "targets.yaml")

		// --- When ---
		cfg, err := effectiveImports(srcDir, "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"a.com/x"}, cfg.Paths())
	})

	t.Run("no flag, an absent source yields empty config", func(t *testing.T) {
		// --- When ---
		cfg, err := effectiveImports(t.TempDir(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 0, len(cfg.Imports()))
	})

	t.Run("flag file overrides and exposes raw bytes", func(t *testing.T) {
		// --- Given ---
		srcDir := t.TempDir()
		content := "imports:\n  - import: src.com/x\n"
		oskit.Write(t, content, srcDir, "targets.yaml")
		flagFile := filepath.Join(t.TempDir(), "flag.yaml")
		oskit.Write(t, "imports:\n  - import: flag.com/y\n", flagFile)

		// --- When ---
		cfg, err := effectiveImports(srcDir, flagFile)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "imports:\n  - import: flag.com/y\n", string(cfg.Raw()))
		assert.Equal(t, []string{"flag.com/y"}, cfg.Paths())
	})

	t.Run("error - missing flag file", func(t *testing.T) {
		// --- Given ---
		flagFile := filepath.Join(t.TempDir(), "missing.yaml")

		// --- When ---
		_, err := effectiveImports(t.TempDir(), flagFile)

		// --- Then ---
		assert.ErrorContain(t, "read targets", err)
	})

	t.Run("error - invalid flag file", func(t *testing.T) {
		// --- Given ---
		flagFile := filepath.Join(t.TempDir(), "bad.yaml")
		oskit.Write(t, "{", flagFile)

		// --- When ---
		_, err := effectiveImports(t.TempDir(), flagFile)

		// --- Then ---
		assert.ErrorContain(t, "invalid external targets config", err)
	})
}

func Test_moduleCacheDir(t *testing.T) {
	t.Run("returns directory for a cached module", func(t *testing.T) {
		// --- Given ---
		env := ring.New()

		// --- When ---
		dir, err := moduleCacheDir(env, "github.com/ctx42/ring@v0.7.0")

		// --- Then ---
		assert.NoError(t, err)
		assert.NotEqual(t, "", dir)
	})

	t.Run("error - invalid module", func(t *testing.T) {
		// --- Given ---
		env := ring.New()

		// --- When ---
		_, err := moduleCacheDir(env, "not.a.module@!!!")

		// --- Then ---
		// Pin the offending module so the assertion fails if a sibling
		// path (e.g. a missing go binary) produced the error instead.
		assert.ErrorContain(t, "module download not.a.module@!!!", err)
	})

	t.Run("error - go binary not in PATH", func(t *testing.T) {
		// --- Given ---
		env := ring.New()
		t.Setenv("PATH", t.TempDir())

		// --- When ---
		_, err := moduleCacheDir(env, "example.com/mod@v1.0.0")

		// --- Then ---
		// The exec-not-found cause is unique to this path; a plain
		// "module download" wrapper is shared with the ExitError branch.
		assert.ErrorContain(t, "executable file not found", err)
	})
}

func Test_copyToTemp(t *testing.T) {
	t.Run("copies source and returns cleanup", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		oskit.Write(t, "hello", src, "a.txt")

		// --- When ---
		dst, cleanup, err := copyToTemp(src)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileContain(t, "hello", filepath.Join(dst, "a.txt"))
		cleanup()
		assert.False(t, oskit.PathExists(t, dst))
	})

	t.Run("error - missing source", func(t *testing.T) {
		// --- Given ---
		src := filepath.Join(t.TempDir(), "missing")

		// --- When ---
		_, _, err := copyToTemp(src)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
	})

	t.Run("error - temp dir cannot be created", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "nonexistent"))

		// --- When ---
		_, _, err := copyToTemp(src)

		// --- Then ---
		assert.Error(t, err)
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
		assert.Equal(t, os.FileMode(0o644), oskit.Stat(t, dst, "a.txt").Mode())
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
		// --- Given ---
		src := t.TempDir()
		pth := filepath.Join(src, "a.txt")
		oskit.Write(t, "data", pth)
		_ = os.Chmod(pth, 0o000)
		t.Cleanup(func() { _ = os.Chmod(pth, 0o644) })
		dst := t.TempDir()

		// --- When ---
		err := copyDir(src, dst)

		// --- Then ---
		assert.Error(t, err)
	})
}
