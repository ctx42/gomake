---
title: "Importing Targets"
description: "Share targets across projects using the //gomake:import comment."
weight: 4
---

## Overview

The `//gomake:import` comment tag lets you pull target functions from any Go
package into your makefile. This is the primary mechanism for sharing build
logic across repositories.

---

## Syntax

```go
import _ "github.com/myorg/build-targets/docker"   //gomake:import
import _ "github.com/myorg/build-targets/release"  //gomake:import deploy
```

- Without a namespace argument: targets are merged into the root namespace.
- With a namespace argument (`deploy`): targets are prefixed with `deploy:`.

The import uses the blank identifier (`_`) for the ordinary Go reason: the
package is never referenced in your code, so a named import would fail to
compile as "imported and not used". GoMake reads the target functions from the
package during parsing.

---

## Example

```go
//go:build gomake

package main

import (
    "context"
    "os/exec"

    "github.com/ctx42/ring/pkg/ring"

    _ "github.com/myorg/gomake-targets/docker"   //gomake:import
    _ "github.com/myorg/gomake-targets/infra"    //gomake:import infra
)

// Build compiles the project.
func Build(ctx context.Context, rng *ring.Ring) error {
    cmd := exec.CommandContext(ctx, "go", "build", "./...")
    cmd.Stdout = rng.Stdout()
    cmd.Stderr = rng.Stderr()
    return cmd.Run()
}
```

```shell
$ gomake --list
build           compiles the project
docker:build    builds the Docker image        # from docker package
docker:push     pushes the image               # from docker package
infra:plan      plans infrastructure changes   # from infra package (namespaced)
infra:apply     applies infrastructure changes # from infra package (namespaced)
```

---

## Writing a shareable target package

A shareable package is a regular Go package with functions matching the
target signature. It does not need `package main`:

```go
// Package docker provides reusable Docker build targets.
package docker

import (
    "context"
    "os/exec"

    "github.com/ctx42/ring/pkg/ring"
)

// Build builds the Docker image for the current project.
func Build(ctx context.Context, rng *ring.Ring) error {
    cmd := exec.CommandContext(ctx, "docker", "build", ".")
    cmd.Stdout = rng.Stdout()
    cmd.Stderr = rng.Stderr()
    return cmd.Run()
}

// Push pushes the most recently built image to the registry.
func Push(ctx context.Context, rng *ring.Ring) error {
    cmd := exec.CommandContext(ctx, "docker", "push", "myorg/myimage")
    cmd.Stdout = rng.Stdout()
    cmd.Stderr = rng.Stderr()
    return cmd.Run()
}
```

Place the package in its own module and `go get` it from the project that
uses it.

---

## Import resolution

GoMake resolves `//gomake:import` packages by running `go list` in the project
directory. This means the imported package must be available in the project's
module graph (i.e. present in `go.mod` / `go.sum`).

Multiple imports are resolved concurrently. An error in any import aborts the
build. Packages inside the current module are resolved in-process without
`go list`, and results for external, versioned modules are cached on disk, so
repeat runs stay fast.

---

## Namespaced imports

The second word after `//gomake:import` becomes the namespace prefix. It is
converted to lowercase regardless of how it is written in the source:

```go
import _ "github.com/myorg/targets/db"  //gomake:import DB
```

```shell
$ gomake --list
db:migrate   ...
db:rollback  ...
```

---

## What counts as a valid import comment

| Comment | Valid? | Namespace |
|---------|--------|-----------|
| `//gomake:import` | Yes | root |
| `//gomake:import ns` | Yes | `ns` |
| `//gomake:import NS` | Yes | `ns` (lowercased) |
| `// gomake:import` | Yes | root — a space after `//` is tolerated |
| `//gomake:` | No — missing `import` keyword |  |
| `//notgomake:import` | No — wrong prefix |  |
| `//gomake:import ns extra` | No — too many words |  |

Unlike the `// gomake:hidden` tag, `//gomake:import` is read straight from the
source comment, so a space after `//` makes no difference here.
