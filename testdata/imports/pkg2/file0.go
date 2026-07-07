// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package pkg2 is used to test "gomake:import".
package pkg2

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

// Print is a test target with help message.
func Print(_ context.Context, rng *ring.Ring) error {
	msg := "message"
	if len(rng.Args()) > 0 {
		msg = fmt.Sprintf("%v", rng.Args())
	}
	_, _ = fmt.Fprint(rng.Stdout(), msg)
	return nil
}
