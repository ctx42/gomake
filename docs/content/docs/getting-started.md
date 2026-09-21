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
# gomake v0.26.1
```

### With custom built-in targets

Pass a `targets.yaml` to compile external target packages permanently into the
binary at install time:

```shell
go run github.com/ctx42/gomake/cmd/install@latest --targets=./targets.yaml
```

Or forward `--targets` through the install script with `sh -s --`:

```shell
curl -fsSL https://raw.githubusercontent.com/ctx42/gomake/master/install.sh | sh -s -- --targets=./targets.yaml
```

See [Builtin targets]({{< relref "builtin-targets" >}}) for details.

### Installing from a local clone

To build from a checked-out copy of the source instead of a published
release — for instance to install unreleased changes on `master` — run the
installer from the clone. It builds whatever is in your working tree; no tag,
release, or module proxy is involved:

```shell
go run ./cmd/install
```

`--targets` works the same way:

```shell
go run ./cmd/install --targets=./targets.yaml
```

Use the package path `./cmd/install`, not a file path like
`cmd/install/install.go` — building from an explicit file leaves the build
metadata empty, and the installer then mistakes the run for a published
install and fails.

---

## Versioning

`gomake --version` reports the version the Go toolchain recorded for the
binary, so the string says where the build came from:

| Build                             | Version                                       |
|-----------------------------------|-----------------------------------------------|
| Published release (`…@latest`)    | `v0.26.1`                                     |
| From a clone                      | `v0.26.2-0.20260921121020-5bde820e3c1c+dirty` |
| From a source copy, no repository | `(devel)`                                     |

The middle form is a Go pseudo-version: the last tag with its patch raised, a
pre-release stamp holding the commit time and hash, and `+dirty` when the
work tree carried uncommitted changes. It sorts after `v0.26.1` and before
`v0.26.2`, so a build from a clone never claims to be the release it was
built on top of.

A published install is the one build the toolchain cannot version by itself:
it compiles from the module cache, which carries no repository to read. The
installer stamps the version there with `-ldflags -X`.

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
