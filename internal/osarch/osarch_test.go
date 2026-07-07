// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package osarch

import (
	"bufio"
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testing/pkg/tester"
)

func Test_IsGOOS_tabular(t *testing.T) {
	tt := []struct {
		testN string

		in   string
		want bool
	}{
		{"linux", "linux", true},
		{"windows", "windows", true},
		{"darwin", "darwin", true},
		{"unknown", "nope", false},
		{"empty", "", false},
		{"arch not os", "amd64", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := IsGOOS(tc.in)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_IsGOARCH_tabular(t *testing.T) {
	tt := []struct {
		testN string

		in   string
		want bool
	}{
		{"amd64", "amd64", true},
		{"arm64", "arm64", true},
		{"386", "386", true},
		{"unknown", "nope", false},
		{"empty", "", false},
		{"os not arch", "linux", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := IsGOARCH(tc.in)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

// Test_list_isToolchainSuperset is a guard: every GOOS/GOARCH supported by the
// active toolchain must be present in the generated lists. GOOS/GOARCH values
// are only added in newer Go releases, so a missing value means the list was
// generated from an older Go than the one running the tests — regenerate with
// `go generate ./internal/osarch`.
func Test_list_isToolchainSuperset(t *testing.T) {
	// --- Given ---
	osSet, archSet := distList(t)

	// --- Then ---
	for _, v := range osSet {
		if !IsGOOS(v) {
			t.Errorf("GOOS %q reported by the toolchain is missing from "+
				"versions_gen.go; run: go generate ./internal/osarch", v)
		}
	}
	for _, v := range archSet {
		if !IsGOARCH(v) {
			t.Errorf("GOARCH %q reported by the toolchain is missing from "+
				"versions_gen.go; run: go generate ./internal/osarch", v)
		}
	}
}

// distList returns the unique GOOS and GOARCH values from `go tool dist list`.
func distList(t tester.T) (goos, goarch []string) {
	t.Helper()
	out := must.Value(exec.Command("go", "tool", "dist", "list").Output())

	osSet := map[string]struct{}{}
	archSet := map[string]struct{}{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		pair := strings.SplitN(strings.TrimSpace(sc.Text()), "/", 2)
		if len(pair) != 2 {
			continue
		}
		osSet[pair[0]] = struct{}{}
		archSet[pair[1]] = struct{}{}
	}
	assert.NoError(t, sc.Err())
	return keys(osSet), keys(archSet)
}

func keys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
