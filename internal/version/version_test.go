// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package version

import (
	"slices"
	"strings"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
)

func Test_Line(t *testing.T) {
	// --- Given ---
	saveVars(t)
	scmRev = "v1.2.3"

	// --- When ---
	have := Line("gomake")

	// --- Then ---
	assert.Equal(t, "gomake v1.2.3", have)
}

func Test_LDFlags(t *testing.T) {
	t.Run("stamped", func(t *testing.T) {
		// --- When ---
		have := LDFlags("v1.2.3")

		// --- Then ---
		want := "-X github.com/ctx42/gomake/internal/version.scmRev=v1.2.3"
		assert.Equal(t, want, have)
	})

	t.Run("stamp target exists", func(t *testing.T) {
		// The linker silently ignores a -X definition naming a variable that
		// does not exist, so renaming the stamped variable would break
		// injection without failing any other test. Every name it targets
		// must be a package-level variable declared in version.go.
		// --- Given ---
		declared := declaredVarNames(t, "version.go")

		// --- When ---
		have := LDFlags("v1.2.3")

		// --- Then ---
		for _, arg := range strings.Fields(have) {
			if arg == "-X" {
				continue
			}
			def, _, _ := strings.Cut(arg, "=")
			name := strings.TrimPrefix(def, importPath+".")
			assert.True(t, slices.Contains(declared, name), name)
		}
	})

	t.Run("devel", func(t *testing.T) {
		// --- When ---
		have := LDFlags(devel)

		// --- Then ---
		assert.Equal(t, "", have)
	})

	t.Run("empty", func(t *testing.T) {
		// --- When ---
		have := LDFlags("")

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_revision(t *testing.T) {
	t.Run("stamped", func(t *testing.T) {
		// --- Given ---
		saveVars(t)
		scmRev = "v1.2.3"

		// --- When ---
		have := revision()

		// --- Then ---
		assert.Equal(t, "v1.2.3", have)
	})

	t.Run("unstamped", func(t *testing.T) {
		// "go test" builds without version-control stamping, so the build
		// info of this very binary reports devel. The branch reading a
		// version the toolchain did work out cannot be reached in-process;
		// pick covers the choice, and installing gomake exercises the rest.
		// --- Given ---
		saveVars(t)
		scmRev = ""

		// --- When ---
		have := revision()

		// --- Then ---
		assert.Equal(t, devel, have)
	})
}

func Test_pick_tabular(t *testing.T) {
	tt := []struct {
		testN string

		stamped string
		built   string
		want    string
	}{
		{"stamp wins over the toolchain", "v1.2.3", "v2.0.0", "v1.2.3"},
		{"stamp wins over devel", "v1.2.3", devel, "v1.2.3"},
		{"the toolchain version", "", "v2.0.0", "v2.0.0"},
		{
			"a pseudo-version",
			"",
			"v0.1.1-0.20260921123456-abcdef123456+dirty",
			"v0.1.1-0.20260921123456-abcdef123456+dirty",
		},
		{"neither", "", "", devel},
		{"devel only", "", devel, devel},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := pick(tc.stamped, tc.built)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
