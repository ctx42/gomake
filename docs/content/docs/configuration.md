---
title: "Configuration"
description: "Configure GoMake and individual targets with gomake.yaml."
weight: 9
---

## Overview

GoMake reads an optional YAML configuration file named `gomake.yaml`. It holds
two kinds of configuration:

- a `settings` section that configures GoMake itself, and
- a `targets` section that carries per-target configuration a target reads at
  run time.

There are two levels, both using the same file name. The user-level file holds
defaults that apply across every project; the project-level file holds
configuration committed with a specific repository.

Both files are optional. When neither is present, GoMake behaves as if no
configuration existed.

## File locations

| Level   | Path                                  |
|---------|---------------------------------------|
| User    | `$XDG_CONFIG_HOME/gomake/gomake.yaml` |
| Project | `<source-scan-dir>/gomake.yaml`       |

When `XDG_CONFIG_HOME` is unset, the user-level path falls back to
`$HOME/.config/gomake/gomake.yaml`. The source-scan directory is the directory
GoMake searches for `makefile*.go` files — the current directory by default,
or whatever `--src` selects.

## Precedence

When more than one source provides the same value, the highest-priority source
wins:

```
command-line option  >  environment variable  >  project file  >  user file
```

For a target's configuration, GoMake resolves a single block per invocation. It
looks in the project file first and falls back to the user file only when the
project file carries nothing for that target; the two are never deep-merged.
When GoMake runs inside a module, the user-file fallback is skipped for that
module's own targets — configure local targets in the project file.

## Schema

The `targets` section is a nested tree keyed first by import path and then by
the target's invocation-path names — the same lower-case, kebab-cased
identifiers `gomake --list` prints, split on `:`. A namespace becomes a nesting
level; a target's own settings live under its name.

```yaml
version: 1                          # required, integer schema version
settings:                           # configuration for GoMake itself
  timeout: 30s                      # default target execution timeout
  tmp: /abs/path/to/tmp             # default temporary directory (absolute)
targets:                            # per-target configuration
  github.com/acme/tasks:            # import path
    deploy:                         # target :deploy
      region: eu
      hosts: [web-1, web-2]
    go:                             # namespace :go:*
      timeout: 5m                   # shared by every :go target
      lint:                         # sub-namespace :go:lint:*
        version: v2.13.0            # shared by :go:lint and its children
        file: .golangci.yml
```

