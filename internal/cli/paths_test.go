// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/exekit"
)

func Test_GoBinPath(t *testing.T) {
	t.Run("toolchain", func(t *testing.T) {
		// --- Given ---
		raw := exekit.New(t).ExeStdout(
			"go", "env", "-json", "GOBIN", "GOPATH",
		)

		want := must.Value(goBinPath(raw))

		env := ring.New()

		// --- When ---
		have, err := GoBinPath(env)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, want, have)
	})

	t.Run("honors GOBIN from the passed environment", func(t *testing.T) {
		// --- Given ---
		want := t.TempDir()

		env := ring.New()
		env.EnvSet("GOBIN", want)

		// --- When ---
		have, err := GoBinPath(env)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, want, have)
	})
}

func Test_goBinPath_success_tabular(t *testing.T) {
	tt := []struct {
		testN string

		output string
		want   string
	}{
		{
			"GOBIN set",
			`{"GOBIN":"/custom/bin","GOPATH":"/home/user/go"}`,
			"/custom/bin",
		},
		{
			"GOBIN empty and GOPATH set",
			`{"GOBIN":"","GOPATH":"/home/user/go"}`,
			"/home/user/go/bin",
		},
		{
			"GOBIN empty and GOPATH with spaces and trailing slash",
			`{"GOBIN":"", "GOPATH":" /tmp/some/go/ "}`,
			"/tmp/some/go/bin",
		},
		{
			"GOPATH multiple entries takes first",
			`{"GOBIN":"","GOPATH":"/first/go:/second/go"}`,
			"/first/go/bin",
		},
		{
			"GOPATH leading separator uses first non-empty entry",
			`{"GOBIN":"","GOPATH":":/second/go"}`,
			"/second/go/bin",
		},
		{
			"GOBIN whitespace only falls back to GOPATH",
			`{"GOBIN":"   ","GOPATH":"/home/user/go"}`,
			"/home/user/go/bin",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have, err := goBinPath(tc.output)

			// --- Then ---
			assert.NoError(t, err)
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_goBinPath_error(t *testing.T) {
	t.Run("error - GOBIN and GOPATH empty", func(t *testing.T) {
		// --- Given ---
		output := `{"GOBIN":"","GOPATH":""}`

		// --- When ---
		have, err := goBinPath(output)

		// --- Then ---
		want := "cannot determine bin directory: GOBIN and GOPATH unusable"
		assert.ErrorEqual(t, want, err)
		assert.Empty(t, have)
	})

	t.Run("error - invalid json from go env", func(t *testing.T) {
		// --- Given ---
		output := `{"GOBIN": 42}`

		// --- When ---
		have, err := goBinPath(output)

		// --- Then ---
		assert.ErrorContain(t, "cannot parse 'go env' output", err)
		assert.Empty(t, have)
	})
}
