// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package version

import (
	"slices"
	"strings"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/xdef/pkg/xdef"
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
	t.Run("stamps the version", func(t *testing.T) {
		// --- When ---
		have := LDFlags("v1.2.3")

		// --- Then ---
		want := "-X github.com/ctx42/gomake/internal/version.scmRev=v1.2.3"
		assert.Equal(t, want, have)
	})

	t.Run("the name comes from xdef", func(t *testing.T) {
		// The injected name is sourced from xdef so that it never drifts
		// from the name the ctx42 build tooling injects. Guard that linkage.
		// --- When ---
		have := LDFlags("v1.2.3")

		// --- Then ---
		assert.Contain(t, "."+xdef.VarScmRev+"=", have)
	})

	t.Run("the name targets a package variable", func(t *testing.T) {
		// The linker silently ignores a -X definition naming a variable that
		// does not exist, so renaming the stamped variable would break
		// injection without failing any other test. The package-level
		// variables declared in version.go are exactly the ones it targets.
		// --- Given ---
		want := declaredVarNames(t, "version.go")
		slices.Sort(want)

		// --- When ---
		flags := LDFlags("v1.2.3")

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

	t.Run("a version the toolchain knows is not stamped", func(t *testing.T) {
		// --- When ---
		have := LDFlags(devel)

		// --- Then ---
		assert.Equal(t, "", have)
	})

	t.Run("an unknown version is not stamped", func(t *testing.T) {
		// --- When ---
		have := LDFlags("")

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_revision(t *testing.T) {
	t.Run("a stamped version wins", func(t *testing.T) {
		// --- Given ---
		saveVars(t)
		scmRev = "v1.2.3"

		// --- When ---
		have := revision()

		// --- Then ---
		assert.Equal(t, "v1.2.3", have)
	})

	t.Run("an unstamped test binary has no version", func(t *testing.T) {
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
		{"a pseudo-version", "", "v0.1.1-0.20260921-abc+dirty",
			"v0.1.1-0.20260921-abc+dirty"},
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
