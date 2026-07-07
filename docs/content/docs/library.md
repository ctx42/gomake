---
title: "Target Author Toolkit"
description: "The pkg/gomake helpers available when writing GoMake targets."
weight: 8
---

## GoMake is a tool, not a library

GoMake is a command-line tool. Its machinery — parsing, out-of-source
builds, the makefile runtime, built-in target generation, and installation —
lives in `internal/` packages that cannot be imported from other modules.
There is no supported way to drive GoMake programmatically, and that is by
design.

The one public package, `pkg/gomake`, is **not** an entry point for building
tooling on top of GoMake. It is a small toolkit of helpers for the code you
write *as* GoMake targets: functions in a project's `makefile.go`, or the
external and built-in target packages compiled into the binary.

## The only import you need

```go
import "github.com/ctx42/gomake/pkg/gomake"
```

It is the only GoMake package a `makefile.go` or a target package needs — or
is able — to import. A target itself is an ordinary function:

```go
func Name(ctx context.Context, rng *ring.Ring) error
```

The `ctx` carries the running target's name; the `rng`
([`ring.Ring`](https://pkg.go.dev/github.com/ctx42/ring/pkg/ring)) carries the
arguments, environment, standard streams, and metadata. The `pkg/gomake`
helpers operate on those values.

## What's in the toolkit

| Area                | Helpers                                                                                                               |
|---------------------|-----------------------------------------------------------------------------------------------------------------------|
| Target context      | `TargetName`, `WithTargetName`                                                                                        |
| Configuration       | `TargetConfig`, `ConfigMetaKey`                                                                                       |
| Module & paths      | `Root`, `PathExists`, `FileExists`, `DirExists`, `ReadFile`                                                           |
| Environment         | `Getenv`, `LookupEnv`, `GetGOOS`, `GetGOARCH`, `EnvSplit`, `EnvJoin`, `EnvSplitOrdered`, `Expander`, `PrettyPrintEnv` |
| Interactive input   | `ReadLine`, `ReadChar`                                                                                                |
| Errors & exit codes | `ExitStatus`, `HasRun`, `ErrNoGoMod`                                                                                  |
| Constants           | `CCIDEnvKey`, `ProjectDirEnvKey`, `VersionEnvKey`                                                                     |

## Using the helpers in a target

```go
package targets

import (
    "context"
    "fmt"

    "github.com/ctx42/ring/pkg/ring"

    "github.com/ctx42/gomake/pkg/gomake"
)

// Info reports details about the current invocation.
func Info(ctx context.Context, rng *ring.Ring) error {
    env := rng.EnvAll()

    name, _ := gomake.TargetName(ctx)
    root, err := gomake.Root(".")
    if err != nil {
        return err
    }

    fmt.Fprintf(
        rng.Stdout(),
        "target=%s goos=%s root=%s\n",
        name, gomake.GetGOOS(env), root,
    )
    return nil
}
```

`TargetName` reads the name the runtime set for this call, `Root` walks up to
the module's `go.mod`, and `GetGOOS` resolves the effective `GOOS` from the
target's environment (falling back to the runtime value).

## Reading interactive input

`ReadLine` and `ReadChar` take any `io.Reader`, so a target can prompt the
user:

```go
func Confirm(ctx context.Context, rng *ring.Ring) error {
    fmt.Fprint(rng.Stdout(), "continue? [y/N] ")
    answer, err := gomake.ReadLine(os.Stdin)
    if err != nil {
        return err
    }
    if answer != "y" {
        return errors.New("aborted")
    }
    return nil
}
```

## Browse the API

```shell
go doc github.com/ctx42/gomake/pkg/gomake
go doc github.com/ctx42/gomake/pkg/gomake Root
```

Runnable examples live in `pkg/gomake/examples_test.go`:

```shell
go test ./pkg/gomake -run '^Example'
```
