// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/exekit"
	"github.com/ctx42/testkit/pkg/iokit"
)

func Test_ExitStatus(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		// --- When ---
		code := ExitStatus(nil)

		// --- Then ---
		assert.Equal(t, 0, code)
	})

	t.Run("has ExitStatus method", func(t *testing.T) {
		// --- Given ---
		tmp := TError{Err: "test error", ExStatus: 123}

		// --- When ---
		code := ExitStatus(tmp)

		// --- Then ---
		assert.Equal(t, 123, code)
	})

	t.Run("is ExitError instance", func(t *testing.T) {
		// --- Given ---
		cmd := exec.Command(os.Args[0], "--exitCode", "99")
		err := cmd.Run()

		// --- When ---
		code := ExitStatus(err)

		// --- Then ---
		assert.Equal(t, 99, code)
	})

	t.Run("exit code 0", func(t *testing.T) {
		// --- Given ---
		sout, eout := iokit.WetBuffer(t), iokit.DryBuffer(t)
		cmd := exec.Command(os.Args[0], "--exitCode", "0", "--toStdout", "abc")
		cmd.Stdout = sout
		cmd.Stderr = eout
		cmd.Env = exekit.MaybeAddGoCovDir(os.Environ(), os.Args, t.TempDir)

		err := cmd.Run()

		// --- When ---
		code := ExitStatus(err)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "|sout: abc|", sout.String())
	})

	t.Run("unknown error", func(t *testing.T) {
		// --- When ---
		code := ExitStatus(ErrTest)

		// --- Then ---
		assert.Equal(t, 1, code)
	})
}

func Test_HasRun(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		// --- When ---
		ran := HasRun(nil)

		// --- Then ---
		assert.True(t, ran)
	})

	t.Run("unknown error", func(t *testing.T) {
		// --- When ---
		ran := HasRun(ErrTest)

		// --- Then ---
		assert.False(t, ran)
	})

	t.Run("not run", func(t *testing.T) {
		// --- Given ---
		c := exec.Command("not-known-command")
		c.Stdout = io.Discard
		c.Stderr = io.Discard
		c.Stdin = os.Stdin

		// --- When ---
		ran := HasRun(c.Run())

		// --- Then ---
		assert.False(t, ran)
	})

	t.Run("forced exit", func(t *testing.T) {
		// --- Given ---
		ctx, cxl := context.WithTimeout(context.Background(), time.Second)
		defer cxl()

		c := exec.CommandContext(ctx, "sleep", "2")
		c.Stdout = io.Discard
		c.Stderr = io.Discard
		c.Stdin = os.Stdin

		// --- When ---
		ran := HasRun(c.Run())

		// --- Then ---
		assert.False(t, ran)
	})
}

func Test_GetGOOS(t *testing.T) {
	t.Run("GOOS from runtime", func(t *testing.T) {
		// --- Given ---
		want := runtime.GOOS
		var env []string

		// --- When ---
		have := GetGOOS(env)

		// --- Then ---
		assert.Equal(t, want, have)
	})

	t.Run("GOOS from environment", func(t *testing.T) {
		// --- Given ---
		env := []string{"KEY=VAL", "GOOS=my-os", "GOOS=your-os"}

		// --- When ---
		have := GetGOOS(env)

		// --- Then ---
		assert.Equal(t, "your-os", have)
	})
}

func Test_GetGOARCH(t *testing.T) {
	t.Run("GOARCH from runtime", func(t *testing.T) {
		// --- Given ---
		want := runtime.GOARCH
		env := make([]string, 0)

		// --- When ---
		have := GetGOARCH(env)

		// --- Then ---
		assert.Equal(t, want, have)
	})

	t.Run("GOARCH from environment", func(t *testing.T) {
		// --- Given ---
		env := []string{"KEY=VAL", "GOARCH=my-arch", "GOARCH=your-arch"}

		// --- When ---
		have := GetGOARCH(env)

		// --- Then ---
		assert.Equal(t, "your-arch", have)
	})
}

func Test_LookupEnv_tabular(t *testing.T) {
	tt := []struct {
		testN string

		env        []string
		findKey    string
		wantValue  string
		wantExists bool
	}{
		{"found", []string{"key0=val0", "key1=val1"}, "key1", "val1", true},
		{"not found", []string{"key0=val0", "key1=val1"}, "key9", "", false},
		{"partial", []string{"key0=val0", "key1=val1"}, "key", "", false},
		{"empty env", []string{}, "key", "", false},
		{"empty key", []string{"key0=val0", "key1=val1"}, "", "", false},
		{
			"last value counts",
			[]string{"key0=val0", "key1=val1", "key0=abc"},
			"key0",
			"abc",
			true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---

			// --- When ---
			haveValue, haveExists := LookupEnv(tc.env, tc.findKey)

			// --- Then ---
			assert.Equal(t, tc.wantValue, haveValue)
			assert.Equal(t, tc.wantExists, haveExists)
		})
	}
}

func Test_Getenv_tabular(t *testing.T) {
	tt := []struct {
		testN string

		env       []string
		findKey   string
		wantValue string
	}{
		{"found", []string{"key0=val0", "key1=val1"}, "key1", "val1"},
		{"not found", []string{"key0=val0", "key1=val1"}, "key9", ""},
		{"partial", []string{"key0=val0", "key1=val1"}, "key", ""},
		{"empty env", []string{}, "key", ""},
		{"empty key", []string{"key0=val0", "key1=val1"}, "", ""},
		{
			"last value counts",
			[]string{"key0=val0", "key1=val1", "key0=abc"},
			"key0",
			"abc",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---

			// --- When ---
			haveValue := Getenv(tc.env, tc.findKey)

			// --- Then ---
			assert.Equal(t, tc.wantValue, haveValue)
		})
	}
}
