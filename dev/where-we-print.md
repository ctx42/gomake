# Where gomake prints to a terminal

Inventory of functions that write to a terminal (stdout/stderr), grouped by
binary. Output flows through `rng.Stdout()`/`rng.Stderr()` (the external
`ring.Ring`, bound to `os.Stdout`/`os.Stderr` by `ring.New()`) or, in a few
dev-only tools, directly to `os.Stderr`. Each entry lists the enclosing
function, not every individual print call.

## `gomake` binary (`cmd/gomake` -> `cli.Main`)

| Function                | File                                  | What it prints                                                                                              |
|-------------------------|---------------------------------------|-------------------------------------------------------------------------------------------------------------|
| `cli.Main`              | `internal/cli/main.go:42`             | Bulk of it: version, help, all error lines to stderr; shell-completion to **stdout** (`:92`)                |
| `cli.(*goMake).Execute` | `internal/cli/cli.go:118`             | **The task runner** — wires the compiled makefile subprocess `Stdout`/`Stderr` to the terminal (`:156-157`) |
| `cli.(*goMake).Compile` | `internal/cli/cli.go:103`             | `"Compiling makefile..."` progress to stderr (via `withProgress`)                                           |
| `cli.withProgress`      | `internal/cli/progress.go:19`         | Progress/spinner to the writer it's handed (always `rng.Stderr()`)                                          |
| `cli.runWithoutCompile` | `internal/cli/main.go:266`            | Error lines to stderr (`:277`, `:281`)                                                                      |
| `cli.PrepareTargets`    | `internal/cli/pull_orchestrate.go:21` | Announces each external-target import to stderr (`:30`)                                                     |
| `cli.runCheckConfig`    | `internal/cli/config_file.go:386`     | Config-check report to stderr (`:404`)                                                                      |

## Generated makefile binary (embedded template) + `mkf` runtime

| Function                  | File                           | What it prints                                                                                        |
|---------------------------|--------------------------------|-------------------------------------------------------------------------------------------------------|
| `run` (template)          | `internal/cli/gen_main.go:78`  | **Top-level of the generated binary** (embedded, has `{{...}}` placeholders); panics/errors to stderr |
| `mkf.(*Makefile).Execute` | `internal/mkf/makefile.go:177` | Help output to stderr (`:189`); orchestrates the target run                                           |
| `mkf.fTgtVersion`         | `internal/mkf/makefile.go:224` | `:version` core target -> stderr (`:226`)                                                             |
| `mkf.fTgtList`            | `internal/mkf/makefile.go:232` | `:list` core target -> stderr (`:234`)                                                                |

## `install` binary (`cmd/install`)

| Function           | File                        | What it prints                     |
|--------------------|-----------------------------|------------------------------------|
| `cmd/install.main` | `cmd/install/install.go:30` | Build/Main error to stderr (`:37`) |

> [!NOTE]
> `install.Main` (`internal/install/main.go:32`) does **not** print anything —
> it returns errors up the stack. The only terminal write on the install path
> is in `cmd/install`'s `main`.

## `go:generate` tools (`//go:build ignore`, not in any shipped binary)

These print to `os.Stderr` directly and only run at development time:

- `internal/parser/main.go` `main` (`:19`)
- `internal/builtin/00_generate_main.go` `main`
- `internal/builtin/builtintest/00_generate_main.go` `main`
- `internal/osarch/00_generate_versions.go` `main` (`:44`, `:78`)

## Deliberately excluded (look like prints, aren't terminal)

- `cli.editGoWork`, `cli.editGoMod`, `cli.compile`, `parser.NewPackage` —
  subprocess `Stdout`/`Stderr` go to a `bytes.Buffer`, captured for error
  messages, never printed.
- `cli/cache.go`, `parser/listcache.go` — `Fprint` into a `hash.Hash`.
- `cli/complete.go:76` — `Fprintf` into a file.
- `mkf.HelpTargets` / `HelpUsage` / `helpCommand` — build strings into buffers;
  the actual terminal write is done by the `cli.Main` / `mkf.*.Execute` callers
  above.
- `gomake.PrettyPrintEnv` (`pkg/gomake/env.go:79`) — writes to a caller-supplied
  `io.Writer`; no shipped caller passes it a terminal writer.
