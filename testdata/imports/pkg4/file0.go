// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package pkg4 is used to test "gomake:import".
package pkg4

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

type NS struct{} //gomake:ns_root

// M0 is a method in namespace NS. The rest of the doc string.
func (NS) M0(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "pkg4.NS.M0")
	return nil
}

// M1 is a method in namespace NS. The rest of the doc string.
func (NS) M1(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "pkg4.NS.M1")
	return nil
}
