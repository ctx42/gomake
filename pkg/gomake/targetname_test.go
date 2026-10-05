// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"context"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
)

func Test_TargetName_tabular(t *testing.T) {
	// A string key is required so the value matches the inlined runtime.
	badType := context.WithValue(
		t.Context(),
		targetNameKey, //nolint:staticcheck
		42,
	)

	tt := []struct {
		testN string
		ctx   context.Context
		wName string
		wOk   bool
	}{
		{
			"unset",
			t.Context(),
			"",
			false,
		},
		{
			"set",
			WithTargetName(t.Context(), "tgt-print-name"),
			"tgt-print-name",
			true,
		},
		{
			"wrong type",
			badType,
			"",
			false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			hName, hOk := TargetName(tc.ctx)

			// --- Then ---
			assert.Equal(t, tc.wName, hName)
			assert.Equal(t, tc.wOk, hOk)
		})
	}
}
