// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package pkg2

import (
	"context"

	"github.com/ctx42/ring/pkg/ring"
)

func PanicString(_ context.Context, rng *ring.Ring) error {
	msg := "panic string"
	if len(rng.Args()) > 0 {
		msg = rng.Args()[0]
	}
	panic(msg)
}
