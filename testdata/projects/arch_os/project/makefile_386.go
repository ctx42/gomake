// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package main

import (
	"context"
)

func targetArch(ctx context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "arch:386")
	return nil
}
