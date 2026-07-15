// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_PrepareExternalTargets(t *testing.T) {
	t.Run("an absent file generates empty targets", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t)
		dir := t.TempDir()
		oskit.MkdirAll(t, dir, "internal", "builtin", "data")

		// --- When ---
		err := prepareExternalTargets(rng.Ring(), dir, "")

		// --- Then ---
		assert.NoError(t, err)
		oskit.Stat(t, dir, "internal", "builtin", "targets.go")
	})

	t.Run("invalid config", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t)
		dir := t.TempDir()
		oskit.MkdirAll(t, dir, "internal", "builtin", "data")
		oskit.Write(t, `{bad json}`, dir, TargetsFile)

		// --- When ---
		err := prepareExternalTargets(rng.Ring(), dir, "")

		// --- Then ---
		assert.ErrorIs(t, errInvConfig, err)
	})

	t.Run("go get fails", func(t *testing.T) {
		// --- Given ---
		// No go.mod in dir so go get fails before touching the network.
		rng := ringtest.New(t)
		dir := t.TempDir()
		oskit.MkdirAll(t, dir, "internal", "builtin", "data")
		content := "imports:\n  - import: example.com/pkg@v1.0.0\n"
		oskit.Write(t, content, dir, TargetsFile)

		// --- When ---
		err := prepareExternalTargets(rng.Ring(), dir, "")

		// --- Then ---
		assert.ErrorContain(t, "go get example.com/pkg@v1.0.0", err)
	})
}

func Test_regenBuiltins(t *testing.T) {
	t.Run("empty specs", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t)
		dir := t.TempDir()
		oskit.MkdirAll(t, dir, "internal", "builtin", "data")
		cfg := &ImportsConfig{}

		// --- When ---
		err := regenBuiltins(rng.Ring(), dir, cfg)

		// --- Then ---
		assert.NoError(t, err)
		oskit.Stat(t, dir, "internal", "builtin", "targets.go")
	})

	t.Run("invalid import spec", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t)
		dir := t.TempDir()
		oskit.MkdirAll(t, dir, "internal", "builtin", "data")
		cfg := &ImportsConfig{
			imports: []ImportEntry{{Path: "not-a-valid-import-###"}},
		}

		// --- When ---
		err := regenBuiltins(rng.Ring(), dir, cfg)

		// --- Then ---
		assert.ErrorContain(t, "codegen builtins", err)
	})
}

func Test_regenBuiltins_writesToCompiledPackage(t *testing.T) {
	// regenBuiltins must regenerate the built-in targets into the same package
	// directory the gomake binary compiles. When the two drift apart (as when
	// the built-in package moved from pkg/builtin to internal/builtin but
	// regenBuiltins kept writing to pkg/builtin) external targets silently
	// never reach the installed binary. This pins regenBuiltins to the package
	// cmd/gomake actually imports and calls Generated() on.

	// --- Given ---
	root := goList(t, "-m", "-f", "{{.Dir}}", "github.com/ctx42/gomake")
	gomakeSrc := filepath.Join(root, "cmd", "gomake", "gomake.go")
	pkgDir := goList(t, "-f", "{{.Dir}}", builtinImportPath(t, gomakeSrc))
	rel, err := filepath.Rel(root, pkgDir)
	must.Nil(err)
	parts := strings.Split(rel, string(filepath.Separator))

	// A fresh, empty build tree; regenBuiltins creates the destination dir.
	wd := t.TempDir()
	rng := ringtest.New(t)

	// --- When ---
	err = regenBuiltins(rng.Ring(), wd, &ImportsConfig{})

	// --- Then ---
	assert.NoError(t, err)
	oskit.Stat(t, wd, append(append([]string{}, parts...), "targets.go")...)
}

