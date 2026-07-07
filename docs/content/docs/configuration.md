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

For a target's configuration block, the project file replaces the user file's
block for that target as a whole; the two are not deep-merged. When GoMake runs
inside a module, a user-level entry for one of that module's own targets is
ignored — configure local targets in the project file.

## Schema

```yaml
version: 1                       # required, integer schema version
settings:                        # configuration for GoMake itself
  timeout: 30s                   # default target execution timeout
  tmp: /abs/path/to/tmp          # default temporary directory (absolute)
targets:                         # per-target configuration
  github.com/acme/tasks#Deploy:  # <import-path>#<name>
    region: eu
    hosts: [web-1, web-2]
  example.com/proj#Project.Setup:
    dirs: [cmd, internal]
```

The `version` field is required and must be an integer. GoMake aborts when a
file declares a version newer than the running binary supports. Unknown keys
under `version`, `settings`, or `targets` are rejected; the contents of a
single target block are opaque to GoMake and never validated.

## Target keys

Each target block is keyed by the target's canonical origin:

```
<import-path>#<name>
```

`<name>` is the Go identifier as declared in the target's own package: a plain
function contributes its function name (`Deploy`), and a namespaced method
contributes `<Receiver>.<Method>` (`Project.Setup`). The key follows the
target's home package, so renaming an import with `//gomake:import` does not
change it.

Run `gomake --check-config` to print the exact key for every discovered target,
so there is no need to guess:

```shell
gomake --check-config
```

The same command reports project-level keys that match no target and any
`settings.tmp` value that is not absolute.

## How configuration reaches a target

Only the *invoked* target receives configuration, and it receives only its own
block. The value travels from the YAML file to your function through a fixed
pipeline:

```text
gomake.yaml  (user file + project file)
    │   1. load, validate, merge   (project block wins, whole-block replace)
    ▼
merged targets map                 keyed by <import-path>#<name>
    │   2. select the invoked target's block; drop settings + other targets
    ▼
opaque YAML block
    │   3. json.Marshal            (YAML → JSON string)
    ▼
JSON string
    │   4. MetaSet under gomake.ConfigMetaKey
    ▼
gomake's ring meta store  ──────►  in-process target reads it here and stops
    │   5. makefile compiled to its own binary → run as a subprocess
    │      ferry JSON as --gomake-config=<json>; the generated main strips
    ▼      the argument and re-stores it under the same key
subprocess ring meta store  ────►  6. target decodes with gomake.TargetConfig
```

Step by step:

1. **Load and merge.** GoMake reads the user-level and project-level files,
   validates the `version` and `settings` keys, and keeps every target block as
   an opaque value. The two files are merged so that, for any given target, the
   project-level block replaces the user-level block as a whole (see
   [Precedence](#precedence)).

2. **Select one block.** When you run a target, GoMake computes that target's
   canonical `<import-path>#<name>` key and looks it up in the merged `targets`
   map. At most one block is selected — the invoked target's. The `settings`
   section and every other target's block are left behind.

3. **Convert YAML to JSON.** The selected block is re-encoded from YAML into a
   JSON string with `encoding/json`. A target therefore decodes JSON, not YAML,
   and tags its fields with `json:"..."`. YAML scalars, sequences, and mappings
   become their natural JSON counterparts (string, number, bool, array,
   object).

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

6. **Decode in the target.** `gomake.TargetConfig` looks the JSON string up by
   key and unmarshals it into your value.

The environment is deliberately never used to carry configuration; it is left
free for a target's own override logic. A target that has no matching block
sees an unset meta key, and `TargetConfig` leaves your value untouched.

## Reading configuration in a target

A target receives only its own configuration block, as a JSON value in the ring
meta store. Decode it into any type with `gomake.TargetConfig`, which uses the
standard-library JSON decoder — no extra dependency is required.

Given this project-level block:

```yaml
version: 1
targets:
  github.com/acme/tasks#Deploy:
    region: eu
    hosts: [web-1, web-2]
    replicas: 3
    dry_run: false
```

GoMake delivers it to the target as the JSON string:

```json
{"region":"eu","hosts":["web-1","web-2"],"replicas":3,"dry_run":false}
```

which the target decodes into a struct — or any JSON-compatible type, such as a
`map[string]any`:

```go
package main

import (
    "context"

    "github.com/ctx42/ring/pkg/ring"

    "github.com/ctx42/gomake/pkg/gomake"
)

// Deploy reads its configuration and deploys accordingly.
func Deploy(_ context.Context, rng *ring.Ring) error {
    var cfg struct {
        Region   string   `json:"region"`
        Hosts    []string `json:"hosts"`
        Replicas int      `json:"replicas"`
        DryRun   bool     `json:"dry_run"`
    }
    if err := gomake.TargetConfig(rng, &cfg); err != nil {
        return err
    }
    // cfg holds its zero value when the target has no configuration; supply
    // your own defaults for the fields you require.
    _ = cfg
    return nil
}
```

`TargetConfig` returns an error only when the block is present but fails to
decode into `v` (for example, a type mismatch); a missing block is not an
error. The block is opaque to GoMake, so it is validated only here, against
your own type — a typo in a key name silently leaves that field at its zero
value.

A target never sees the `settings` section or another target's block. When one
target calls another as a plain Go function, GoMake applies no configuration to
the callee — only the top-level invoked target's block is delivered, so pass
whatever the callee needs as function arguments.

## Relationship to targets.yaml

`gomake.yaml` is unrelated to `targets.yaml`. The latter declares external
target imports and is consulted only at install time; `gomake.yaml` carries
runtime configuration.
