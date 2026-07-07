// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"errors"
	"os"
	"testing"

	"github.com/ctx42/testkit/pkg/selfkit"
)

func TestMain(m *testing.M) {
	runTests, exitCode := selfkit.New().Run(os.Stdout, os.Stderr)
	if runTests {
		ensureTestdataSymlinks()
		os.Exit(m.Run())
	}
	os.Exit(exitCode)
}

// /////////////////////////////////////////////////////////////////////////////

// ErrTest is a general error used in tests.
var ErrTest = errors.New("test error")

// ensureTestdataSymlinks creates git-tracked symlinks when missing from a
// checkout (e.g. after clone without symlink support).
func ensureTestdataSymlinks() {
	const (
		link   = "testdata/file0_link.txt"
		target = "file0.txt"
	)
	fi, err := os.Lstat(link)
	if err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return
	}
	_ = os.Remove(link)
	_ = os.Symlink(target, link)
}

// TError is a test structure implementing error and exitStatus interfaces.
type TError struct {
	Err      string
	ExStatus int
}

func (e TError) Error() string   { return e.Err }
func (e TError) ExitStatus() int { return e.ExStatus }
