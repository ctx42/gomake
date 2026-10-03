// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/selfkit"
)

func TestMain(m *testing.M) {
	runTests, exitCode := selfkit.New().Run(os.Stdout, os.Stderr)
	if runTests {
		// Recreate the git-tracked testdata symlink when the checkout
		// has none, such as a clone without symlink support.
		err := ensureSymlink("testdata/file0_link.txt", "file0.txt")
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(m.Run())
	}
	os.Exit(exitCode)
}

// /////////////////////////////////////////////////////////////////////////////

// errTest is a general error used in tests.
var errTest = errors.New("test error")

// ensureSymlink makes link a symlink to target. An existing symlink is left
// in place. Any other file at link is removed first.
func ensureSymlink(link, target string) error {
	fi, err := os.Lstat(link)
	if err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if err = os.Remove(link); err != nil {
			return fmt.Errorf("remove %s: %w", link, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("lstat %s: %w", link, err)
	}
	if err = os.Symlink(target, link); err != nil {
		return fmt.Errorf("symlink %s: %w", link, err)
	}
	return nil
}

func Test_ensureSymlink(t *testing.T) {
	t.Run("creates the link", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		link := filepath.Join(dir, "link")
		target := "file0.txt"

		// --- When ---
		err := ensureSymlink(link, target)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, target, must.Value(os.Readlink(link)))
	})

	t.Run("keeps an existing symlink", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		link := filepath.Join(dir, "link")
		kept := "kept.txt"
		must.Nil(os.Symlink(kept, link))

		// --- When ---
		err := ensureSymlink(link, "other.txt")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, kept, must.Value(os.Readlink(link)))
	})

	t.Run("replaces a file", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		link := oskit.Create(t, "x", dir, "link")
		target := "file0.txt"

		// --- When ---
		err := ensureSymlink(link, target)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, target, must.Value(os.Readlink(link)))
	})
}

// tError is a test structure implementing error and exitStatus interfaces.
type tError struct {
	Err      string
	ExStatus int
}

func (ter tError) Error() string   { return ter.Err }
func (ter tError) ExitStatus() int { return ter.ExStatus }
