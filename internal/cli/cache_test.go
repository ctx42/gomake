// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

func Test_binaryCacheDir(t *testing.T) {
	t.Run("ring XDG_CACHE_HOME", func(t *testing.T) {
		// --- Given ---
		cache := t.TempDir()
		rng := ring.New()
		rng.EnvSet("XDG_CACHE_HOME", cache)

		// --- When ---
		have, err := binaryCacheDir(rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(cache, "gomake", "bin"), have)
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
		hA := must.Value(binaryCacheKey(
			ring.New(), root, mkf, "1.0", "linux", "amd64", ""),
		)
		hB := must.Value(binaryCacheKey(
			ring.New(), root, mkf, "1.0", "linux", "amd64", ""),
		)

		// --- Then ---
		assert.Equal(t, hA, hB)
		assert.Len(t, 64, hA)
	})

	t.Run("changes when ring GOFLAGS changes", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "module example.com/m\n", root, "go.mod")
		oskit.Write(t, "package main\n", root, "makefile.go")

		mkf := []string{"makefile.go"}

		base := ring.New()
		base.EnvSet("GOFLAGS", "")

		alt := ring.New()
		alt.EnvSet("GOFLAGS", "-tags=extra")

		// --- When ---
		hWithout := must.Value(binaryCacheKey(
			base, root, mkf, "1.0", "linux", "amd64", "",
		))
		hWith := must.Value(binaryCacheKey(
			alt, root, mkf, "1.0", "linux", "amd64", "",
		))

		// --- Then ---
		assert.NotEqual(t, hWithout, hWith)
	})

	t.Run("error - go.sum stat", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "module example.com/m\n", root, "go.mod")
		oskit.Write(t, "package main\n", root, "makefile.go")
		sum := filepath.Join(root, "go.sum")
		must.Nil(os.Symlink("go.sum", sum))

		mkf := []string{"makefile.go"}

		// --- When ---
		_, err := binaryCacheKey(
			ring.New(), root, mkf, "1.0", "linux", "amd64", "",
		)

		// --- Then ---
		assert.ErrorContain(t, "go.sum", err)
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
		hBefore := must.Value(binaryCacheKey(
			ring.New(), root, mkf, "1.0", "linux", "amd64", ""),
		)
		must.Nil(os.WriteFile(libPath, []byte("package lib\nconst V = 2\n"), 0o600))
		hAfter := must.Value(binaryCacheKey(
			ring.New(), root, mkf, "1.0", "linux", "amd64", ""),
		)

		// --- Then ---
		assert.NotEqual(t, hBefore, hAfter)
	})

	t.Run("changes when embedded file changes", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "module example.com/m\n", root, "go.mod")
		oskit.Write(t, "package main\n", root, "makefile.go")
		oskit.MkdirAll(t, root, "lib")
		src := "" +
			"package lib\n" +
			"\n" +
			"import \"embed\"\n" +
			"\n" +
			"//go:embed data.txt\n" +
			"var data embed.FS\n"
		oskit.Write(t, src, root, "lib", "lib.go")
		oskit.Write(t, "one\n", root, "lib", "data.txt")
		mkf := []string{"makefile.go"}

		// --- When ---
		hBefore := must.Value(binaryCacheKey(
			ring.New(), root, mkf, "1.0", "linux", "amd64", ""),
		)
		oskit.Create(t, "two\n", root, "lib", "data.txt")
		hAfter := must.Value(binaryCacheKey(
			ring.New(), root, mkf, "1.0", "linux", "amd64", ""),
		)

		// --- Then ---
		assert.NotEqual(t, hBefore, hAfter)
	})

	t.Run("changes when go.mod changes", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		modPath := filepath.Join(root, "go.mod")
		must.Nil(os.WriteFile(modPath, []byte("module example.com/m\n"), 0o600))

		oskit.Write(t, "package main\n", root, "makefile.go")
		mkf := []string{"makefile.go"}

		// --- When ---
		hBefore := must.Value(binaryCacheKey(
			ring.New(), root, mkf, "1.0", "linux", "amd64", ""),
		)
		must.Nil(os.WriteFile(modPath, []byte("module example.com/m\ngo 1.22\n"), 0o600))
		hAfter := must.Value(binaryCacheKey(
			ring.New(), root, mkf, "1.0", "linux", "amd64", ""),
		)

		// --- Then ---
		assert.NotEqual(t, hBefore, hAfter)
	})

	t.Run("changes when go.work appears", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "module example.com/m\n", root, "go.mod")
		oskit.Write(t, "package main\n", root, "makefile.go")

		mkf := []string{"makefile.go"}

		// --- When ---
		hBefore := must.Value(binaryCacheKey(
			ring.New(), root, mkf, "1.0", "linux", "amd64", ""),
		)
		oskit.Write(t, "go 1.22\nuse .\n", root, "go.work")
		hAfter := must.Value(binaryCacheKey(
			ring.New(), root, mkf, "1.0", "linux", "amd64", ""),
		)

		// --- Then ---
		assert.NotEqual(t, hBefore, hAfter)
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
		hBefore := must.Value(binaryCacheKey(
			ring.New(), proj, mkf, "1.0", "linux", "amd64", ""),
		)
		must.Nil(os.WriteFile(libPath, []byte("package other\nconst V = 2\n"), 0o600))
		hAfter := must.Value(binaryCacheKey(
			ring.New(), proj, mkf, "1.0", "linux", "amd64", ""),
		)

		// --- Then ---
		assert.NotEqual(t, hBefore, hAfter)
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
		hBefore := must.Value(binaryCacheKey(
			ring.New(), proj, mkf, "1.0", "linux", "amd64", ""),
		)
		must.Nil(os.WriteFile(libPath, []byte("package lib\nconst V = 2\n"), 0o600))
		hAfter := must.Value(binaryCacheKey(
			ring.New(), proj, mkf, "1.0", "linux", "amd64", ""),
		)

		// --- Then ---
		assert.NotEqual(t, hBefore, hAfter)
	})

	t.Run("changes when go.work replace tree changes", func(t *testing.T) {
		// --- Given ---
		base := t.TempDir()
		proj := oskit.MkdirAll(t, base, "project")
		lib := oskit.MkdirAll(t, base, "lib")
		oskit.Write(t, "module example.com/m\n", proj, "go.mod")
		oskit.Write(t, "package main\n", proj, "makefile.go")
		work := "" +
			"go 1.22\n" +
			"use .\n" +
			"replace example.com/lib => ../lib\n"
		oskit.Write(t, work, proj, "go.work")

		oskit.Write(t, "module example.com/lib\n", lib, "go.mod")

		libPath := filepath.Join(lib, "lib.go")
		must.Nil(os.WriteFile(libPath, []byte("package lib\nconst V = 1\n"), 0o600))

		mkf := []string{"makefile.go"}

		// --- When ---
		hBefore := must.Value(binaryCacheKey(
			ring.New(), proj, mkf, "1.0", "linux", "amd64", ""),
		)
		must.Nil(os.WriteFile(libPath, []byte("package lib\nconst V = 2\n"), 0o600))
		hAfter := must.Value(binaryCacheKey(
			ring.New(), proj, mkf, "1.0", "linux", "amd64", ""),
		)

		// --- Then ---
		assert.NotEqual(t, hBefore, hAfter)
	})

	t.Run("error - missing makefile", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "module example.com/m\n", root, "go.mod")

		mkf := []string{"makefile.go"}

		// --- When ---
		_, err := binaryCacheKey(
			ring.New(), root, mkf, "1.0", "linux", "amd64", "",
		)

		// --- Then ---
		assert.ErrorIs(t, os.ErrNotExist, err)
	})

	t.Run("GOWORK off differs from auto workspace", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "module example.com/m\n", root, "go.mod")
		oskit.Write(t, "package main\n", root, "makefile.go")
		oskit.Write(t, "go 1.22\nuse .\n", root, "go.work")

		mkf := []string{"makefile.go"}

		// --- When ---
		hWithWS := must.Value(binaryCacheKey(
			ring.New(), root, mkf, "1.0", "linux", "amd64", ""),
		)
		hOff := must.Value(binaryCacheKey(
			ring.New(), root, mkf, "1.0", "linux", "amd64", "off"),
		)

		// --- Then ---
		assert.NotEqual(t, hWithWS, hOff)
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

