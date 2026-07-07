---
title: "Namespaces"
description: "Group related targets into named hierarchies."
weight: 3
---

## Overview

Namespaces let you group related targets under a common prefix. GoMake uses
Go struct types as namespaces — a pattern that integrates cleanly with the
language and IDE tooling.

---

## Declaring a namespace

Define a struct type and mark it as a namespace root with the
`//gomake:ns_root` inline comment:

```go
// Docker groups container image targets.
type Docker struct{} //gomake:ns_root

// Build builds the Docker image.
func (Docker) Build(ctx context.Context, rng *ring.Ring) error { ... }

// Push pushes the image to the registry.
func (Docker) Push(ctx context.Context, rng *ring.Ring) error { ... }
```

```shell
$ gomake --list
docker:build   builds the Docker image
docker:push    pushes the image to the registry

$ gomake docker:build
$ gomake docker:push
```

By convention the namespace type is an empty struct (`struct{}`), since it only
serves to group targets. The `//gomake:ns_root` comment must appear on the same
line as the type declaration.

---

## Kebab-case conversion

Namespace and target names are converted from `CamelCase` to `kebab-case`
automatically:

| Go name         | CLI name         |
|-----------------|------------------|
| `Docker`        | `docker`         |
| `DockerRelease` | `docker-release` |
| `CI`            | `ci`             |
| `CIDocker`      | `ci-docker`      |

---

## Nested namespaces

Create a nested namespace by aliasing one type to another:

```go
type CI struct{} //gomake:ns_root

func (CI) Lint(ctx context.Context, rng *ring.Ring) error { ... }

// Docker nests under CI.
type Docker CI

func (Docker) Build(ctx context.Context, rng *ring.Ring) error { ... }
func (Docker) Push(ctx context.Context, rng *ring.Ring) error  { ... }
```

```shell
$ gomake --list
ci:lint           ...
ci:docker:build   ...
ci:docker:push    ...
```

The breadcrumb trail is built by following the chain of type aliases until a
`//gomake:ns_root` struct is found. Each segment is kebab-cased from its own
type name, so the child type here is named `Docker` (giving `ci:docker`), not
`CIDocker` (which would give `ci:ci-docker`). The chain can span multiple files
but must stay within the same package.

---

## Default target in a namespace

`var Default` can point to a namespace method:

```go
var Default = CI.Lint
```

Running `gomake` without a target name calls `CI.Lint`.

---

## Namespace with a default method

Declare a method named `Default` on a namespace type to set the default for
that namespace prefix:

```go
type Release struct{} //gomake:ns_root

func (Release) Default(ctx context.Context, rng *ring.Ring) error { ... }
func (Release) Publish(ctx context.Context, rng *ring.Ring) error { ... }
```

```shell
gomake release          # runs Release.Default
gomake release:publish  # runs Release.Publish
```

---

## Namespace rules

- The namespace type must be a struct; use `struct{}` by convention.
- The `//gomake:ns_root` comment must be on the same line as the type.
- A namespace can only be rooted once — you cannot have two
  `//gomake:ns_root` structs that alias each other.
- Invalid chains (e.g. aliasing a built-in type or a non-struct type) are
  silently ignored — the type is not treated as a namespace.
