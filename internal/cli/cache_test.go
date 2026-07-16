// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_findModuleRoot(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		sub := oskit.MkdirAll(t, root, "pkg", "sub")
		oskit.Write(t, "module example.com/m\n", root, "go.mod")

		// --- When ---
		have := findModuleRoot(sub)

		// --- Then ---
		assert.Equal(t, root, have)
	})

	t.Run("missing", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()

		// --- When ---
		have := findModuleRoot(dir)

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_binaryCacheKey(t *testing.T) {
	t.Run("stable for same inputs", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "module example.com/m\n", root, "go.mod")
		oskit.Write(t, "package main\n", root, "makefile.go")
		mkf := []string{"makefile.go"}

		// --- When ---
		a := must.Value(binaryCacheKey(root, mkf, "1.0", "linux", "amd64", ""))
		b := must.Value(binaryCacheKey(root, mkf, "1.0", "linux", "amd64", ""))

		// --- Then ---
		assert.Equal(t, a, b)
		assert.Len(t, 64, a)
	})

	t.Run("changes when package go file changes", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "module example.com/m\n", root, "go.mod")
		oskit.Write(t, "package main\n", root, "makefile.go")
		pkgDir := oskit.MkdirAll(t, root, "lib")
		libPath := filepath.Join(pkgDir, "lib.go")
		must.Nil(os.WriteFile(libPath, []byte("package lib\nconst V = 1\n"), 0o600))
		mkf := []string{"makefile.go"}

		// --- When ---
		before := must.Value(binaryCacheKey(root, mkf, "1.0", "linux", "amd64", ""))
		must.Nil(os.WriteFile(libPath, []byte("package lib\nconst V = 2\n"), 0o600))
		after := must.Value(binaryCacheKey(root, mkf, "1.0", "linux", "amd64", ""))

		// --- Then ---
		assert.NotEqual(t, before, after)
	})

	t.Run("changes when go.mod changes", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		modPath := filepath.Join(root, "go.mod")
		must.Nil(os.WriteFile(modPath, []byte("module example.com/m\n"), 0o600))
		oskit.Write(t, "package main\n", root, "makefile.go")
		mkf := []string{"makefile.go"}

		// --- When ---
		before := must.Value(binaryCacheKey(root, mkf, "1.0", "linux", "amd64", ""))
		must.Nil(os.WriteFile(modPath, []byte("module example.com/m\ngo 1.22\n"), 0o600))
		after := must.Value(binaryCacheKey(root, mkf, "1.0", "linux", "amd64", ""))

		// --- Then ---
		assert.NotEqual(t, before, after)
	})

	t.Run("changes when go.work appears", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "module example.com/m\n", root, "go.mod")
		oskit.Write(t, "package main\n", root, "makefile.go")
		mkf := []string{"makefile.go"}

		// --- When ---
		before := must.Value(binaryCacheKey(root, mkf, "1.0", "linux", "amd64", ""))
		oskit.Write(t, "go 1.22\nuse .\n", root, "go.work")
		after := must.Value(binaryCacheKey(root, mkf, "1.0", "linux", "amd64", ""))

		// --- Then ---
		assert.NotEqual(t, before, after)
	})

	t.Run("changes when workspace sibling go file changes", func(t *testing.T) {
		// --- Given ---
		base := t.TempDir()
		proj := oskit.MkdirAll(t, base, "project")
		other := oskit.MkdirAll(t, base, "other")
		oskit.Write(t, "module example.com/m\n", proj, "go.mod")
		oskit.Write(t, "package main\n", proj, "makefile.go")
		oskit.Write(t, "go 1.22\nuse .\nuse ../other\n", proj, "go.work")
		oskit.Write(t, "module example.com/other\n", other, "go.mod")
		libPath := filepath.Join(other, "lib.go")
		must.Nil(os.WriteFile(libPath, []byte("package other\nconst V = 1\n"), 0o600))
		mkf := []string{"makefile.go"}

		// --- When ---
		before := must.Value(binaryCacheKey(proj, mkf, "1.0", "linux", "amd64", ""))
		must.Nil(os.WriteFile(libPath, []byte("package other\nconst V = 2\n"), 0o600))
		after := must.Value(binaryCacheKey(proj, mkf, "1.0", "linux", "amd64", ""))

		// --- Then ---
		assert.NotEqual(t, before, after)
	})

	t.Run("changes when local replace go file changes", func(t *testing.T) {
		// --- Given ---
		base := t.TempDir()
		proj := oskit.MkdirAll(t, base, "project")
		lib := oskit.MkdirAll(t, base, "lib")
		mod := "" +
			"module example.com/m\n" +
			"replace example.com/lib => ../lib\n"
		oskit.Write(t, mod, proj, "go.mod")
		oskit.Write(t, "package main\n", proj, "makefile.go")
		oskit.Write(t, "module example.com/lib\n", lib, "go.mod")
		libPath := filepath.Join(lib, "lib.go")
		must.Nil(os.WriteFile(libPath, []byte("package lib\nconst V = 1\n"), 0o600))
		mkf := []string{"makefile.go"}

		// --- When ---
		before := must.Value(binaryCacheKey(proj, mkf, "1.0", "linux", "amd64", ""))
		must.Nil(os.WriteFile(libPath, []byte("package lib\nconst V = 2\n"), 0o600))
		after := must.Value(binaryCacheKey(proj, mkf, "1.0", "linux", "amd64", ""))

		// --- Then ---
		assert.NotEqual(t, before, after)
	})

	t.Run("error - missing makefile", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "module example.com/m\n", root, "go.mod")
		mkf := []string{"makefile.go"}

		// --- When ---
		_, err := binaryCacheKey(root, mkf, "1.0", "linux", "amd64", "")

		// --- Then ---
		assert.Error(t, err)
	})

	t.Run("GOWORK off differs from auto workspace", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "module example.com/m\n", root, "go.mod")
		oskit.Write(t, "package main\n", root, "makefile.go")
		oskit.Write(t, "go 1.22\nuse .\n", root, "go.work")
		mkf := []string{"makefile.go"}

		// --- When ---
		withWS := must.Value(binaryCacheKey(root, mkf, "1.0", "linux", "amd64", ""))
		off := must.Value(binaryCacheKey(root, mkf, "1.0", "linux", "amd64", "off"))

		// --- Then ---
		assert.NotEqual(t, withWS, off)
	})
}