func Test_absWorkPaths(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		// --- When ---
		have := absWorkPaths("")

		// --- Then ---
		assert.Nil(t, have)
	})

	t.Run("missing file", func(t *testing.T) {
		// --- Given ---
		path := filepath.Join(t.TempDir(), "go.work")

		// --- When ---
		have := absWorkPaths(path)

		// --- Then ---
		assert.Nil(t, have)
	})

	t.Run("resolves relative and keeps absolute", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		content := "" +
			"go 1.22\n" +
			"use ../other\n" +
			"replace example.com/abs => \"/tmp/my lib\"\n"
		path := oskit.Write(t, content, root, "go.work")

		// --- When ---
		have := absWorkPaths(path)

		// --- Then ---
		want := []string{
			filepath.Join(root, "../other"),
			"/tmp/my lib",
		}
		assert.Equal(t, want, have)
	})
	t.Run("use forms", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		content := "" +
			"go 1.22\n" +
			"use .\n" +
			"use \"../my work\"\n" +
			"use (\n" +
			"  ./a\n" +
			"  ./b // comment\n" +
			"  \"./c d\" // comment\n" +
			")\n"
		path := oskit.Write(t, content, root, "go.work")

		// --- When ---
		have := absWorkPaths(path)

		// --- Then ---
		want := []string{
			root,
			filepath.Join(root, "../my work"),
			filepath.Join(root, "a"),
			filepath.Join(root, "b"),
			filepath.Join(root, "c d"),
		}
		assert.Equal(t, want, have)
	})

	t.Run("malformed file", func(t *testing.T) {
		// --- Given ---
		path := oskit.Write(t, "use (\n", t.TempDir(), "go.work")

		// --- When ---
		have := absWorkPaths(path)

		// --- Then ---
		assert.Nil(t, have)
	})
}

