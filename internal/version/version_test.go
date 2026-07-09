// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package version

import (
	"runtime/debug"
	"strings"
	"testing"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gomake/pkg/gomake"
)

func Test_Get_Set(t *testing.T) {
	// --- Given ---
	saveVars(t)

	// --- When ---
	Set("2024-06-01T12:00:00Z", "v1.2.3", "abc1234", "clean", "ccid")
	date, rev, hash, state, cc := Get()

	// --- Then ---
	assert.Equal(t, "2024-06-01T12:00:00Z", date)
	assert.Equal(t, "v1.2.3", rev)
	assert.Equal(t, "abc1234", hash)
	assert.Equal(t, "clean", state)
	assert.Equal(t, "ccid", cc)
}

func Test_PopulateVersion(t *testing.T) {
	t.Run("nil info", func(t *testing.T) {
		// --- Given ---
		saveVars(t)
		rng := ringtest.New(t)

		// --- When ---
		PopulateVersion(rng.Ring(), nil)

		// --- Then ---
		_, rev, hash, state, _ := Get()
		assert.Equal(t, xdef.NotSet, rev)
		assert.Equal(t, xdef.NotSet, hash)
		assert.Equal(t, xdef.NotSet, state)
	})

	t.Run("published build no vcs", func(t *testing.T) {
		// --- Given ---
		saveVars(t)
		rng := ringtest.New(t)
		info := &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}

		// --- When ---
		PopulateVersion(rng.Ring(), info)

		// --- Then ---
		_, rev, hash, state, _ := Get()
		assert.Equal(t, "v1.2.3", rev)
		assert.Equal(t, xdef.NotSet, hash)
		assert.Equal(t, xdef.NotSet, state)
	})

	t.Run("build info with vcs", func(t *testing.T) {
		// --- Given ---
		saveVars(t)
		rng := ringtest.New(t)
		info := &debug.BuildInfo{
			Main: debug.Module{Version: "v2.0.0"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "deadbeefcafe"},
				{Key: "vcs.modified", Value: "true"},
			},
		}

		// --- When ---
		PopulateVersion(rng.Ring(), info)

		// --- Then ---
		_, rev, hash, state, _ := Get()
		assert.Equal(t, "v2.0.0", rev)
		assert.Equal(t, "deadbee", hash)
		assert.Equal(t, "dirty", state)
	})

	t.Run("unset ccid env", func(t *testing.T) {
		// --- Given ---
		saveVars(t)
		rng := ringtest.New(t)

		// --- When ---
		PopulateVersion(rng.Ring(), nil)

		// --- Then ---
		_, _, _, _, ccid := Get()
		assert.Equal(t, xdef.NotSet, ccid)
	})

	t.Run("ccid from env", func(t *testing.T) {
		// --- Given ---
		saveVars(t)
		rng := ringtest.New(t)
		r := rng.Ring()
		r.EnvSet(gomake.CCIDEnvKey, "job-42")

		// --- When ---
		PopulateVersion(r, nil)

		// --- Then ---
		_, _, _, _, ccid := Get()
		assert.Equal(t, "job-42", ccid)
	})
}

