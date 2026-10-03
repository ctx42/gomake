// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"
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
		want := filepath.Join(cache, "gomake", "bin")
		assert.Equal(t, want, have)
	})
}

func Test_binaryCacheKey(t *testing.T) {
	t.Run("stable for same inputs", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "module example.com/m\n", root, "go.mod")
		oskit.Write(t, "package main\n", root, "makefile.go")

		mkf := []string{"makefile.go"}

		rng := ring.New()

		ver := "1.0"

		linux := "linux"

		amd64 := "amd64"

		blank := ""

		rng2 := ring.New()

		// --- When ---
		hA := must.Value(binaryCacheKey(
			rng, root, mkf, ver, linux, amd64, blank),
		)
		hB := must.Value(binaryCacheKey(
			rng2, root, mkf, ver, linux, amd64, blank),
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

		ver := "1.0"

		linux := "linux"

		amd64 := "amd64"

		blank := ""

		// --- When ---
		hWithout := must.Value(binaryCacheKey(
			base, root, mkf, ver, linux, amd64, blank,
		))
		hWith := must.Value(binaryCacheKey(
			alt, root, mkf, ver, linux, amd64, blank,
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

		rng := ring.New()

		ver := "1.0"

		linux := "linux"

		amd64 := "amd64"

		blank := ""

		// --- When ---
		_, err := binaryCacheKey(
			rng, root, mkf, ver, linux, amd64, blank,
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

		rng := ring.New()

		ver := "1.0"

		linux := "linux"

		amd64 := "amd64"

		blank := ""

		rng2 := ring.New()

		// --- When ---
		hBefore := must.Value(binaryCacheKey(
			rng, root, mkf, ver, linux, amd64, blank),
		)
		must.Nil(os.WriteFile(libPath, []byte("package lib\nconst V = 2\n"), 0o600))
		hAfter := must.Value(binaryCacheKey(
			rng2, root, mkf, ver, linux, amd64, blank),
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

		rng := ring.New()

		ver := "1.0"

		linux := "linux"

		amd64 := "amd64"

		blank := ""

		text := "two\n"

		lib := "lib"

		text2 := "data.txt"

		rng2 := ring.New()

		// --- When ---
		hBefore := must.Value(binaryCacheKey(
			rng, root, mkf, ver, linux, amd64, blank),
		)
		oskit.Create(t, text, root, lib, text2)
		hAfter := must.Value(binaryCacheKey(
			rng2, root, mkf, ver, linux, amd64, blank),
		)

		// --- Then ---
		assert.NotEqual(t, hBefore, hAfter)
	})

	t.Run(
		"ignores a file the embed directive does not name",
		func(t *testing.T) {
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

			oskit.Write(t, "other\n", root, "lib", "other.txt")

			mkf := []string{"makefile.go"}

			rng := ring.New()

			ver := "1.0"

			linux := "linux"

			amd64 := "amd64"

			blank := ""

			text := "changed\n"

			lib := "lib"

			text2 := "other.txt"

			rng2 := ring.New()

			// --- When ---
			hBefore := must.Value(binaryCacheKey(
				rng, root, mkf, ver, linux, amd64, blank),
			)
			oskit.Create(t, text, root, lib, text2)
			hAfter := must.Value(binaryCacheKey(
				rng2, root, mkf, ver, linux, amd64, blank),
			)

			// --- Then ---
			assert.Equal(t, hBefore, hAfter)
		})

	t.Run("changes when go.mod changes", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()

		modPath := filepath.Join(root, "go.mod")
		must.Nil(os.WriteFile(modPath, []byte("module example.com/m\n"), 0o600))

		oskit.Write(t, "package main\n", root, "makefile.go")

		mkf := []string{"makefile.go"}

		rng := ring.New()

		ver := "1.0"

		linux := "linux"

		amd64 := "amd64"

		blank := ""

		rng2 := ring.New()

		// --- When ---
		hBefore := must.Value(binaryCacheKey(
			rng, root, mkf, ver, linux, amd64, blank),
		)
		must.Nil(os.WriteFile(modPath, []byte("module example.com/m\ngo 1.22\n"), 0o600))
		hAfter := must.Value(binaryCacheKey(
			rng2, root, mkf, ver, linux, amd64, blank),
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

		rng := ring.New()

		ver := "1.0"

		linux := "linux"

		amd64 := "amd64"

		blank := ""

		text := "go 1.22\nuse .\n"

		text2 := "go.work"

		rng2 := ring.New()

		// --- When ---
		hBefore := must.Value(binaryCacheKey(
			rng, root, mkf, ver, linux, amd64, blank),
		)
		oskit.Write(t, text, root, text2)
		hAfter := must.Value(binaryCacheKey(
			rng2, root, mkf, ver, linux, amd64, blank),
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

		rng := ring.New()

		ver := "1.0"

		linux := "linux"

		amd64 := "amd64"

		blank := ""

		rng2 := ring.New()

		// --- When ---
		hBefore := must.Value(binaryCacheKey(
			rng, proj, mkf, ver, linux, amd64, blank),
		)
		must.Nil(os.WriteFile(libPath, []byte("package other\nconst V = 2\n"), 0o600))
		hAfter := must.Value(binaryCacheKey(
			rng2, proj, mkf, ver, linux, amd64, blank),
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

		rng := ring.New()

		ver := "1.0"

		linux := "linux"

		amd64 := "amd64"

		blank := ""

		rng2 := ring.New()

		// --- When ---
		hBefore := must.Value(binaryCacheKey(
			rng, proj, mkf, ver, linux, amd64, blank),
		)
		must.Nil(os.WriteFile(libPath, []byte("package lib\nconst V = 2\n"), 0o600))
		hAfter := must.Value(binaryCacheKey(
			rng2, proj, mkf, ver, linux, amd64, blank),
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

		rng := ring.New()

		ver := "1.0"

		linux := "linux"

		amd64 := "amd64"

		blank := ""

		rng2 := ring.New()

		// --- When ---
		hBefore := must.Value(binaryCacheKey(
			rng, proj, mkf, ver, linux, amd64, blank),
		)
		must.Nil(os.WriteFile(libPath, []byte("package lib\nconst V = 2\n"), 0o600))
		hAfter := must.Value(binaryCacheKey(
			rng2, proj, mkf, ver, linux, amd64, blank),
		)

		// --- Then ---
		assert.NotEqual(t, hBefore, hAfter)
	})

	t.Run("error - missing makefile", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "module example.com/m\n", root, "go.mod")

		mkf := []string{"makefile.go"}

		rng := ring.New()

		ver := "1.0"

		linux := "linux"

		amd64 := "amd64"

		blank := ""

		// --- When ---
		_, err := binaryCacheKey(
			rng, root, mkf, ver, linux, amd64, blank,
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

		rng := ring.New()

		ver := "1.0"

		linux := "linux"

		amd64 := "amd64"

		blank := ""

		rng2 := ring.New()

		off := "off"

		// --- When ---
		hWithWS := must.Value(binaryCacheKey(
			rng, root, mkf, ver, linux, amd64, blank),
		)
		hOff := must.Value(binaryCacheKey(
			rng2, root, mkf, ver, linux, amd64, off),
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

func Test_localPathsFromGoWork(t *testing.T) {
	// --- Given ---
	root := t.TempDir()

	content := "" +
		"go 1.22\n" +
		"use .\n" +
		"use ../other\n" +
		"use \"../my work\"\n" +
		"use (\n" +
		"  ./a\n" +
		"  ./b // comment\n" +
		"  \"./c d\" // comment\n" +
		")\n"

	path := oskit.Write(t, content, root, "go.work")

	// --- When ---
	have := localPathsFromGoWork(path)

	// --- Then ---
	want := []string{".", "../other", "../my work", "./a", "./b", "./c d"}
	assert.Equal(t, want, have)
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
		// --- Given ---
		dir := t.TempDir()

		// --- When ---
		have := findModuleRoot(dir)

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_lookupBinaryCache_miss(t *testing.T) {
	// --- Given ---
	root := t.TempDir()
	oskit.Write(t, "module example.com/m\n", root, "go.mod")
	oskit.Write(t, "package main\n", root, "makefile.go")

	rng := ring.New()

	items := []string{"makefile.go"}

	ver := "1.0"

	linux := "linux"

	amd64 := "amd64"

	blank := ""

	// --- When ---
	hPth, hOk := lookupBinaryCache(rng,
		root, items, ver, linux, amd64, blank,
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

	rng := ring.New()

	ver := "1.0"

	linux := "linux"

	amd64 := "amd64"

	blank := ""

	rng2 := ring.New()

	// --- When ---
	storeBinaryCache(
		rng, bin, root, mkf, ver, linux, amd64, blank,
	)
	hPth, hOk := lookupBinaryCache(
		rng2, root, mkf, ver, linux, amd64, blank,
	)

	// --- Then ---
	assert.True(t, hOk)
	assert.True(t, filepath.IsAbs(hPth))
	assert.Equal(t, "fake-binary", oskit.ReadFileStr(t, hPth))
}
