---
title: "Writing Targets"
description: "How to write, document, and organise GoMake targets."
weight: 2
---

## Target signature

Every target must have exactly this signature:

```go
func TargetName(ctx context.Context, rng *ring.Ring) error
```

GoMake ignores functions that do not match. Invalid signatures are silently
skipped, not errors. Only the parameter *types* matter — the `ctx` and `rng`
names are yours to choose. One caveat: the second parameter must refer to the
`ring` package by its real name, so avoid aliasing the import (`r
"github.com/ctx42/ring/pkg/ring"`) in a makefile, or the function will not be
recognised as a target.

| Parameter | Type              | Purpose                                            |
|-----------|-------------------|----------------------------------------------------|
| `ctx`     | `context.Context` | Cancelled on interrupt; pass to all blocking calls |
| `rng`     | `*ring.Ring`      | I/O, environment, and argument access              |
| return    | `error`           | Non-nil causes a non-zero exit                     |

### The Ring type

`ring.Ring` is the execution environment for a target:

```go
rng.Stdout()      // io.Writer — write target output here
rng.Stderr()      // io.Writer — write errors and logs here
rng.Args()        // []string  — arguments after the target name
rng.EnvGet("KEY") // string    — read an environment variable
```

Always use `rng.Stdout()` / `rng.Stderr()` instead of `os.Stdout` /
`os.Stderr`. This allows GoMake to capture output in tests and redirections.

---

## Naming

The CLI name is the function name converted to `kebab-case`:

| Function name | CLI name    |
|---------------|-------------|
| `Build`       | `build`     |
| `RunTests`    | `run-tests` |
| `CILint`      | `ci-lint`   |

---

## Documentation

The first sentence of a Go doc comment becomes the synopsis shown by
`--list`. The full comment appears in `--help <target>`.

```go
// Build compiles the project for the current platform. It reads GOOS and
// GOARCH from the environment for cross-compilation and writes the output
// binary to the dist/ directory.
func Build(ctx context.Context, rng *ring.Ring) error { ... }
```

```shell
$ gomake --list
build   compiles the project for the current platform

$ gomake --help build
Usage:
  build

Description:
  Build compiles the project for the current platform. It reads GOOS and
  GOARCH from the environment for cross-compilation and writes the output
  binary to the dist/ directory.
```

---

## Arguments

Use `rng.Args()` to access extra arguments passed after the target name:

```go
func Greet(ctx context.Context, rng *ring.Ring) error {
    name := "World"
    if args := rng.Args(); len(args) > 0 {
        name = args[0]
    }
    out := rng.Stdout()
    fmt.Fprintf(out, "Hello, %s!\n", name)
    return nil
}
```

```shell
gomake greet Alice
# Hello, Alice!
```

### Flag parsing inside a target

Use `flag.FlagSet` for structured argument parsing:

```go
func Deploy(ctx context.Context, rng *ring.Ring) error {
    fs := flag.NewFlagSet("deploy", flag.ContinueOnError)
    env := fs.String("env", "staging", "deployment environment")
    dry := fs.Bool("dry-run", false, "print actions without executing")
    if err := fs.Parse(rng.Args()); err != nil {
        return err
    }
    fmt.Fprintf(rng.Stdout(), "Deploying to %s (dry=%v)\n", *env, *dry)
    return nil
}
```

```shell
gomake deploy --env production --dry-run
```

---

## Default target

Set `var Default` to the function to run when no target name is given:

```go
var Default = Build

func Build(ctx context.Context, rng *ring.Ring) error { ... }
```

```shell
gomake        # runs Build
gomake build  # same
```

`Default` can also point to a namespace method:

```go
var Default = CI.All
```

---

## Hidden targets

Add a `// gomake:hidden` line anywhere in a target's doc comment to exclude it
from `--list` output. The target still runs normally by name. You may add a
trailing note explaining why it is hidden.

```go
// SetupEnv prepares the build environment.
//
// gomake:hidden internal helper, not meant to be run directly
func SetupEnv(ctx context.Context, rng *ring.Ring) error { ... }
```

Note the space after `//`: the tag is read as documentation, so
`// gomake:hidden` works but `//gomake:hidden` (no space) is treated as a Go
compiler directive and is stripped before GoMake sees it. The target must also
be exported — an unexported function is not a target at all, hidden or not.

Hidden targets are useful for internal helpers that shouldn't be visible to
users but are needed by other targets.

---

## Build tags

Use the `//go:build gomake` tag to keep makefile code out of your regular
`go build`:

```go
//go:build gomake

package main

import (
    "context"
    "github.com/ctx42/ring/pkg/ring"
)

func Build(ctx context.Context, rng *ring.Ring) error { ... }
```

GoMake strips the tag before compiling, so the file compiles normally in its
temporary build directory. Files without the tag work too.

---

## OS- and arch-specific makefiles

All your targets live in `makefile.go`. The only additional makefile files
gomake loads are OS/arch variants, named with Go's standard filename build
constraints:

```
makefile.go               # main targets (required)
makefile_linux.go         # only when GOOS=linux
makefile_windows.go       # only when GOOS=windows
makefile_amd64.go         # only when GOARCH=amd64
makefile_linux_amd64.go   # only when GOOS=linux AND GOARCH=amd64
```

The suffix after `makefile_` must be a valid `GOOS`, a valid `GOARCH`, or
`GOOS_GOARCH` in that order. gomake builds with the `GOOS`/`GOARCH` from your
environment, so to cross-compile you just set them:

```shell
GOOS=windows GOARCH=amd64 gomake build
```

Files whose suffix is not a real `GOOS`/`GOARCH` (for example
`makefile_docker.go`) are **ignored with a warning** — gomake does not support
splitting unrelated targets across custom-named files; keep them in
`makefile.go`. All files must declare `package main`, and you must not mix
tagged and untagged makefile sources in the same directory.

---

## Calling external commands

Use `os/exec` and pass `ctx` for cancellation:

```go
func Lint(ctx context.Context, rng *ring.Ring) error {
    cmd := exec.CommandContext(ctx, "golangci-lint", "run", "./...")
    cmd.Stdout = rng.Stdout()
    cmd.Stderr = rng.Stderr()
    return cmd.Run()
}
```

A helper wrapping exec is a common pattern:

```go
func run(ctx context.Context, rng *ring.Ring, args ...string) error {
    cmd := exec.CommandContext(ctx, args[0], args[1:]...)
    cmd.Stdout = rng.Stdout()
    cmd.Stderr = rng.Stderr()
    return cmd.Run()
}
```

---

## Knowing the target name at runtime

Inside a target, `gomake.TargetName(ctx)` returns the name the target was
invoked with:

```go
import "github.com/ctx42/gomake/pkg/gomake"

func MyTarget(ctx context.Context, rng *ring.Ring) error {
    name, _ := gomake.TargetName(ctx)
    fmt.Fprintf(rng.Stdout(), "running as: %s\n", name)
    return nil
}
```
