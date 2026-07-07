// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package version

import (
	"github.com/ctx42/testing/pkg/tester"
)

// saveVars restores the package build-metadata variables after the test.
func saveVars(t tester.T) {
	t.Helper()
	d, r, h, s, c := buildDate, scmRev, scmHash, scmState, ccid
	t.Cleanup(func() {
		buildDate, scmRev, scmHash, scmState, ccid = d, r, h, s, c
	})
}
