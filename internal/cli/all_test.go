// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"errors"
	"os"
	"testing"
	"time"
)

// ErrTest is a general error used in tests.
var ErrTest = errors.New("test error")

// TestMain disables timing-based progress output for the whole package so
// tests that capture stderr are deterministic regardless of how long a
// compilation takes (a cold build cache would otherwise emit "Compiling
// makefile..."). Test_withProgress re-enables a short threshold locally.
func TestMain(m *testing.M) {
	progressThreshold = time.Hour
	os.Exit(m.Run())
}
