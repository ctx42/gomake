// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package main is a parser-only fixture for cross-file namespace breadcrumbs.
// Namespace roots (NS0, NOT2) live here; namespaces nested from them in another
// file (NS3, NS4, NOT5) live in makefile_nested.go, exercising the partial
// breadcrumb path. It is intentionally not gomake-discoverable as a real
// project.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/pkg/gomake"
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

func (NS2) SayMyName(ctx context.Context, rng *ring.Ring) error {
	name, _ := gomake.TargetName(ctx)
	_, _ = fmt.Fprintf(rng.Stdout(), "my name is %s\n", name)
	return nil
}

// KebabCase represents a namespace with using lower and upper case letters.
type KebabCase struct{} //gomake:ns_root

func (KebabCase) HelloWorld(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "KebabCase.HelloWorld")
	return nil
}

// NOT0 is invalid gomake namespace because it is not of a struct type.
type NOT0 int

// NOT1 is invalid gomake namespace because underlying type is invalid gomake
// namespace.
type NOT1 NOT0

// NOT2 is invalid gomake namespace because underlying type is invalid gomake
// namespace.
type NOT2 NOT1

// NOT3 is invalid gomake namespace because it is not tagged with "gomake:ns_root".
type NOT3 struct{}

// NOT4 is invalid gomake namespace because it is not of a struct type.
type NOT4 []struct{}

// NOT6 is invalid gomake namespace because it is not of a struct type.
type NOT6 time.Time
