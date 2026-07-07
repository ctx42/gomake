---
title: "Getting Started"
description: "Install GoMake and run your first target in five minutes."
weight: 1
---

## Installation

Install with a single command (requires Go 1.26+):

```shell
curl -fsSL https://raw.githubusercontent.com/ctx42/gomake/master/install.sh | sh
```

Or directly with `go run`:

```shell
go run github.com/ctx42/gomake/cmd/install@latest
```

After installation, verify it works:

```shell
gomake --version
# gomake v0.12.1, hash: abc1234, ...
```

### Embedding a CI/CD tag

Set `GOMAKE_CCID` to embed a build identifier in the version string:

```shell
GOMAKE_CCID=build-42 go run github.com/ctx42/gomake/cmd/install@latest
```

### With custom built-in targets

Pass a `targets.yaml` to compile external target packages permanently into the
binary at install time:

```shell
go run github.com/ctx42/gomake/cmd/install@latest --targets=./targets.yaml
```

See [Builtin targets]({{< relref "builtin-targets" >}}) for details.

---

## Your first makefile

Create `makefile.go` in your project root. Your targets live in
`makefile.go`; the only other files GoMake loads are OS/arch variants such as
`makefile_linux.go` (see [Writing targets]({{< relref "writing-targets" >}})):

```go
//go:build gomake

package main

import (
    "context"
    "fmt"

    "github.com/ctx42/ring/pkg/ring"
)

// Hello prints a greeting.
func Hello(ctx context.Context, rng *ring.Ring) error {
    fmt.Fprintln(rng.Stdout(), "Hello from GoMake!")
    return nil
}
```

Run it:

```shell
gomake hello
# Hello from GoMake!
```

List targets:

```shell
gomake --list
# hello   prints a greeting
```

Get help:

```shell
gomake --help hello
```

---

## What happens on first run

1. GoMake finds `makefile.go` in the current directory.
2. It creates a temporary build directory and copies your makefile sources.
3. It generates a thin `main` package that wires your targets to the runner.
4. It compiles the package with `go build`.
5. It caches the binary under `~/.cache/gomake/bin/`.
6. It runs the binary with the target name and any extra arguments.

On subsequent runs with unchanged sources, steps 1–5 are skipped. The cached
binary is used directly.

---

## Next steps

- [Writing targets]({{< relref "writing-targets" >}}) — full target syntax
- [Namespaces]({{< relref "namespaces" >}}) — group targets into hierarchies
- [Importing targets]({{< relref "imports" >}}) — share targets across projects
- [CLI reference]({{< relref "cli-reference" >}}) — all flags and options
