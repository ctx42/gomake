---
title: "CLI Reference"
description: "All GoMake flags, environment variables, and exit codes."
weight: 7
---

## Usage

```
gomake [flags] [target] [target-arguments...]
```

---

## Flags

| Flag                   | Default        | Description                                                                 |
|------------------------|----------------|-----------------------------------------------------------------------------|
| `--list`               | —              | Print all available targets with their synopsis and exit                    |
| `--help`, `-h`         | —              | Print usage, or print help for a specific target                            |
| `--version`            | —              | Print the gomake version string and exit                                    |
| `--src <dir>`          | `$PWD`         | Directory where GoMake looks for `makefile*.go` files                       |
| `--wd <dir>`           | `$PWD`         | Working directory for the compiled makefile binary                          |
| `--bin <path>`         | —              | Compile the makefile and write the binary to `<path>` instead of running it |
| `--tmp <dir>`          | See below      | Override the temporary build directory                                      |
| `--timeout <duration>` | `0` (no limit) | Target execution timeout (e.g. `30s`, `5m`)                                 |
| `--complete`           | —              | Install bash shell completion                                               |
| `--check-config`       | —              | Validate `gomake.yaml` and list the config key of every target, then exit   |

---

## Environment variables

| Variable         | Description                                                                             |
|------------------|-----------------------------------------------------------------------------------------|
| `GOMAKE_TMP_DIR` | Overrides the default temporary directory. `--tmp` takes precedence over this variable. |

The default temporary directory is `<os-temp>/gomake/` (e.g.
`/tmp/gomake/` on Linux, `%TEMP%\gomake\` on Windows).

---

## Examples

```shell
# Run a target
gomake build

# Run a target with arguments
gomake deploy --env production

# Run a namespace target
gomake ci:lint

# List all targets
gomake --list

# Help for a specific target
gomake --help deploy

# Compile a standalone binary
gomake --bin /usr/local/bin/project-make

# Run targets from a different source directory
gomake --src ./build build

# Set a timeout
gomake --timeout 10m test

# Override the working directory
gomake --wd /var/deploy deploy

# Use a custom temp directory
gomake --tmp /fast/tmp build
```

---

## Flag interactions and constraints

- `--bin` is incompatible with `--help`, `--version`, `--list`, `--timeout`,
  and specifying a target name.
- `--complete` must be the only flag — no other flags or targets are allowed.
- `--tmp` must be an absolute path.
- `--src` is made absolute relative to the current working directory when a
  relative path is given.

---

## Exit codes

| Code    | Meaning                                                        |
|---------|----------------------------------------------------------------|
| `0`     | Success                                                        |
| `1`     | General error (config, I/O, or target failure)                 |
| `125`   | Target exceeded the `--timeout` deadline                       |
| `126`   | No target given and no default target is set (`ErrPickTarget`) |
| `127`   | Unknown target name (`ErrUnkTarget`)                           |
| `129`   | Makefile or generated code failed to compile                   |
| `128+n` | Terminated by fatal signal `n`                                 |

---

## Binary cache

Compiled binaries are cached under `~/.cache/gomake/bin/`. The cache key is a
SHA-256 hash of:

- The makefile sources actually compiled (validated `makefile*.go` names)
- The module's `go.mod` and `go.sum`
- The effective `go.work` / `go.work.sum` (parent walk or `GOWORK`)
- Non-test `.go` files under the module and local `use`/`replace` trees
- The gomake version, `GOOS`, and `GOARCH`
- The Go runtime version and toolchain env (`GOFLAGS`, `CGO_*`,
  `GOTOOLCHAIN`) when set

The cache is invalidated automatically when any input changes. Errors reading
or writing the cache are silently ignored.

---

## Progress messages

GoMake prints progress messages to stderr only when an operation takes longer
than 500 ms:

```
Analyzing sources...   (while running go list / parsing AST)
Compiling makefile...  (while running go build)
```

Fast (cached) invocations are completely silent.

---

## Bash completion

Run `--complete` once to install completion (bash only — it checks `$SHELL`):

```shell
gomake --complete
```

This writes the completion script to `~/.bash_completion.d/gomake` and appends a
`source` line to `~/.bashrc`. To activate it in the current shell without
reopening the terminal:

```shell
source ~/.bashrc
```

Completion covers **built-in targets only**, so no compilation is needed to
complete them. User-defined targets from a project's `makefile.go` are not
completed.
