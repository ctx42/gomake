// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

// NS3 represents nested namespace which root is in a different file.
type NS3 NS0

func (NS3) Hello(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "NS3.Hello")
	return nil
}

// NS4 represents double nested namespace which root is in a different file.
type NS4 NS3

func (NS4) Hello(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "NS4.Hello")
	return nil
}

// NOT5 is invalid gomake namespace because underlying type is invalid gomake
// namespace.
type NOT5 NOT2
