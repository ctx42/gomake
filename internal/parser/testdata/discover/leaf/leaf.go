// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package leaf is a single-package import for discovery tests.
package leaf

import (
	"context"

	"github.com/ctx42/ring/pkg/ring"
)

// Leaf is a leaf-package target.
func Leaf(_ context.Context, _ *ring.Ring) error {
	return nil
}
