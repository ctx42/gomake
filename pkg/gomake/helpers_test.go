// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"testing"

	"github.com/ctx42/testing/pkg/assert"
)

func Test_runeSize_tabular(t *testing.T) {
	tt := []struct {
		testN string

		lead byte
		want int
	}{
		{"ascii", 'a', 1},
		{"two bytes", 0xC3, 2},
		{"three bytes", 0xE2, 3},
		{"four bytes", 0xF0, 4},
		{"continuation byte", 0x80, 1},
		{"invalid lead", 0xFF, 1},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := runeSize(tc.lead)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
