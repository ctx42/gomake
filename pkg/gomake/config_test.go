// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testing/pkg/tester"
)

// configFrom builds a [Config] from a JSON block by delivering it through the
// ring meta store as a string, mirroring how gomake feeds a target at run time.
func configFrom(t tester.T, jsn string) *Config {
	t.Helper()
	rng := ring.New()
	rng.MetaSet(ConfigMetaKey, jsn)
	return must.Value(TargetConfig(rng))
}

func Test_TargetConfig(t *testing.T) {
	t.Run("decodes byte block", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.MetaSet(ConfigMetaKey, []byte(`{"region":"eu","count":3}`))

		// --- When ---
		cfg, err := TargetConfig(rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, cfg.Has("region"))
		assert.Equal(t, "eu", must.Value(GetCfg[string](cfg, "region")))
		assert.Equal(t, 3, must.Value(GetCfg[int](cfg, "count")))
	})

	t.Run("decodes string block", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.MetaSet(ConfigMetaKey, `{"region":"eu","count":3}`)

		// --- When ---
		cfg, err := TargetConfig(rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, cfg.Has("region"))
		assert.Equal(t, "eu", must.Value(GetCfg[string](cfg, "region")))
		assert.Equal(t, 3, must.Value(GetCfg[int](cfg, "count")))
	})

	t.Run("no config", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		// --- When ---
		cfg, err := TargetConfig(rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, cfg.Has("region"))
	})

	t.Run("unsupported meta value type", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.MetaSet(ConfigMetaKey, 123)

		// --- When ---
		cfg, err := TargetConfig(rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, cfg.Has("region"))
	})

	t.Run("error - invalid json", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.MetaSet(ConfigMetaKey, "{not json")

		// --- When ---
		cfg, err := TargetConfig(rng)

		// --- Then ---
		assert.Error(t, err)
		assert.Nil(t, cfg)
	})
}

