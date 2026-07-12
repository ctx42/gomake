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
	tspy.AssertExpectations()

	assert.Has(t, "GOCACHE="+goCache(t), have)
	mayBe := []string{
		"GOROOT",
		"GO111MODULE",
		"GOPATH",
		"SHELL",
		"PATH",
		"HOME",
		"USER",
		"TERM",
		"SSH_AUTH_SOCK",
	}
	for _, wantKey := range mayBe {
		if val, exists := os.LookupEnv(wantKey); exists {
			assert.Has(t, wantKey+"="+val, have)
		}
	}
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

func Test_goCache(t *testing.T) {
	t.Run("system", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.Close()
		want := exekit.New(t).ExeStdout("go", "env", "GOCACHE")
		want = strings.TrimSpace(want)

		// --- When ---
		have := goCache(tspy)

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
		have := goCache(tspy)

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

	t.Run("error - invalid base", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectError()
		tspy.ExpectLogContain("missing protocol scheme")
		tspy.Close()

		// --- When ---
		have := JoinImpSpec(tspy, ":", "example.com/mod/pkg")

		// --- Then ---
		assert.Empty(t, have)
	})
}

func Test_rowColValue(t *testing.T) {
	t.Run("error - column not positive", func(t *testing.T) {
		tspy := tester.New(t)
		tspy.ExpectError()
		tspy.ExpectLogEqual("expected column to be positive, got: 0")
		tspy.Close()

		// --- When ---
		have := rowColValue(tspy, "header", 0, "header col1 col2")

		// --- Then ---
		assert.Equal(t, "", have)
	})

	t.Run("error - column out of range", func(t *testing.T) {
		tspy := tester.New(t)
		tspy.ExpectError()
		tspy.ExpectLogEqual("expected row to have at least 5 fields")
		tspy.Close()

		// --- When ---
		have := rowColValue(tspy, "header", 5, "header col1 col2")

		// --- Then ---
		assert.Equal(t, "", have)
	})

	t.Run("error - header row missing", func(t *testing.T) {
		tspy := tester.New(t)
		tspy.ExpectError()
		tspy.ExpectLogEqual("expected row with header \"header1\" to exist")
		tspy.Close()

		// --- When ---
		have := rowColValue(tspy, "header1", 5, "header0 col01 col02")

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_rowColValue_tabular(t *testing.T) {
	tt := []struct {
		testN string

		header string
		column int
		text   string
		want   string
	}{
		{
			"first column",
			"header0",
			1,
			"header0 col1 col2",
			"col1",
		},
		{
			"last column",
			"header0",
			2,
			"header0 col1 col2",
			"col2",
		},
		{
			"returned value is trimmed",
			"header0",
			2,
			"header0 col1  col2  ",
			"col2",
		},
		{
			"works with tabs",
			"header0",
			2,
			"header0\tcol1\tcol2",
			"col2",
		},
		{
			"works with tabs and spaces",
			"header0",
			2,
			"header0  \t  col1 \t col2 ",
			"col2",
		},
		{
			"finds row",
			"header1",
			2,
			"header0 col01 col02\nheader1 col11 col12\n",
			"col12",
		},
		{
			"handles empty lines",
			"header1",
			2,
			"header0 col01 col02\n\nheader1 col11 col12\n",
			"col12",
		},
		{
			"double column header",
			"header00 header01",
			2,
			"header00 header01 col01 col02",
			"col02",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			tspy := tester.New(t)
			tspy.Close()

			// --- When ---
			have := rowColValue(tspy, tc.header, tc.column, tc.text)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_infoToEnv(t *testing.T) {
	t.Run("split", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.Close()

		output := "" +
			"key0 value0\n" +
			"key1 value1\n"

		// --- When ---
		have := infoToEnv(tspy, output)

		// --- Then ---
		want := map[string]string{
			"key0": "value0",
			"key1": "value1",
		}
		assert.Equal(t, want, have)
	})

	t.Run("error - repeating keys", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectError()
		tspy.ExpectLogEqual("did not expect keys to repeat, key: \"key0\"")
		tspy.Close()

		output := "" +
			"key0 value0\n" +
			"key0 value1\n"

		// --- When ---
		have := infoToEnv(tspy, output)

		// --- Then ---
		want := map[string]string{
			"key0": "value1",
		}
		assert.Equal(t, want, have)
	})

	t.Run("error - more than two columns", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectError()
		wMsg := "" +
			"expected line to have two fields, " +
			"got: \"key1 value10 value11\""
		tspy.ExpectLogEqual(wMsg)
		tspy.Close()

		output := "" +
			"key0 value0\n" +
			"key1 value10 value11\n"

		// --- When ---
		have := infoToEnv(tspy, output)

		// --- Then ---
		want := map[string]string{
			"key0": "value0",
		}
		assert.Equal(t, want, have)
	})
}
