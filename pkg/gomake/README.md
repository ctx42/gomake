# gomake

Package `gomake` is the toolkit available to gomake target authors. It exposes
the running target's name, locates the module root, decodes a target's
configuration block, and provides small filesystem and environment helpers. It
is the only gomake package a `makefile.go` or a built-in target package needs
to import.

## Install

```
go get github.com/ctx42/gomake/pkg/gomake
```

Import path:

```go
import "github.com/ctx42/gomake/pkg/gomake"
```

## Usage

A target reads the name it was invoked under, then locates the module root to
resolve paths relative to the project:

```go
package main

import (
	"context"
	"fmt"

	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
)

func target(ctx context.Context, rng *ring.Ring) error {
	name, ok := gomake.TargetName(ctx)
	if !ok {
		return fmt.Errorf("target name not set")
	}

	root, err := gomake.Root(".")
	if err != nil {
		return err
	}

	fmt.Printf("running %q from module root %s\n", name, root)
	return nil
}
```
