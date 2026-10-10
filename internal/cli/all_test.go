// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/modkit"
)

// ErrTest is a general error used in tests.
var ErrTest = errors.New("test error")

// TestMain disables timing-based progress output for the whole package so
// tests that capture stderr are deterministic regardless of how long a
// compilation takes (a cold build cache would otherwise emit "Compiling
// makefile..."). Test_withProgress re-enables a short threshold locally.
//
// It also sets the xflag version from the module's go.mod, because test
// binaries do not record dependency versions in their build information.
func TestMain(m *testing.M) {
	progressThreshold = time.Hour
	modPth := filepath.Join(modkit.Root(), "go.mod")
	xflagVer = must.Value(modkit.ModVer(modPth, xflagModPath))
	os.Exit(m.Run())
}
