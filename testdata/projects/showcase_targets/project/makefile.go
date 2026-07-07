// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/pkg/gomake"
)

// Basic prints its name to stdout. Some more detailed documentation here.
// Even more detailed documentation.
func Basic(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "Basic")
	return nil
}

func BasicArgs(ctx context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "BasicArgs", rng.Args())
	return nil
}

func BasicEnv(ctx context.Context, rng *ring.Ring) error {
	key := "PWD"
	args := rng.Args()
	if len(args) > 0 {
		key = args[0]
	}
	val := rng.EnvGet(key)
	_, _ = fmt.Fprintf(rng.Stdout(), "BasicEnv %s=%s", key, val)
	return nil
}

func SayHello(ctx context.Context, rng *ring.Ring) error {
	fs := flag.NewFlagSet("say-hello", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var name string
	fs.StringVar(&name, "to", "the World", "target argument")
	if err := fs.Parse(rng.Args()); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(rng.Stdout(), "gomake says hello to %s\n", name)
	return nil
}

func PrintArgs(ctx context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), rng.Args())
	return nil
}

func PanicString(_ context.Context, _ *ring.Ring) error {
	panic("panic string")
}

func Long(ctx context.Context, rng *ring.Ring) error {
	time.Sleep(100 * time.Millisecond)
	_, _ = fmt.Fprint(rng.Stdout(), "done after 100ms")
	return nil
}

// ==== All the targets below this line have invalid signatures. ====

// ArgInvMultiCtx is invalid target because it has multiple contexts.
func ArgInvMultiCtx(ctx0 context.Context, ctx1 context.Context) error {
	return nil
}

// ArgInvMultiCtxReuse is invalid target because it has multiple contexts.
func ArgInvMultiCtxReuse(ctx0, ctx1 context.Context) error {
	return nil
}

// ArgInvNoArgs is invalid target because it has no arguments.
func ArgInvNoArgs() error {
	return nil
}

// ArgInvNoCtx is invalid target because it does not have context as a first
// argument.
func ArgInvNoCtx(args []string, i int) error {
	return nil
}

// ArgInvCtxPosition is invalid target because context must be the first on the
// argument list.
func ArgInvCtxPosition(rng *ring.Ring, ctx context.Context) error {
	return nil
}

// ArgInvType is invalid target because the second argument must be
// an argument slice.
func ArgInvType(context.Context, complex64) error {
	return nil
}

// RetInvType is invalid target because it does not return an error.
func RetInvType(_ context.Context, rng *ring.Ring) int {
	_, _ = fmt.Fprint(rng.Stdout(), "RetInvType")
	return 0
}

// RetInvMulti is invalid target because it does not have single return.
func RetInvMulti(_ context.Context, rng *ring.Ring) (int, error) {
	_, _ = fmt.Fprint(rng.Stdout(), "RetInvMulti")
	return 0, nil
}

// ArgInvToMany is invalid target because it has too many arguments.
func ArgInvToMany(ctx context.Context, args0 []string, args1 []string) error {
	return nil
}

// ArgInvToManyReuse is invalid target because it has too many arguments.
func ArgInvToManyReuse(ctx context.Context, args0, args1 []string) error {
	return nil
}

// notExported is invalid target because it is not exported.
func notExported(ctx context.Context, rng *ring.Ring) error { return nil }

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

type NSDef struct{} //gomake:ns_root

func (NSDef) Default(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "NSDef.Default")
	return nil
}

func (NSDef) Hello(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "NSDef.Hello")
	return nil
}

func (NSDef) World(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "NSDef.World")
	return nil
}
