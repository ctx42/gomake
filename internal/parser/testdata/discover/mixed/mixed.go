// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package mixed is a directory import with a root file and subpackages.
package mixed

import (
	"context"

	"github.com/ctx42/ring/pkg/ring"
)

// MixedRoot is defined in mixed/mixed.go only.
func MixedRoot(_ context.Context, _ *ring.Ring) error {
	return nil
}
