// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package versiontest provides test helpers for the [version] package.
package versiontest

import (
	"github.com/ctx42/testing/pkg/tester"

	"github.com/ctx42/gomake/internal/version"
)

// SaveVersion saves [version] package vars and restores them after the test.
func SaveVersion(t tester.T) {
	t.Helper()
	date, rev, hash, state, cc := version.Get()
	fn := func() { version.Set(date, rev, hash, state, cc) }
	t.Cleanup(fn)
}