func Test_Config_Has_tabular(t *testing.T) {
	block := `{
	"region": "eu",
	"lint": {"file": ".golangci.yml"},
	"hosts": ["web-1", "web-2"],
	"github.com/acme/app": {"package": "version"},
	"modules": {
		"github.com/acme/app": {"package": "version"}
	}
}`
	cfg := configFrom(t, block)

	tt := []struct {
		name string
		path string
		want bool
	}{
		{"scalar key", "region", true},
		{"nested key", "lint.file", true},
		{"array index", "hosts.1", true},
		{"quoted key with dots", "'github.com/acme/app'.package", true},
		{"quoted then nested", "modules.'github.com/acme/app'.package", true},
		{"absent key", "nope", false},
		{"out of range", "hosts.9", false},
		{"absent quoted key", "'github.com/acme/nope'.package", false},
		{"empty path", "", false},
		{"descend scalar", "region.foo", false},
		{"unterminated quote", "'github.com/acme", false},
		{"quote not at boundary", "'github.com'x.package", false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// --- When ---
			have := cfg.Has(tc.path)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_splitPath_tabular(t *testing.T) {
	tt := []struct {
		name string
		path string
		want []string
	}{
		{"single segment", "region", []string{"region"}},
		{"dotted segments", "lint.file", []string{"lint", "file"}},
		{"array index", "hosts.1", []string{"hosts", "1"}},
		{
			"quoted key with dots",
			"'github.com/acme/app'",
			[]string{"github.com/acme/app"},
		},
		{
			"quoted then nested",
			"modules.'github.com/acme/app'.package",
			[]string{"modules", "github.com/acme/app", "package"},
		},
		{
			"quoted at end",
			"modules.'github.com/acme/app'",
			[]string{"modules", "github.com/acme/app"},
		},
		{"quoted array index", "'a.b'.0", []string{"a.b", "0"}},
		{"empty quotes", "a.''.b", []string{"a", "", "b"}},
		{"quote mid segment literal", "a.b'c", []string{"a", "b'c"}},
		{"trailing dot", "a.", []string{"a", ""}},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// --- When ---
			have, err := splitPath(tc.path)

			// --- Then ---
			assert.NoError(t, err)
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_splitPath_error_tabular(t *testing.T) {
	tt := []struct {
		name string
		path string
	}{
		{"error - empty path", ""},
		{"error - unterminated quote", "'github.com/acme"},
		{"error - quote not at boundary", "'github.com'x.package"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// --- When ---
			_, err := splitPath(tc.path)

			// --- Then ---
			assert.ErrorIs(t, ErrMiss, err)
		})
	}
}

func Test_GetCfg(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"region":"eu"}`)

		// --- When ---
		have, err := GetCfg[string](cfg, "region")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "eu", have)
	})

	t.Run("int from whole number", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"count":3}`)

		// --- When ---
		have, err := GetCfg[int](cfg, "count")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 3, have)
	})

	t.Run("bool", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"dry_run":true}`)

		// --- When ---
		have, err := GetCfg[bool](cfg, "dry_run")

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have)
	})

	t.Run("float", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"ratio":0.5}`)

		// --- When ---
		have, err := GetCfg[float64](cfg, "ratio")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 0.5, have)
	})

	t.Run("duration", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"timeout":"5m"}`)

		// --- When ---
		have, err := GetCfg[time.Duration](cfg, "timeout")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 5*time.Minute, have)
	})

	t.Run("string slice", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"hosts":["web-1","web-2"]}`)

		// --- When ---
		have, err := GetCfg[[]string](cfg, "hosts")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"web-1", "web-2"}, have)
	})

	t.Run("struct", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"lint":{"file":".golangci.yml","fix":true}}`)
		type lint struct {
			File string `json:"file"`
			Fix  bool   `json:"fix"`
		}

		// --- When ---
		have, err := GetCfg[lint](cfg, "lint")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, lint{File: ".golangci.yml", Fix: true}, have)
	})

	t.Run("any returns raw", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"count":3}`)

		// --- When ---
		have, err := GetCfg[any](cfg, "count")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, float64(3), have)
	})

	t.Run("any returns copy of map", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"lint":{"version":"v1"}}`)

		// --- When ---
		have, err := GetCfg[any](cfg, "lint")
		assert.NoError(t, err)
		m, ok := have.(map[string]any)
		assert.True(t, ok)
		m["version"] = "mutated"

		// --- Then ---
		assert.Equal(t, "v1", must.Value(GetCfg[string](cfg, "lint.version")))
	})

	t.Run("nested path", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"lint":{"version":"v2.13.0"}}`)

		// --- When ---
		have, err := GetCfg[string](cfg, "lint.version")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v2.13.0", have)
	})

	t.Run("array index", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"hosts":["web-1","web-2"]}`)

		// --- When ---
		have, err := GetCfg[string](cfg, "hosts.0")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "web-1", have)
	})

	t.Run("quoted key with dots", func(t *testing.T) {
		// --- Given ---
		block := `{
	"modules": {
		"github.com/acme/app": {"names": {"buildDate": "BuildDate"}}
	}
}`
		cfg := configFrom(t, block)
		path := "modules.'github.com/acme/app'.names.buildDate"

		// --- When ---
		have, err := GetCfg[string](cfg, path)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "BuildDate", have)
	})

	t.Run("error - unterminated quote", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"region":"eu"}`)

		// --- When ---
		_, err := GetCfg[string](cfg, "'github.com/acme")

		// --- Then ---
		assert.ErrorIs(t, ErrMiss, err)
	})

	t.Run("error - miss", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"region":"eu"}`)

		// --- When ---
		_, err := GetCfg[string](cfg, "nope")

		// --- Then ---
		assert.ErrorIs(t, ErrMiss, err)
	})

	t.Run("error - int from fractional", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"ratio":3.5}`)

		// --- When ---
		_, err := GetCfg[int](cfg, "ratio")

		// --- Then ---
		assert.ErrorIs(t, ErrType, err)
	})

	t.Run("error - string from number", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"count":3}`)

		// --- When ---
		_, err := GetCfg[string](cfg, "count")

		// --- Then ---
		assert.ErrorIs(t, ErrType, err)
	})

	t.Run("error - duration parse", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"timeout":"nope"}`)

		// --- When ---
		_, err := GetCfg[time.Duration](cfg, "timeout")

		// --- Then ---
		assert.ErrorIs(t, ErrType, err)
	})

	t.Run("duration from number", func(t *testing.T) {
		// --- Given ---
		// A JSON number is the nanosecond count, the form encoding/json marshals
		// a time.Duration to.
		cfg := configFrom(t, `{"timeout":300000000000}`)

		// --- When ---
		have, err := GetCfg[time.Duration](cfg, "timeout")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 5*time.Minute, have)
	})

	t.Run("error - duration from fractional number", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"timeout":3.5}`)

		// --- When ---
		_, err := GetCfg[time.Duration](cfg, "timeout")

		// --- Then ---
		assert.ErrorIs(t, ErrType, err)
	})

	t.Run("error - descend scalar", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"region":"eu"}`)

		// --- When ---
		_, err := GetCfg[string](cfg, "region.foo")

		// --- Then ---
		assert.ErrorIs(t, ErrType, err)
	})
}

func Test_GetCfgDefault(t *testing.T) {
	t.Run("returns the delivered value", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"timeout":"5m"}`)

		// --- When ---
		have, err := GetCfgDefault(cfg, "timeout", "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "5m", have)
	})

	t.Run("absent path returns default", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"region":"eu"}`)

		// --- When ---
		have, err := GetCfgDefault(cfg, "timeout", "1m")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "1m", have)
	})

	t.Run("empty block returns default", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{}`)

		// --- When ---
		have, err := GetCfgDefault(cfg, "timeout", "1m")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "1m", have)
	})

	t.Run("error - type mismatch", func(t *testing.T) {
		// --- Given ---
		cfg := configFrom(t, `{"count":3}`)

		// --- When ---
		have, err := GetCfgDefault(cfg, "count", "def")

		// --- Then ---
		assert.ErrorIs(t, ErrType, err)
		assert.Equal(t, "", have)
	})
}
