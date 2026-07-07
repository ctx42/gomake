// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package sub is a child package under mixed.
package sub

import (
	"context"

	"github.com/ctx42/ring/pkg/ring"
)

// Sub is a child leaf target.
func Sub(_ context.Context, _ *ring.Ring) error {
	return nil
}