func Test_isSubpath_tabular(t *testing.T) {
	tt := []struct {
		test   string
		parent string
		child  string
		want   bool
	}{
		{"same", "/a/b", "/a/b", true},
		{"child", "/a/b", "/a/b/c", true},
		{"sibling prefix", "/a/b", "/a/bc", false},
		{"parent", "/a/b/c", "/a/b", false},
	}

	for _, tc := range tt {
		t.Run(tc.test, func(t *testing.T) {
			// --- When ---
			have := isSubpath(tc.parent, tc.child)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_isLocalDiskPath_tabular(t *testing.T) {
	tt := []struct {
		test string
		path string
		want bool
	}{
		{"dot", ".", true},
		{"dot slash", "./", true},
		{"parent", "../other", true},
		{"absolute", "/abs", true},
		{"module path", "example.com/lib", false},
		{"empty", "", false},
	}

	for _, tc := range tt {
		t.Run(tc.test, func(t *testing.T) {
			// --- When ---
			have := isLocalDiskPath(tc.path)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_localPathsFromGoWork(t *testing.T) {
	// --- Given ---
	root := t.TempDir()
	content := "" +
		"go 1.22\n" +
		"use .\n" +
		"use ../other\n" +
		"use (\n" +
		"  ./a\n" +
		"  ./b // comment\n" +
		")\n"
	path := oskit.Write(t, content, root, "go.work")

	// --- When ---
	have := localPathsFromGoWork(path)

	// --- Then ---
	want := []string{".", "../other", "./a", "./b"}
	assert.Equal(t, want, have)
}

func Test_localPathsFromGoMod(t *testing.T) {
	// --- Given ---
	root := t.TempDir()
	content := "" +
		"module example.com/m\n" +
		"replace example.com/lib => ../lib\n" +
		"replace example.com/v => example.com/v v1.2.3\n" +
		"replace (\n" +
		"  example.com/a => ./a\n" +
		")\n"
	path := oskit.Write(t, content, root, "go.mod")

	// --- When ---
	have := localPathsFromGoMod(path)

	// --- Then ---
	want := []string{"../lib", "./a"}
	assert.Equal(t, want, have)
}

func Test_shouldSkipCacheDir_tabular(t *testing.T) {
	tt := []struct {
		name string
		want bool
	}{
		{"vendor", true},
		{"node_modules", true},
		{".git", true},
		{"pkg", false},
		{"internal", false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// --- When ---
			have := shouldSkipCacheDir(tc.name)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_lookupBinaryCache_miss(t *testing.T) {
	// --- Given ---
	root := t.TempDir()
	oskit.Write(t, "module example.com/m\n", root, "go.mod")
	oskit.Write(t, "package main\n", root, "makefile.go")

	// --- When ---
	pth, ok := lookupBinaryCache(
		root, []string{"makefile.go"}, "1.0", "linux", "amd64", "",
	)

	// --- Then ---
	assert.False(t, ok)
	assert.Equal(t, "", pth)
}

func Test_storeBinaryCache_and_lookup(t *testing.T) {
	// --- Given ---
	// os.UserCacheDir honors XDG_CACHE_HOME on Linux.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	root := t.TempDir()
	oskit.Write(t, "module example.com/m\n", root, "go.mod")
	oskit.Write(t, "package main\n", root, "makefile.go")
	bin := oskit.Write(t, "fake-binary", root, "makefile")
	mkf := []string{"makefile.go"}

	// --- When ---
	storeBinaryCache(bin, root, mkf, "1.0", "linux", "amd64", "")
	pth, ok := lookupBinaryCache(root, mkf, "1.0", "linux", "amd64", "")

	// --- Then ---
	assert.True(t, ok)
	assert.True(t, filepath.IsAbs(pth))
	assert.Equal(t, "fake-binary", oskit.ReadFileStr(t, pth))
}
