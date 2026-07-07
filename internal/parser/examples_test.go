// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser_test

import (
	"fmt"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/internal/parser"
)

func ExampleBuildTag() {
	fmt.Println(parser.BuildTag)
	// Output: gomake
}

func ExampleSetBuildTag() {
	rng := parser.SetBuildTag(ring.New())
	fmt.Println(parser.GetBuildTag(rng))
	// Output: gomake
}

func ExampleNewTargets() {
	tgs := parser.NewTargets()
	tgt := &mkf.Target{Name: "hello"}
	if err := tgs.Add(tgt); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(tgs.Len(), tgs.Get("hello") == tgt)
	// Output: 1 true
}

func ExampleTargetsFromList() {
	t0 := &mkf.Target{Name: "a"}
	t1 := &mkf.Target{Name: "b"}
	tgs, err := parser.TargetsFromList(t0, t1)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(tgs.Len())
	// Output: 2
}