func Test_localPathsFromGoMod(t *testing.T) {
	// --- Given ---
	root := t.TempDir()
	content := "" +
		"module example.com/m\n" +
		"replace example.com/lib => ../lib\n" +
		"replace example.com/v => example.com/v v1.2.3\n" +
		"replace example.com/sp => \"../my lib\"\n" +
		"replace example.com/sl => \"../foo//bar\" // keep\n" +
		"replace example.com/abs => \"/tmp/my lib\"\n" +
		"replace (\n" +
		"  example.com/a => ./a\n" +
		"  example.com/b => \"./dir with space\"\n" +
		")\n"
	path := oskit.Write(t, content, root, "go.mod")

	// --- When ---
	have := localPathsFromGoMod(path)

	// --- Then ---
	want := []string{
		"../lib",
		"../my lib",
		"../foo//bar",
		"/tmp/my lib",
		"./a",
		"./dir with space",
	}
	assert.Equal(t, want, have)
}

func Test_localReplacePaths(t *testing.T) {
	// --- Given ---
	rpls := []*modfile.Replace{
		{New: module.Version{Path: "../lib"}},
		{New: module.Version{Path: "example.com/v", Version: "v1.2.3"}},
		{New: module.Version{Path: "/abs"}},
	}

	// --- When ---
	have := localReplacePaths(rpls)

	// --- Then ---
	assert.Equal(t, []string{"../lib", "/abs"}, have)
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

func Test_walkCacheFiles(t *testing.T) {
	t.Run("kept files sorted", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "", root, "b.go")
		oskit.Write(t, "", root, "a.txt")
		oskit.Write(t, "", oskit.MkdirAll(t, root, "sub"), "c.go")
		oskit.Write(t, "", oskit.MkdirAll(t, root, ".git"), "d.go")
		keep := func(name string) bool { return strings.HasSuffix(name, ".go") }

		// --- When ---
		have, err := walkCacheFiles(root, keep)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			filepath.Join(root, "b.go"),
			filepath.Join(root, "sub", "c.go"),
		}
		assert.Equal(t, want, have)
	})

	t.Run("error - missing root", func(t *testing.T) {
		// --- Given ---
		root := filepath.Join(t.TempDir(), "missing")
		keep := func(string) bool { return true }

		// --- When ---
		_, err := walkCacheFiles(root, keep)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
	})
}

func Test_cacheLabel(t *testing.T) {
	// --- When ---
	have := cacheLabel("/root", filepath.Join("/root", "a", "b.txt"))

	// --- Then ---
	assert.Equal(t, "a/b.txt", have)
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
		// --- When ---
		have := findModuleRoot(t.TempDir())

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_lookupBinaryCache_miss(t *testing.T) {
	// --- Given ---
	root := t.TempDir()
	oskit.Write(t, "module example.com/m\n", root, "go.mod")
	oskit.Write(t, "package main\n", root, "makefile.go")

	items := []string{"makefile.go"}

	// --- When ---
	hPth, hOk := lookupBinaryCache(ring.New(),
		root, items, "1.0", "linux", "amd64", "",
	)

	// --- Then ---
	assert.False(t, hOk)
	assert.Equal(t, "", hPth)
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
	storeBinaryCache(ring.New(), bin, root, mkf, "1.0", "linux", "amd64", "")
	hPth, hOk := lookupBinaryCache(
		ring.New(), root, mkf, "1.0", "linux", "amd64", "",
	)

	// --- Then ---
	assert.True(t, hOk)
	assert.True(t, filepath.IsAbs(hPth))
	assert.Equal(t, "fake-binary", oskit.ReadFileStr(t, hPth))
}