func Test_orNotSet_tabular(t *testing.T) {
	tt := []struct {
		testN string

		s    string
		want string
	}{
		{"empty returns NotSet", "", xdef.NotSet},
		{"non-empty returned as is", "v1.2", "v1.2"},
		{"whitespace is non-empty", " ", " "},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := orNotSet(tc.s)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_Version_tabular(t *testing.T) {
	tt := []struct {
		testN string

		cmd                          string
		rev, hash, date, state, ccid string
		want                         string
	}{
		{
			"all set",
			"cmd",
			"v1.2", "12ab34", "2000-01-02T03:04:05Z", "clean", "ccid",
			"cmd v1.2, hash: 12ab34, build date: 2000-01-02T03:04:05Z, " +
				"scm state: clean, cc tag: ccid",
		},
		{
			"only rev",
			"my-cmd",
			"v1.2", xdef.NotSet, xdef.NotSet, xdef.NotSet, xdef.NotSet,
			"my-cmd v1.2, hash: <not set>, build date: <not set>, " +
				"scm state: <not set>, cc tag: <not set>",
		},
		{
			"unset",
			"my-cmd",
			xdef.NotSet, xdef.NotSet, xdef.NotSet, xdef.NotSet, xdef.NotSet,
			"my-cmd <not set>, hash: <not set>, build date: <not set>, " +
				"scm state: <not set>, cc tag: <not set>",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			saveVars(t)

			scmRev = tc.rev
			scmHash = tc.hash
			buildDate = tc.date
			scmState = tc.state
			ccid = tc.ccid

			// --- When ---
			have := Version(tc.cmd)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_LDFlags(t *testing.T) {
	t.Run("formats all fields", func(t *testing.T) {
		// --- Given ---
		saveVars(t)

		buildDate = "2000-01-02T03:04:05Z"
		scmRev = "v1.2"
		scmHash = "12ab34"
		scmState = "clean"
		ccid = "my-cc-tag"

		// --- When ---
		have := LDFlags()

		// --- Then ---
		want := "" +
			"-X github.com/ctx42/gomake/internal/version.buildDate=" +
			"2000-01-02T03:04:05Z " +
			"-X github.com/ctx42/gomake/internal/version.scmRev=v1.2 " +
			"-X github.com/ctx42/gomake/internal/version.scmHash=12ab34 " +
			"-X github.com/ctx42/gomake/internal/version.scmState=clean " +
			"-X github.com/ctx42/gomake/internal/version.ccid=my-cc-tag"
		assert.Equal(t, want, have)
	})

	t.Run("names come from xdef", func(t *testing.T) {
		// The injected field names are sourced from xdef so they never drift
		// from the names gmgo injects into other projects. Guard that linkage.
		// --- Given ---
		saveVars(t)

		// --- When ---
		have := LDFlags()

		// --- Then ---
		assert.Contain(t, "."+xdef.VarBuildDate+"=", have)
		assert.Contain(t, "."+xdef.VarScmRev+"=", have)
		assert.Contain(t, "."+xdef.VarScmHash+"=", have)
		assert.Contain(t, "."+xdef.VarScmState+"=", have)
		assert.Contain(t, "."+xdef.VarCcid+"=", have)
	})
}

func Test_buildInfoFields_tabular(t *testing.T) {
	tt := []struct {
		testN string

		info   *debug.BuildInfo
		wRev   string
		wHash  string
		wState string
	}{
		{
			"nil info",
			nil,
			"",
			"",
			"",
		},
		{
			"devel version is treated as absent",
			&debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
			},
			"",
			"",
			"",
		},
		{
			"full metadata",
			&debug.BuildInfo{
				Main: debug.Module{Version: "v1.2.3"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abc123def456"},
					{Key: "vcs.modified", Value: "false"},
				},
			},
			"v1.2.3",
			"abc123d",
			"clean",
		},
		{
			"dirty working directory",
			&debug.BuildInfo{
				Main: debug.Module{Version: "v2.0.0"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "deadbeef"},
					{Key: "vcs.modified", Value: "true"},
				},
			},
			"v2.0.0",
			"deadbee",
			"dirty",
		},
		{
			"revision shorter than 7 chars",
			&debug.BuildInfo{
				Main: debug.Module{Version: "v0.1.0"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abc"},
				},
			},
			"v0.1.0",
			"abc",
			"",
		},
		{
			"no vcs settings",
			&debug.BuildInfo{
				Main: debug.Module{Version: "v3.0.0"},
			},
			"v3.0.0",
			"",
			"",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			hRev, hHash, hState := buildInfoFields(tc.info)

			// --- Then ---
			assert.Equal(t, tc.wRev, hRev)
			assert.Equal(t, tc.wHash, hHash)
			assert.Equal(t, tc.wState, hState)
		})
	}
}

func Test_writeField_tabular(t *testing.T) {
	tt := []struct {
		testN string

		init  string
		label string
		val   string
		want  string
	}{
		{"empty builder", "", "key", "val", ", key: val"},
		{"non-empty builder", "prefix", "key", "val", "prefix, key: val"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			var b strings.Builder
			b.WriteString(tc.init)

			// --- When ---
			writeField(&b, tc.label, tc.val)

			// --- Then ---
			assert.Equal(t, tc.want, b.String())
		})
	}
}

func Test_ldflag_tabular(t *testing.T) {
	tt := []struct {
		testN string

		field string
		value string
		want  string
	}{
		{
			"plain",
			"ScmRev",
			"v1.2",
			"-X github.com/ctx42/gomake/internal/version.ScmRev=v1.2",
		},
		{
			"apostrophe in value",
			"ScmRev",
			"it's",
			`-X "github.com/ctx42/gomake/internal/version.ScmRev=it's"`,
		},
		{
			"space in value",
			"CCID",
			"job 123",
			`-X "github.com/ctx42/gomake/internal/version.CCID=job 123"`,
		},
		{
			"double quote in value",
			"CCID",
			`say "hi"`,
			`-X 'github.com/ctx42/gomake/internal/version.CCID=say "hi"'`,
		},
		{
			// go build splits -ldflags with quoted.Split, which does no
			// backslash unescaping; a backslash must survive verbatim.
			"backslash in value",
			"CCID",
			`C:\proj`,
			`-X "github.com/ctx42/gomake/internal/version.CCID=C:\proj"`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := ldflag(tc.field, tc.value)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_needsLDQuote_tabular(t *testing.T) {
	tt := []struct {
		testN string

		def  string
		want bool
	}{
		{"plain", "path.Field=value", false},
		{"space", "path.Field=val ue", true},
		{"tab", "path.Field=val\tue", true},
		{"double quote", `path.Field=say "hi"`, true},
		{"single quote", "path.Field=it's", true},
		{"backslash", `path.Field=a\b`, true},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := needsLDQuote(tc.def)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
