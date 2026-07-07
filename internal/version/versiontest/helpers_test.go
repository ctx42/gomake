// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package versiontest

import (
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/tester"

	"github.com/ctx42/gomake/internal/version"
)

func Test_SaveVersion(t *testing.T) {
	// --- Given ---
	wD, wR, wH, wS, wC := version.Get()
	t.Cleanup(func() { version.Set(wD, wR, wH, wS, wC) })

	tspy := tester.New(t)
	tspy.ExpectCleanups(1)
	tspy.Close()

	// --- When ---
	SaveVersion(tspy)

	// --- Then ---
	version.Set("d0", "r0", "h0", "s0", "c0")

	hD, hR, hH, hS, hC := version.Get()
	assert.Equal(t, "d0", hD)
	assert.Equal(t, "r0", hR)
	assert.Equal(t, "h0", hH)
	assert.Equal(t, "s0", hS)
	assert.Equal(t, "c0", hC)

	tspy.Finish()

	hD, hR, hH, hS, hC = version.Get()
	assert.Equal(t, wD, hD)
	assert.Equal(t, wR, hR)
	assert.Equal(t, wH, hH)
	assert.Equal(t, wS, hS)
	assert.Equal(t, wC, hC)
}
