// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mkf_test

import (
	"context"
	"fmt"

	"github.com/ctx42/gomake/internal/mkf"
)

func ExampleNewTarget() {
	tgt := mkf.NewTarget()
	err := tgt.Run(context.Background(), nil)
	fmt.Println(err == nil)
	// Output: true
}
