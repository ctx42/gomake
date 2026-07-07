// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package badchild has a child directory without child/child.go.
package badchild

import (
	"context"

	"github.com/ctx42/ring/pkg/ring"
)

// BadRoot is the root target.
func BadRoot(_ context.Context, _ *ring.Ring) error {
	return nil
}
