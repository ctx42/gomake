// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"bytes"
	"io"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/iokit"
)

func Test_EnvSplit_tabular(t *testing.T) {
	tt := []struct {
		testN string

		env  []string
		want map[string]string
	}{
		{"1", []string{}, map[string]string{}},
		{"1a", []string{""}, map[string]string{}},
		{"2", []string{"A=B"}, map[string]string{"A": "B"}},
		{"3", []string{"A=B=C"}, map[string]string{"A": "B=C"}},
		{"4", []string{"A="}, map[string]string{"A": ""}},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := EnvSplit(tc.env)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_EnvSplitOrdered(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		env := []string{
			"key0=val0",
			"key1=val1",
			"key2=val2",
		}

		// --- When ---
		haveMap, haveOrder := EnvSplitOrdered(env)

		// --- Then ---
		wantMap := map[string]string{
			"key0": "val0",
			"key1": "val1",
			"key2": "val2",
		}
		assert.Equal(t, wantMap, haveMap)
		assert.Equal(t, []string{"key0", "key1", "key2"}, haveOrder)
	})

	t.Run("environment variable with empty value", func(t *testing.T) {
		// --- Given ---
		env := []string{
			"key0=val0",
			"key1=",
			"key2=val2",
		}

		// --- When ---
		haveMap, haveOrder := EnvSplitOrdered(env)

		// --- Then ---
		wantMap := map[string]string{
			"key0": "val0",
			"key1": "",
			"key2": "val2",
		}
		assert.Equal(t, wantMap, haveMap)
		assert.Equal(t, []string{"key0", "key1", "key2"}, haveOrder)
	})

	t.Run("environment variable with empty name", func(t *testing.T) {
		// --- Given ---
		env := []string{
			"key0=val0",
			"=val1",
			"key2=val2",
		}

		// --- When ---
		haveMap, haveOrder := EnvSplitOrdered(env)

		// --- Then ---
		wantMap := map[string]string{
			"key0": "val0",
			"key2": "val2",
		}
		assert.Equal(t, wantMap, haveMap)
		assert.Equal(t, []string{"key0", "key2"}, haveOrder)
	})

	t.Run("entry without equal sign", func(t *testing.T) {
		// --- Given ---
		env := []string{
			"key0=val0",
			"key1val1",
			"key2=val2",
		}

		// --- When ---
		haveMap, haveOrder := EnvSplitOrdered(env)

		// --- Then ---
		wantMap := map[string]string{
			"key0": "val0",
			"key2": "val2",
		}
		assert.Equal(t, wantMap, haveMap)
		assert.Equal(t, []string{"key0", "key2"}, haveOrder)
	})

	t.Run("empty entry is skipped", func(t *testing.T) {
		// --- Given ---
		env := []string{
			"key0=val0",
			"",
			"key2=val2",
		}

		// --- When ---
		haveMap, haveOrder := EnvSplitOrdered(env)

		// --- Then ---
		wantMap := map[string]string{
			"key0": "val0",
			"key2": "val2",
		}
		assert.Equal(t, wantMap, haveMap)
		assert.Equal(t, []string{"key0", "key2"}, haveOrder)
	})
}

func Test_EnvJoin_tabular(t *testing.T) {
	tt := []struct {
		testN string

		env  map[string]string
		want []string
	}{
		{"1", map[string]string{}, []string{}},
		{"2", map[string]string{"A": "B"}, []string{"A=B"}},
		{"3", map[string]string{"A": "B=C"}, []string{"A=B=C"}},
		{"4", map[string]string{"A": ""}, []string{"A="}},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := EnvJoin(tc.env)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_Expander(t *testing.T) {
	t.Run("key exists", func(t *testing.T) {
		// --- Given ---
		expander := Expander([]string{"key=val"})

		// --- When ---
		have := expander("key")

		// --- Then ---
		assert.Equal(t, "val", have)
	})

	t.Run("key does not exist", func(t *testing.T) {
		// --- Given ---
		expander := Expander([]string{"key=val"})

		// --- When ---
		have := expander("abc")

		// --- Then ---
		assert.Equal(t, "", have)
	})

	t.Run("nil env", func(t *testing.T) {
		// --- Given ---
		expander := Expander(nil)

		// --- When ---
		have := expander("key")

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_PrettyPrintEnv(t *testing.T) {
	t.Run("empty env", func(t *testing.T) {
		// --- Given ---
		buf := &bytes.Buffer{}

		// --- When ---
		err := PrettyPrintEnv(nil, buf)

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, buf.String())
	})

	t.Run("multiple values", func(t *testing.T) {
		// --- Given ---
		env := []string{
			"KEY0=VAL0",
			"KEY1=VAL1",
			"KEY2=VAL2",
		}
		buf := &bytes.Buffer{}

		// --- When ---
		err := PrettyPrintEnv(env, buf)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"KEY0    VAL0\n" +
			"KEY1    VAL1\n" +
			"KEY2    VAL2\n"
		assert.Equal(t, want, buf.String())
	})

	t.Run("does not print empty values", func(t *testing.T) {
		// --- Given ---
		env := []string{
			"KEY0=VAL0",
			"KEY1=",
			"KEY2=VAL2",
		}
		buf := &bytes.Buffer{}

		// --- When ---
		err := PrettyPrintEnv(env, buf)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"KEY0    VAL0\n" +
			"KEY2    VAL2\n"
		assert.Equal(t, want, buf.String())
	})

	t.Run("error - write failure propagates", func(t *testing.T) {
		// --- Given ---
		env := []string{"KEY0=VAL0"}
		buf := iokit.ErrWriter(io.Discard, 0)

		// --- When ---
		err := PrettyPrintEnv(env, buf)

		// --- Then ---
		assert.ErrorIs(t, iokit.ErrWrite, err)
	})
}