The `version` field is required and must be an integer. GoMake aborts when a
file declares a version newer than the running binary supports. Unknown keys
under `version` or `settings` are rejected; the target tree is opaque to GoMake
and its contents are never validated (see [Validation](#validation)).

## Target keys

A target's configuration lives at the node reached by walking from its import
path down through its namespace names to the target's own name — exactly the
pieces that make up the name `gomake --list` shows:

```
targets:
  <import-path>:
    <namespace>:      # zero or more namespace levels
      <target-name>:  # the kebab function name, e.g. install or test-v
        <settings>
```

The key follows the target's home package, so renaming an import with
`//gomake:import` does not change it.

GoMake delivers the **nearest-level** block: it walks from the target's own node
up toward the import-path root and uses the first node that carries settings, so
namespace-level keys are shared by every target beneath them while a target's
own block shadows them. A block never includes a nested target's keys.

Run `gomake --check-config` to print every discovered target grouped by import
path, so there is no need to guess the nesting:

```shell
gomake --check-config
```

The same command reports project- and user-level import keys that match no
discovered target, an import config that is not a mapping, and any
`settings.tmp` value that is not absolute.

## How configuration reaches a target

Only the *invoked* target receives configuration, and it receives only its own
nearest-level block. The value travels from the YAML file to your function
through a fixed pipeline:

```text
gomake.yaml  (user file + project file)
    │   1. load + validate; keep each file's target tree
    ▼
per-file target trees              keyed by <import-path>, then by names
    │   2. resolve the invoked target's nearest-level block
    │      (project tree first, user tree as a fallback)
    ▼
opaque YAML block                  child-node keys stripped
    │   3. json.Marshal            (YAML → JSON string)
    ▼
JSON string
    │   4. MetaSet under gomake.ConfigMetaKey
    ▼
gomake's ring meta store  ──────►  in-process target reads it here and stops
    │   5. makefile compiled to its own binary → run as a subprocess
    │      ferry JSON as --gomake-config=<json>; the generated main strips
    ▼      the argument and re-stores it under the same key
subprocess ring meta store  ────►  6. target reads via TargetConfig + GetCfg[T]
```

Step by step:

1. **Load.** GoMake reads the user-level and project-level files, validates the
   `version` and `settings` keys, and keeps each file's `targets` tree as an
   opaque nested value. Only the `settings` sections are merged (see
   [Precedence](#precedence)); the target trees stay separate.

2. **Resolve one block.** When you run a target, GoMake walks the tree for that
   target's import path down its namespace and target names, then back up to the
   nearest node carrying settings, stripping any nested target's keys. It tries
   the project tree first and the user tree as a fallback. At most one block is
   selected — the invoked target's. The `settings` section and every other
   target's block are left behind.

3. **Convert YAML to JSON.** The selected block is re-encoded from YAML into a
   JSON string with `encoding/json`. A target therefore reads JSON values, not
   YAML, and a struct decoded from the block tags its fields with `json:"..."`.
   YAML scalars, sequences, and mappings become their natural JSON counterparts
   (string, number, bool, array, object).

4. **Store in the ring meta store.** GoMake places the JSON string in the ring
   meta store under the exported key `gomake.ConfigMetaKey`. For a target run
   in-process — the [library]({{< relref "library" >}}) use case, where you
   embed the makefile in your own program — this is the end of the journey.

5. **Cross the process boundary.** A generated makefile is compiled to its own
   binary and executed as a subprocess, which does not share the parent
   process's meta store. GoMake ferries the same JSON string to it as an
   internal `--gomake-config=<json>` command-line argument. The generated
   `main` strips that argument from `os.Args` — so it never reaches your target
   as a positional argument — and re-stores its value under
   `gomake.ConfigMetaKey` in the subprocess ring.

6. **Read in the target.** `gomake.TargetConfig` looks the JSON string up by
   key and decodes it once into a `*Config`; `gomake.GetCfg[T]` then pulls typed
   values from it by path.

The environment is deliberately never used to carry configuration; it is left
free for a target's own override logic. A target that has no matching block
sees an unset meta key, so `TargetConfig` returns an empty `Config` and every
`GetCfg` on it returns `ErrMiss`.

## Reading configuration in a target

A target receives only its own configuration block, as a JSON value in the ring
meta store. Build a `*gomake.Config` from the ring once with
`gomake.TargetConfig`, then read typed values by path with the generic
`gomake.GetCfg[T]` — no extra dependency is required.

Given this project-level block:

```yaml
version: 1
targets:
  github.com/acme/tasks:
    deploy:
      region: eu
      hosts: [web-1, web-2]
      replicas: 3
      dry_run: false
```

GoMake delivers it to the target as the JSON string:

```json
{"region":"eu","hosts":["web-1","web-2"],"replicas":3,"dry_run":false}
```

which the target reads value by value:

```go
package main

import (
    "context"
    "errors"

    "github.com/ctx42/ring/pkg/ring"

    "github.com/ctx42/gomake/pkg/gomake"
)

// Deploy reads its configuration and deploys accordingly.
func Deploy(_ context.Context, rng *ring.Ring) error {
    cfg, err := gomake.TargetConfig(rng)
    if err != nil {
        return err
    }

    region, err := gomake.GetCfg[string](cfg, "region")
    if err != nil && !errors.Is(err, gomake.ErrMiss) {
        return err
    }
    hosts, err := gomake.GetCfg[[]string](cfg, "hosts")
    if err != nil && !errors.Is(err, gomake.ErrMiss) {
        return err
    }
    // region is "" and hosts is nil when their keys are absent; supply your
    // own defaults for the values you require.
    _, _ = region, hosts
    return nil
}
```

### The `GetCfg[T]` contract

`GetCfg[T](cfg, path)` resolves `path` against the block and returns the
value as `T`:

- **Path.** Dot-separated, resolved against each node's type: a map segment is
  a key, an array segment a zero-based index (`"lint.file"`, `"hosts.0"`). Wrap
  a segment in single quotes to address a key that itself contains a dot — an
  import path, say — as in `"modules.'github.com/acme/app'.package"`.
- **Types.** The value is converted through a JSON round-trip, so `T` may be a
  scalar, a slice, a map, or a json-tagged struct (`GetCfg[[]string]`,
  `GetCfg[Deploy]`). `time.Duration` is the one string conversion — it is parsed
  with `time.ParseDuration`, so a YAML `timeout: 5m` reads as
  `GetCfg[time.Duration](cfg, "timeout")`. `GetCfg[any]` returns the raw decoded
  value.
- **Strictness.** Conversion is strict: a number becomes an integer only when
  it has no fractional part, a value of the wrong JSON kind is rejected, and
  JSON `null` yields `ErrType` for a concrete `T` (so `GetCfgDefault` does
  not treat null as a typed zero value).
- **Errors.** `ErrMiss` when the path is absent, out of range, or empty;
  `ErrType` on a type mismatch, a JSON null, or a failed duration parse. Use
  `errors.Is(err, gomake.ErrMiss)` to treat an optional key as a default, and
  `cfg.Has(path)` to test presence without an error.

Because Go methods cannot be generic, `GetCfg` is a package-level function
taking the `*Config` as its first argument, not a method on `Config`.

The block is opaque to GoMake and validated only here, against the type you
ask for — a typo in a path resolves to `ErrMiss` rather than a compile error,
so cover the paths your target reads with tests.

A target never sees the `settings` section or another target's block. When one
target calls another as a plain Go function, GoMake applies no configuration to
the callee — only the top-level invoked target's block is delivered, so pass
whatever the callee needs as function arguments.

## Validation

GoMake navigates the target tree structurally but never judges a block's
contents. It tells a child-node key (a known namespace or target name) from a
setting key (anything else), strips the child nodes, and delivers the rest. A
mistyped key it cannot recognise as a child is simply carried into the delivered
block.

Validation is therefore the target's job. Decode the block into a type that
rejects unknown fields and a typo becomes a hard error the target raises; leave
the decoder lenient and the typo silently leaves its field at the zero value. A
key the target knows but does not use is harmless — targets that share a
namespace node share its settings and each ignores the keys outside its own
schema.

`--check-config` checks only structure: it flags a top-level key matching no
discovered import path and an import config that is not a mapping. It never
judges setting keys, which are opaque to GoMake.

## Relationship to targets.yaml

`gomake.yaml` is unrelated to `targets.yaml`. The latter declares external
target imports and is consulted only at install time; `gomake.yaml` carries
runtime configuration.
