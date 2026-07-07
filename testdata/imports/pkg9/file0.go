// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package pkg9 is used to test "gomake:import".
package pkg9

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

// NS0 is a namespace root.
type NS0 struct{} //gomake:ns_root

func (NS0) Hello(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "NS0.Hello")
	return nil
}

// NS1 represents nested namespace.
type NS1 NS0

func (NS1) Hello(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "NS1.Hello")
	return nil
}

// NS2 represents doubly nested namespace.
type NS2 NS1

func (NS2) HelloHello(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "NS2.Hello")
	return nil
}
