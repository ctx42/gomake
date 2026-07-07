// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

func Hello(ctx context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprintf(rng.Stdout(), "hello %v", rng.Args())
	return nil
}

func Bye(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprintf(rng.Stdout(), "bye %v", rng.Args())
	return nil
}
