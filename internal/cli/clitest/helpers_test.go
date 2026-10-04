// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package clitest

import (
	"os"
	"strings"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/exekit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/randkit"
)

func Test_TestEnv(t *testing.T) {
	// --- Given ---
	tspy := tester.New(t)
	tspy.ExpectTempDir(1)
	tspy.Close()

	// --- When ---
	have := TestEnv(tspy)

	// --- Then ---
	env := make(map[string]string, len(have))
	for _, kv := range have {
		key, val, _ := strings.Cut(kv, "=")
		env[key] = val
	}
	assert.Len(t, 0, oskit.List(t, env["XDG_CONFIG_HOME"]))
	tspy.AssertExpectations() // Removes the XDG_CONFIG_HOME temp dir.
	assert.Equal(t, goEnv(t, "GOCACHE"), env["GOCACHE"])
	assert.Equal(t, goEnv(t, "GOENV"), env["GOENV"])

	copied := []string{
		"GOROOT",
		"GO111MODULE",
		"GOPATH",
		"GOMODCACHE",
		"GOPROXY",
		"GOPRIVATE",
		"GONOPROXY",
		"GONOSUMDB",
		"GOINSECURE",
		"SHELL",
		"PATH",
		"HOME",
		"USER",
		"TERM",
		"SSH_AUTH_SOCK",
	}
	want := len(copied) + 3 // Plus GOCACHE, GOENV and XDG_CONFIG_HOME.
	for _, key := range copied {
		val, set := os.LookupEnv(key)
		if !set {
			want--
		}
		assert.Equal(t, val, env[key], key)
	}
	assert.Len(t, want, have)
}

func Test_fromEnv(t *testing.T) {
	t.Run("existing", func(t *testing.T) {
		// --- Given ---
		kv := randkit.Str()
		t.Setenv(kv, kv)

		env := make([]string, 0)

		// --- When ---
		have := fromEnv(kv, env)

		// --- Then ---
		assert.Len(t, 0, env)
		assert.Equal(t, []string{kv + "=" + kv}, have)
	})

	t.Run("not existing", func(t *testing.T) {
		// --- Given ---
		env := make([]string, 0)

		// --- When ---
		have := fromEnv(randkit.Str(), env)

		// --- Then ---
		assert.Len(t, 0, env)
		assert.Len(t, 0, have)
	})
}

func Test_goEnv(t *testing.T) {
	t.Run("system", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.Close()

		want := exekit.New(t).ExeStdout("go", "env", "GOCACHE")
		want = strings.TrimSpace(want)

		// --- When ---
		have := goEnv(tspy, "GOCACHE")

		// --- Then ---
		tspy.AssertExpectations()
		assert.Equal(t, want, have)
	})

	t.Run("custom", func(t *testing.T) {
		// --- Given ---
		want := t.TempDir()
		t.Setenv("GOCACHE", want)

		tspy := tester.New(t)
		tspy.Close()

		// --- When ---
		have := goEnv(tspy, "GOCACHE")

		// --- Then ---
		tspy.AssertExpectations()
		assert.Equal(t, want, have)
	})
}

func Test_findMakefiles(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.Close()

		// --- When ---
		have := findMakefiles(tspy, "testdata")

		// --- Then ---
		want := []string{
			"makefile.go",
			"makefile_helpers.go",
		}
		assert.Equal(t, want, have)
	})

	t.Run("directory named like a makefile", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.Close()

		dir := t.TempDir()
		oskit.MkdirAll(t, dir, "makefile_x.go")
		oskit.Write(t, "package x\n", dir, "makefile.go")

		// --- When ---
		have := findMakefiles(tspy, dir)

		// --- Then ---
		assert.Equal(t, []string{"makefile.go"}, have)
	})

	t.Run("error - missing directory", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectFatal()
		tspy.ExpectLogContain("no such file or directory")
		tspy.Close()

		fn := func() { findMakefiles(tspy, "testdata/missing") }

		// --- When ---
		assert.Panic(t, fn)
	})
}

func Test_JoinImpSpec(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.Close()

		// --- When ---
		have := JoinImpSpec(tspy, "example.com/mod/pkg", "abc", "def")

		// --- Then ---
		assert.Equal(t, "example.com/mod/pkg/abc/def", have)
	})

	t.Run("characters a URL escapes", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.Close()

		// --- When ---
		have := JoinImpSpec(tspy, "example.com/mod", "a b", "c%d")

		// --- Then ---
		assert.Equal(t, "example.com/mod/a b/c%d", have)
	})
}
