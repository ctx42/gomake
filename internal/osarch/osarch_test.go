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
	"github.com/ctx42/testing/pkg/tester"
)

// Test_IsGOOS guards that every GOOS the active toolchain reports is in the
// generated list. A missing value means the list came from an older Go.
// Regenerate with `go generate ./internal/osarch`.
func Test_IsGOOS(t *testing.T) {
	// --- Given ---
	osList, _ := distList(t)

	// --- When ---
	var have []string
	for _, v := range osList {
		if !IsGOOS(v) {
			have = append(have, v)
		}
	}

	// --- Then ---
	assert.Nil(t, have, ""+
		"GOOS values reported by the toolchain are missing from "+
		"versions_gen.go; run: go generate ./internal/osarch",
	)
}

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

// Test_IsGOARCH guards that every GOARCH the active toolchain reports is
// in the generated list. A missing value means the list came from an older
// Go. Regenerate with `go generate ./internal/osarch`.
func Test_IsGOARCH(t *testing.T) {
	// --- Given ---
	_, archList := distList(t)

	// --- When ---
	var have []string
	for _, v := range archList {
		if !IsGOARCH(v) {
			have = append(have, v)
		}
	}

	// --- Then ---
	assert.Nil(t, have, ""+
		"GOARCH values reported by the toolchain are missing from "+
		"versions_gen.go; run: go generate ./internal/osarch",
	)
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

// distList returns the unique GOOS and GOARCH values from `go tool dist list`.
func distList(t tester.T) (osList, archList []string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "go", "tool", "dist", "list")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
		return nil, nil
	}

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
