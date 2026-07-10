// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
)

func ExampleTargetName() {
	_, ok := gomake.TargetName(context.Background())
	fmt.Println(ok)
	// Output: false
}

func ExampleRoot() {
	// Root walks up from the given path to the directory holding "go.mod".
	root, err := gomake.Root(".")
	fmt.Println(err == nil, root != "")
	// Output: true true
}

func ExampleTargetConfig() {
	rng := ring.New()
	rng.MetaSet(gomake.ConfigMetaKey, `{"region":"eu"}`)

	cfg, _ := gomake.TargetConfig(rng)
	region, _ := gomake.GetCfg[string](cfg, "region")
	fmt.Println(region)
	// Output: eu
}

func ExampleGetCfg() {
	rng := ring.New()
	rng.MetaSet(gomake.ConfigMetaKey, `{"lint":{"version":"v2.13.0"}}`)

	cfg, _ := gomake.TargetConfig(rng)
	version, _ := gomake.GetCfg[string](cfg, "lint.version")
	fmt.Println(version)
	// Output: v2.13.0
}

func ExampleExitStatus() {
	fmt.Println(gomake.ExitStatus(nil))
	fmt.Println(gomake.ExitStatus(errors.New("boom")))
	// Output:
	// 0
	// 1
}

func ExampleHasRun() {
	fmt.Println(gomake.HasRun(nil))
	// Output: true
}
