// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package version

import (
	"runtime/debug"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/xdef/pkg/xdef"
)

func Test_Get_Set(t *testing.T) {
	// --- Given ---
	saveVars(t)

	// --- When ---
	Set("2024-06-01T12:00:00Z", "v1.2.3", "abc1234", "clean")
	date, rev, hash, state := Get()

	// --- Then ---
	assert.Equal(t, "2024-06-01T12:00:00Z", date)
	assert.Equal(t, "v1.2.3", rev)
	assert.Equal(t, "abc1234", hash)
	assert.Equal(t, "clean", state)
}

func Test_PopulateVersion(t *testing.T) {
	t.Run("nil info", func(t *testing.T) {
		// --- Given ---
		saveVars(t)
		fixed := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)
		rng := ring.New(ring.WithClock(func() time.Time { return fixed }))

		// --- When ---
		PopulateVersion(rng, nil)

		// --- Then ---
		date, rev, hash, state := Get()
		assert.Equal(t, "2000-01-02T03:04:05Z", date)
		assert.Equal(t, xdef.PhNotSet, rev)
		assert.Equal(t, xdef.PhNotSet, hash)
		assert.Equal(t, xdef.PhNotSet, state)
	})

	t.Run("published build no vcs", func(t *testing.T) {
		// --- Given ---
		saveVars(t)
		rng := ringtest.New(t)
		info := &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}

		// --- When ---
		PopulateVersion(rng.Ring(), info)

		// --- Then ---
		_, rev, hash, state := Get()
		assert.Equal(t, "v1.2.3", rev)
		assert.Equal(t, xdef.PhNotSet, hash)
		assert.Equal(t, xdef.PhNotSet, state)
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
		_, rev, hash, state := Get()
		assert.Equal(t, "v2.0.0", rev)
		assert.Equal(t, "deadbee", hash)
		assert.Equal(t, "dirty", state)
	})
}

func Test_orNotSet_tabular(t *testing.T) {
	tt := []struct {
		testN string

		s    string
		want string
	}{
		{"empty returns PhNotSet", "", xdef.PhNotSet},
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

		cmd                    string
		rev, hash, date, state string
		want                   string
	}{
		{
			"all set",
			"cmd",
			"v1.2", "12ab34", "2000-01-02T03:04:05Z", "clean",
			"cmd v1.2, hash: 12ab34, build date: 2000-01-02T03:04:05Z, " +
				"scm state: clean",
		},
		{
			"only rev",
			"my-cmd",
			"v1.2", xdef.PhNotSet, xdef.PhNotSet, xdef.PhNotSet,
			"my-cmd v1.2, hash: <not set>, build date: <not set>, " +
				"scm state: <not set>",
		},
		{
			"unset",
			"my-cmd",
			xdef.PhNotSet, xdef.PhNotSet, xdef.PhNotSet, xdef.PhNotSet,
			"my-cmd <not set>, hash: <not set>, build date: <not set>, " +
				"scm state: <not set>",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			saveVars(t)

			scmRev = tc.rev
			scmHash = tc.hash
			bldDate = tc.date
			scmState = tc.state

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

		bldDate = "2000-01-02T03:04:05Z"
		scmRev = "v1.2"
		scmHash = "12ab34"
		scmState = "clean"

		// --- When ---
		have := LDFlags()

		// --- Then ---
		want := "" +
			"-X github.com/ctx42/gomake/internal/version.bldDate=" +
			"2000-01-02T03:04:05Z " +
			"-X github.com/ctx42/gomake/internal/version.scmRev=v1.2 " +
			"-X github.com/ctx42/gomake/internal/version.scmHash=12ab34 " +
			"-X github.com/ctx42/gomake/internal/version.scmState=clean"
		assert.Equal(t, want, have)
	})

	t.Run("names come from xdef", func(t *testing.T) {
		// The field names are sourced from xdef so they never drift from
		// the names gomake injects into other projects. Guard that
		// linkage.
		// --- Given ---
		saveVars(t)

		// --- When ---
		have := LDFlags()

		// --- Then ---
		assert.Contain(t, "."+xdef.VarBldDate+"=", have)
		assert.Contain(t, "."+xdef.VarScmRev+"=", have)
		assert.Contain(t, "."+xdef.VarScmHash+"=", have)
		assert.Contain(t, "."+xdef.VarScmState+"=", have)
	})

	t.Run("every name targets a package variable", func(t *testing.T) {
		// The linker silently ignores a -X definition naming a variable
		// that does not exist, so renaming one in the var block, or
		// adding a variable without its flag, would break injection
		// without failing any other test. The package-level variables
		// declared in version.go are exactly the ones ldflags populates.
		// --- Given ---
		saveVars(t)
		Set("2000-01-02T03:04:05Z", "v1.2", "12ab34", "clean")

		want := declaredVarNames(t, "version.go")
		slices.Sort(want)

		// --- When ---
		flags := LDFlags()

		// --- Then ---
		have := make([]string, 0, len(want))
		for _, arg := range strings.Fields(flags) {
			if arg == "-X" {
				continue
			}
			def, _, _ := strings.Cut(arg, "=")
			have = append(have, strings.TrimPrefix(def, importPath+"."))
		}
		slices.Sort(have)
		assert.Equal(t, want, have)
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
			"ScmState",
			"not clean",
			`-X "github.com/ctx42/gomake/internal/version.ScmState=not clean"`,
		},
		{
			"double quote in value",
			"ScmState",
			`say "hi"`,
			`-X 'github.com/ctx42/gomake/internal/version.ScmState=say "hi"'`,
		},
		{
			// go build splits -ldflags with quoted.Split, which does no
			// backslash unescaping; a backslash must survive verbatim.
			"backslash in value",
			"ScmRepo",
			`C:\proj`,
			`-X "github.com/ctx42/gomake/internal/version.ScmRepo=C:\proj"`,
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
