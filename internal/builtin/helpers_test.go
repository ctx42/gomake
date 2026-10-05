// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package builtin

import (
	"testing"

	"github.com/ctx42/testing/pkg/assert"
)

func Test_validGoIdent_tabular(t *testing.T) {
	tt := []struct {
		testN string
		name  string
		want  bool
	}{
		{"plain", "builtin", true},
		{"digits", "go1", true},
		{"empty", "", false},
		{"blank", "_", false},
		{"dash", "not-a-name", false},
		{"keyword", "package", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := validGoIdent(tc.name)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
