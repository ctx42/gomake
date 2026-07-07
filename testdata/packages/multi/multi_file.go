// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package multi

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

func Hello(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "package multi saying hello")
	return nil
}
