// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/ctx42/ring/pkg/ring"
)

func BasicWin(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "BasicWin")
	return nil
}