func Test_regenBuiltins_resolvesImportFromBuildDir(t *testing.T) {
	// A published install copies the module source to a temp build tree and
	// runs from an unrelated working directory. regenBuiltins must resolve each
	// import spec against the build tree's go.mod, not the process working
	// directory, or `go list` reports "no required module provides package".

	// --- Given ---
	rng := ringtest.New(t)

	wd := t.TempDir()
	oskit.MkdirAll(t, wd, "fakepkg")
	oskit.Write(t, "module example.com/fakepkg\n\ngo 1.24\n", wd, "fakepkg",
		"go.mod")
	oskit.Write(t, "package fakepkg\n", wd, "fakepkg", "fakepkg.go")
	gomod := "module test.example.com\n\ngo 1.24\n\n" +
		"require example.com/fakepkg v0.0.0\n\n" +
		"replace example.com/fakepkg => ./fakepkg\n"
	oskit.Write(t, gomod, wd, "go.mod")
	oskit.MkdirAll(t, wd, "internal", "builtin", "data")

	// A working directory with no go.mod, mimicking a published install run
	// from wherever `go run ...@latest` was invoked.
	t.Chdir(t.TempDir())

	cfg := &ImportsConfig{
		imports: []ImportEntry{{Path: "example.com/fakepkg"}},
	}

	// --- When ---
	err := regenBuiltins(rng.Ring(), wd, cfg)

	// --- Then ---
	assert.NoError(t, err)
	oskit.Stat(t, wd, "internal", "builtin", "targets.go")
}

func Test_PrepareTargets(t *testing.T) {
	t.Run("the absent file succeeds and prints nothing", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t)
		dir := t.TempDir()
		oskit.MkdirAll(t, dir, "internal", "builtin", "data")

		// --- When ---
		err := PrepareTargets(rng.Ring(), dir, "")

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("invalid config returns error", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t)
		dir := t.TempDir()
		oskit.MkdirAll(t, dir, "internal", "builtin", "data")
		oskit.Write(t, `{bad json}`, dir, TargetsFile)

		// --- When ---
		err := PrepareTargets(rng.Ring(), dir, "")

		// --- Then ---
		assert.ErrorIs(t, errInvConfig, err)
	})
}

func Test_underModule_tabular(t *testing.T) {
	tt := []struct {
		testN string

		importPath string
		mod        string
		want       bool
	}{
		{"empty mod matches nothing", "example.com/mod", "", false},
		{"exact module match", "example.com/mod", "example.com/mod", true},
		{"package under module", "example.com/mod/pkg", "example.com/mod",
			true},
		{"unrelated module", "example.com/other", "example.com/mod", false},
		{"prefix but not a path boundary", "example.com/module",
			"example.com/mod", false},
		{"version suffix ignored", "example.com/mod/pkg@v1.2.0",
			"example.com/mod", true},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := underModule(tc.importPath, tc.mod)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_runGoInDir(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		env := ring.New()
		dir := t.TempDir()

		// --- When ---
		err := runGoInDir(env, dir, "version")

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error with output", func(t *testing.T) {
		// --- Given ---
		env := ring.New()
		dir := t.TempDir()

		// --- When ---
		err := runGoInDir(env, dir, "this-subcmd-does-not-exist")

		// --- Then ---
		// go echoes the unknown subcommand
		assert.ErrorContain(t, "this-subcmd-does-not-exist", err)
	})
}

func Test_runGoInDir_error_no_output(t *testing.T) {
	// --- Given ---
	env := ring.New()
	// A non-existent dir triggers a chdir failure before go runs; no output is
	// captured, so the function returns the bare exec error (the msg == ""
	// branch).
	dir := filepath.Join(t.TempDir(), "nonexistent")

	// --- When ---
	err := runGoInDir(env, dir, "version")

	// --- Then ---
	assert.Error(t, err)
}

// goList runs `go list` with args from the module tree and returns the
// single-trimmed line it prints, failing the test on error.
func goList(t tester.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"list"}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// builtinImportPath parses the gomake command source at path and returns the
// import path of the package whose Generated() it calls to load the built-in
// targets, failing the test when no such call is found.
func builtinImportPath(t tester.T, path string) string {
	t.Helper()
	f, err := goparser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	byName := make(map[string]string, len(f.Imports))
	for _, imp := range f.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		name := p[strings.LastIndex(p, "/")+1:]
		if imp.Name != nil {
			name = imp.Name.Name
		}
		byName[name] = p
	}
	var found string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Generated" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok {
			found = byName[id.Name]
			return false
		}
		return true
	})
	if found == "" {
		t.Fatalf("no builtin.Generated() call in %s", path)
	}
	return found
}
