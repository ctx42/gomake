// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package osarch

import (
	"bufio"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/oskit"
)

// Test_IsGOOS guards that every GOOS the active toolchain reports is in the
// generated list. A missing value means the list came from an older Go.
// Regenerate with `go generate ./internal/osarch`.
func Test_IsGOOS(t *testing.T) {
	// --- Given ---
	osSet, _ := distList(t)

	// --- Then ---
	for _, v := range osSet {
		if !IsGOOS(v) {
			t.Errorf("GOOS %q reported by the toolchain is missing from "+
				"versions_gen.go; run: go generate ./internal/osarch", v)
		}
	}
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
	_, archSet := distList(t)

	// --- Then ---
	for _, v := range archSet {
		if !IsGOARCH(v) {
			t.Errorf("GOARCH %q reported by the toolchain is missing from "+
				"versions_gen.go; run: go generate ./internal/osarch", v)
		}
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

func Test_generateVersions(t *testing.T) {
	t.Run("error - invalid GOMAKE_GO_VERSION", func(t *testing.T) {
		// --- Given ---
		t.Setenv("GOMAKE_GO_VERSION", "nope")

		cmd := exec.CommandContext(t.Context(), "go", "run",
			"00_generate_versions.go")

		// --- When ---
		out, err := cmd.CombinedOutput()

		// --- Then ---
		assert.Error(t, err)
		assert.Contain(t, "invalid GOMAKE_GO_VERSION", string(out))
	})

	t.Run("error - version endpoint", func(t *testing.T) {
		// --- Given ---
		srv := httptest.NewServer(http.NotFoundHandler())
		t.Cleanup(srv.Close)

		before := oskit.ReadFileStr(t, "versions_gen.go")

		t.Setenv("GOMAKE_VERSION_URL", srv.URL)
		t.Setenv("GOMAKE_GO_VERSION", "")
		ctx := t.Context()
		cmd := exec.CommandContext(ctx, "go", "run", "00_generate_versions.go")

		// --- When ---
		out, err := cmd.CombinedOutput()

		// --- Then ---
		assert.Error(t, err)
		assert.Contain(t, "HTTP 404", string(out))

		after := oskit.ReadFileStr(t, "versions_gen.go")
		assert.Equal(t, before, after)
	})
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
