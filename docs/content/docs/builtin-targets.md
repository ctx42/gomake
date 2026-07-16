---
title: "Builtin Targets"
description: "Compile targets permanently into the gomake binary at install time."
weight: 5
---

## Overview

`targets.yaml` lets you compile target packages permanently into the gomake
binary. These targets become **builtin** — they are available in every
project, even those without a `makefile.go`, and they appear alongside any
user-defined targets when you run `--list`.

This is different from `//gomake:import`, which declares per-project imports
inside a project's own `makefile.go`:

| Mechanism         | Where declared          | Compiled when        | Available in          |
|-------------------|-------------------------|----------------------|-----------------------|
| `targets.yaml`    | gomake repo root        | `cmd/install` runs   | Every project, always |
| `//gomake:import` | project's `makefile.go` | project makefile builds | That project only  |

---

## The targets.yaml file

`targets.yaml` lives at the root of the gomake source repository and is
committed to version control. An empty config is valid:

```yaml
imports:
```

Each entry in `imports` has a required `import` field and two optional fields:

```yaml
imports:
  - import: example.com/myorg/gomake-targets/docker
  - import: example.com/myorg/gomake-targets/release
    namespace: release
  - import: example.com/myorg/gomake-targets/db
    namespace: db
    config:
      host: db.internal
      port: 5432
```

| Field       | Required | Description                                                   |
|-------------|----------|---------------------------------------------------------------|
| `import`    | Yes      | Go import path of the target package                          |
| `namespace` | No       | CLI prefix for all targets from this package (e.g. `release`) |
| `config`    | No       | Arbitrary YAML surfaced as `ring.Ring` metadata (see below)   |

The `import` path may pin a version, e.g.
`example.com/myorg/gomake-targets/release@v1.3.0`. Duplicate import paths are
rejected with an error, and the file is parsed strictly — an unknown field
fails the load rather than being ignored.

---

## How it works

Passing `--targets` to `go run github.com/ctx42/gomake/cmd/install@latest`
runs the target-preparation step, which:

1. Reads the supplied `targets.yaml`.
2. Runs `go get <import-path>` for each listed package. The exception is a
   local `--targets` file inside a Go module (see [Developing targets
   locally](#developing-targets-locally)): that module is resolved from disk
   through a temporary Go workspace, so `go get` is skipped for its packages.
3. Calls `builtin.GenMain`, which parses each package for target functions and
   generates `internal/builtin/targets.go`.
4. Compiles the generated file into the gomake binary.

After installation, the new targets are part of the binary — no `makefile.go`
in a project is needed to make them available.

During installation, a line is printed to stderr for each import:

```
adding external target example.com/myorg/gomake-targets/docker
adding external target example.com/myorg/gomake-targets/release
```

Nothing is printed when `imports` is empty. Passing `--targets=` with an empty
value — for example a shell variable that expanded to nothing — instead prints
`no external targets provided` and continues the install with no external
targets, so the empty expansion is not mistaken for an omitted flag.

---

## Adding an external target

1. Edit your `targets.yaml` and add the entry.
2. Reinstall gomake with the updated file:

```shell
go run github.com/ctx42/gomake/cmd/install@latest --targets=./targets.yaml
```

Or fetch the file from a URL:

```shell
go run github.com/ctx42/gomake/cmd/install@latest --targets=https://example.com/targets.yaml
```

To regenerate `internal/builtin/targets.go` without a full reinstall:

```shell
go generate ./internal/builtin/
```

---

## Developing targets locally

While developing a target package you can build a gomake binary from its
unpublished source, without pushing to a module proxy. Point `--targets` at a
local `targets.yaml` that lives inside the target's Go module:

```shell
GOBIN=$PWD/dist go run -buildvcs=true ./cmd/install --targets=/path/to/module/targets.yaml
```

When the `--targets` file sits inside a Go module, `cmd/install` resolves that
module from disk through a temporary Go workspace instead of fetching it with
`go get`. Your local, uncommitted edits are compiled straight into the binary,
and `go.mod` is left untouched — no `replace` or `require` is added. Re-run the
command after each edit to rebuild.

The `-buildvcs=true` flag records the checked-out revision in the binary; see
[Installing from a local clone]({{< relref "getting-started#installing-from-a-local-clone" >}})
for why `go run` needs it.

Only the module containing the `targets.yaml` is resolved locally; imports from
any other module still come from the proxy via `go get`.

---

## Namespaces

The optional `namespace` field places all targets from a package under a CLI
prefix. Without it, targets merge into the root namespace.

```yaml
imports:
  - import: example.com/myorg/targets/infra
    namespace: infra
```

```shell
$ gomake --list
infra:plan    plans infrastructure changes
infra:apply   applies infrastructure changes
```

---

## The config field

The optional `config` field carries arbitrary YAML that a target can read as
`ring.Ring` metadata. The metadata key is the `namespace` value, or — when
`namespace` is absent — the last path segment of the import path (version
suffixes like `@v2` and `/v2` are stripped).

```yaml
imports:
  - import: example.com/myorg/targets/db
    namespace: db
    config:
      host: db.internal
      port: 5432
```

There is an important detail about *where* this config is read. Only the target
*code* is compiled into the binary — the `config` values are not. At runtime
GoMake loads `config` from a `targets.yaml` in the **project's source
directory** (the `--src` directory, which defaults to the current directory).
So to feed configuration to a built-in target in a given project, place a
`targets.yaml` carrying the `config` block in that project. The `config` from
the gomake repo's install-time `targets.yaml` is *not* embedded in the binary.

A target reads its config via the Ring; each entry's `config` arrives as a JSON
string under its metadata key:

```go
func Migrate(ctx context.Context, rng *ring.Ring) error {
    raw := rng.MetaGet("db") // key == namespace == "db"
    cfg, ok := raw.(string)  // config is passed as a JSON string
    if !ok {
        return errors.New("db: missing config")
    }
    // unmarshal cfg as needed
    return nil
}
```

---

## Writing a package for targets.yaml

A target package for `targets.yaml` uses the same function signatures as any
other target, but lives in a regular (non-`main`) package:

```go
// Package docker provides Docker build and push targets.
package docker

import (
    "context"
    "os/exec"

    "github.com/ctx42/ring/pkg/ring"
)

// Build builds the Docker image.
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

Publish the package as a normal Go module, then reference its import path in
`targets.yaml`.

---

## Comparison: targets.yaml vs //gomake:import

Use **`targets.yaml`** when:
- The targets should be available in every project without any `makefile.go`.
- You are maintaining a shared organisation-wide toolset and want it compiled
  into the standard gomake binary.
- The targets require no per-project configuration (or configuration is
  supplied via the `config` field).

Use **`//gomake:import`** when:
- The targets are specific to one project.
- Different projects import different versions of the package.
- You want the import to be visible in the project's own `makefile.go`.
