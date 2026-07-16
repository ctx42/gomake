# GoMake

> Go-native build automation — write targets as functions, not shell scripts.

[![Go Version](https://img.shields.io/badge/go-1.26+-blue)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE.md)

GoMake is a build automation tool for Go projects. Instead of writing Makefiles
or shell scripts, you write ordinary Go functions. GoMake discovers them as
runnable targets — complete with documentation, typed arguments, namespaces,
and cross-package imports.

```go
// makefile.go
package main

import (
    "context"
    "os/exec"

    "github.com/ctx42/ring/pkg/ring"
)

// Build compiles the project for the current platform.
func Build(ctx context.Context, rng *ring.Ring) error {
    cmd := exec.CommandContext(ctx, "go", "build", "./...")
    cmd.Stdout = rng.Stdout()
    cmd.Stderr = rng.Stderr()
    return cmd.Run()
}

// Test runs the test suite.
func Test(ctx context.Context, rng *ring.Ring) error {
    cmd := exec.CommandContext(ctx, "go", "test", "./...")
    cmd.Stdout = rng.Stdout()
    cmd.Stderr = rng.Stderr()
    return cmd.Run()
}
```

```shell
$ gomake build
$ gomake test
$ gomake --list
build     compiles the project for the current platform
test      runs the test suite
```

---

## Contents

- [Why GoMake?](#why-gomake)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Writing targets](#writing-targets)
  - [Target signature](#target-signature)
  - [Documentation and synopsis](#documentation-and-synopsis)
  - [Default target](#default-target)
  - [Hidden targets](#hidden-targets)
  - [Build tags](#build-tags)
- [Namespaces](#namespaces)
- [Importing targets from other packages](#importing-targets-from-other-packages)
- [Builtin targets (targets.yaml)](#builtin-targets-targetsyaml)
- [Testing targets](#testing-targets)
  - [The testing advantage](#the-testing-advantage)
  - [Capturing output](#capturing-output)
  - [Injecting arguments](#injecting-arguments)
  - [Injecting environment variables](#injecting-environment-variables)
  - [Using ringtest.Tester](#using-ringtesttester)
  - [Injecting metadata](#injecting-metadata)
  - [Injecting a filesystem](#injecting-a-filesystem)
  - [Table-driven tests](#table-driven-tests)
  - [Testing error paths](#testing-error-paths)
- [CLI reference](#cli-reference)
- [Configuration](#configuration)
- [Binary cache](#binary-cache)
- [Exit codes](#exit-codes)
- [Bash completion](#bash-completion)
- [Target author toolkit](#target-author-toolkit)
- [At a glance](#at-a-glance)

---

## Why GoMake?

Three things set it apart — the pillars everything else builds on:

1. **Build automation written in Go.** Targets are ordinary Go functions —
   same language, same IDE, same `go vet`. No DSL, no YAML, no shell quoting.
2. **Shared across projects and people.** Publish your targets once; every
   repo and every teammate pulls them in with a single `//gomake:import`, and
   `go get -u` ships a fix to all of them.
3. **Fully testable.** Targets get their I/O, arguments, and environment
   through `ring.Ring`, so you call them directly in tests and assert the
   output — no subprocess, no global state.

| Pain point with traditional tools  | GoMake's answer                                           |
|------------------------------------|-----------------------------------------------------------|
| Makefile syntax is unrelated to Go | Targets are Go functions — same language, same IDE        |
| Shell scripts break on Windows     | Go code is cross-platform                                 |
| No type checking, no IDE support   | Full Go tooling: refactoring, completion, `go vet`        |
| Shared targets require copy-paste  | Import targets from any Go package with `//gomake:import` |
| Hard to document                   | Go doc comments become `--help` output automatically      |
| Slow on every invocation           | Compiled binary is cached; repeated runs are instant      |

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

After installation, verify:

```shell
gomake --version
# gomake v0.12.1, hash: abc1234, build date: 2026-05-09T12:00:00Z, scm state: clean
```

### With custom built-in targets

Pass a `targets.yaml` to compile external target packages permanently into the
binary:

```shell
go run github.com/ctx42/gomake/cmd/install@latest --targets=./targets.yaml
go run github.com/ctx42/gomake/cmd/install@latest --targets=https://example.com/targets.yaml
```

Or forward `--targets` through the install script with `sh -s --`:

```shell
curl -fsSL https://raw.githubusercontent.com/ctx42/gomake/master/install.sh | sh -s -- --targets=./targets.yaml
curl -fsSL https://raw.githubusercontent.com/ctx42/gomake/master/install.sh | sh -s -- --targets=https://example.com/targets.yaml
```

### Embedding a CI/CD tag

```shell
GOMAKE_CCID=build-42 go run github.com/ctx42/gomake/cmd/install@latest
gomake --version
# gomake v0.12.1, hash: abc1234, build date: 2026-05-09T12:00:00Z, scm state: clean, cc tag: build-42
```

### Installing to a custom directory

The binary is installed to GOBIN (or `$GOPATH/bin` when GOBIN is unset). Point
GOBIN wherever you want it to land — for example, a project-local `dist/`:

```shell
GOBIN=$PWD/dist go run github.com/ctx42/gomake/cmd/install@latest
```

### Installing from a local clone

To build from a checked-out copy of the source instead of a published
release — for instance to install unreleased changes on `master` — run the
installer from the clone with `go run ./cmd/install`. It builds whatever is in
your working tree; no tag, release, or module proxy is involved.

```shell
go run -buildvcs=true ./cmd/install
```

`--targets` and the other options work exactly as above:

```shell
go run -buildvcs=true ./cmd/install --targets=./targets.yaml
```

> [!IMPORTANT]
> Pass `-buildvcs=true`. Under the default `-buildvcs=auto`, `go run` skips
> version-control stamping, so the binary reports `<not set>` for its revision,
> hash, and clean/dirty state. Forcing it on records them from the checked-out
> commit:
>
> ```shell
> gomake --version
> # gomake v0.24.0, hash: ec7c8f1, build date: 2026-07-15T...Z, scm state: clean
> ```

> [!NOTE]
> Use the package path `./cmd/install`, not a file path like
> `cmd/install/install.go`. Building from an explicit file leaves the build
> metadata empty, and the installer then mistakes the run for a published
> install and fails.

---

## Quick start

1. Create `makefile.go` in your project's root (or any directory):

```go
//go:build gomake

package main

import (
    "context"
    "os/exec"

    "github.com/ctx42/ring/pkg/ring"
)

// Build compiles the project.
func Build(ctx context.Context, rng *ring.Ring) error {
    return exec.CommandContext(ctx, "go", "build", "./...").Run()
}
```

2. Run a target:

```shell
gomake build
```

3. List all targets:

```shell
gomake --list
```

4. Get help for a specific target:

```shell
gomake --help build
```

GoMake compiles your `makefile.go` on the first run and caches the binary.
Subsequent runs with unchanged sources are instant.

---

## Writing targets

### Target signature

Every target is a Go function with this exact signature:

```go
func TargetName(ctx context.Context, rng *ring.Ring) error
```

- `ctx context.Context` — a context that is cancelled when the process
  receives a signal. Pass it to all long-running operations.
- `rng *ring.Ring` — the execution environment: stdout/stderr, environment
  variables, and the argument list.
- Return `nil` on success, any `error` on failure.

Only the parameter types matter — the `ctx` and `rng` names are yours. Don't
alias the `ring` import in a makefile, though, or the function won't be
recognised as a target.

The `ring.Ring` type provides:

```go
rng.Stdout()        // io.Writer — write target output here
rng.Stderr()        // io.Writer — write errors and logs here
rng.Args()          // []string — arguments after the target name
rng.EnvGet("KEY")   // string   — read an environment variable
```

**Accessing target arguments:**

```go
func Greet(ctx context.Context, rng *ring.Ring) error {
    name := "World"
    if args := rng.Args(); len(args) > 0 {
        name = args[0]
    }
    fmt.Fprintf(rng.Stdout(), "Hello, %s!\n", name)
    return nil
}
```

```shell
gomake greet Alice
# Hello, Alice!
```

**Using flags inside a target:**

```go
func Deploy(ctx context.Context, rng *ring.Ring) error {
    fs := flag.NewFlagSet("deploy", flag.ContinueOnError)
    env := fs.String("env", "staging", "deployment environment")
    if err := fs.Parse(rng.Args()); err != nil {
        return err
    }
    fmt.Fprintf(rng.Stdout(), "Deploying to %s\n", *env)
    return nil
}
```

```shell
gomake deploy --env production
```

### Documentation and synopsis

The first sentence of a target's Go doc comment becomes its synopsis in
`--list` and `--help` output. The full comment is shown with
`--help <target>`.

```go
// Build compiles the project for the current platform. It respects the
// GOOS and GOARCH environment variables for cross-compilation and writes
// the binary to dist/.
func Build(ctx context.Context, rng *ring.Ring) error { ... }
```

```shell
$ gomake --list
build   compiles the project for the current platform

$ gomake --help build
Usage:
  build

Description:
  Build compiles the project for the current platform. It respects the
  GOOS and GOARCH environment variables for cross-compilation and writes
  the binary to dist/.
```

### Default target

Declare `var Default = <TargetFn>` to set the target that runs when no target
name is given:

```go
var Default = Build

func Build(ctx context.Context, rng *ring.Ring) error { ... }
```

```shell
gomake        # runs Build
gomake build  # same
```

The default can also point to a namespace target:

```go
var Default = CI.All
```

### Hidden targets

Add a `// gomake:hidden` line anywhere in a target's doc comment to hide it
from `--list` output. The target still runs normally by name.

```go
// InternalSetup is a helper used by other targets.
//
// gomake:hidden not meant to be run directly
func InternalSetup(ctx context.Context, rng *ring.Ring) error { ... }
```

Note the space after `//`: `// gomake:hidden` works, but `//gomake:hidden` (no
space) is treated as a Go compiler directive and stripped before GoMake sees
it. The target must also be exported — an unexported function is not a target.

### Build tags

To keep makefile code out of your regular build, add the `//go:build gomake`
build tag at the top of the file:

```go
//go:build gomake

package main

import (
    "context"
    "github.com/ctx42/ring/pkg/ring"
)

func Build(ctx context.Context, rng *ring.Ring) error { ... }
```

GoMake automatically strips this tag before compiling, so the file compiles
normally in the build directory. Files without the tag work too — use
whichever style fits your project.

**Naming conventions** — your targets live in `makefile.go`. Additional files
are only allowed for OS/arch-specific targets, using Go's standard filename
build constraints:

```text
makefile.go               # main targets (required)
makefile_linux.go         # included only on GOOS=linux
makefile_amd64.go         # included only on GOARCH=amd64
makefile_linux_amd64.go   # GOOS=linux AND GOARCH=amd64
```

The suffix after `makefile_` must be a valid `GOOS`, `GOARCH`, or
`GOOS_GOARCH` (in that order). Files with any other suffix (e.g.
`makefile_docker.go`) are ignored with a warning — keep all non-platform
targets in `makefile.go`.

---

## Namespaces

Group related targets under a namespace by defining a struct type tagged with
`//gomake:ns_root` and attaching target methods to it:

```go
// Docker groups container build and push targets.
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
```

Namespace names are converted to `kebab-case` automatically:
`DockerRelease` → `docker-release`.

### Nested namespaces

Namespaces can be nested by having one type alias another:

```go
type CI struct{}  //gomake:ns_root

func (CI) Lint(ctx context.Context, rng *ring.Ring) error { ... }

type Docker CI

func (Docker) Build(ctx context.Context, rng *ring.Ring) error { ... }
```

```shell
$ gomake --list
ci:lint          ...
ci:docker:build  ...
```

Each segment is kebab-cased from its own type name, so the child type is named
`Docker` (giving `ci:docker`), not `CIDocker` (which would give `ci:ci-docker`).

### Default target in a namespace

Declare `var Default` pointing to a namespace method:

```go
var Default = CI.Lint
```

---

## Importing targets from other packages

Build logic doesn't have to live in one repo. Write a target once, publish it
in a shared package, and every project pulls it in with a single
`//gomake:import` comment. Fix a bug once and `go get -u` propagates it to every
repo and every teammate — no copy-paste, no drift.

**Author once — a shared package.** A shareable package is a regular Go package
whose functions match the target signature. It does not need `package main`:

```go
// Package lint holds the org's standard linting targets,
// shared across every repo.
package lint

import (
    "context"
    "os/exec"

    "github.com/ctx42/ring/pkg/ring"
)

// Lint runs the shared golangci-lint configuration.
func Lint(ctx context.Context, rng *ring.Ring) error {
    cmd := exec.CommandContext(ctx, "golangci-lint", "run")
    cmd.Stdout = rng.Stdout()
    cmd.Stderr = rng.Stderr()
    return cmd.Run()
}
```

**Import anywhere — one comment.** Pull the package into any `makefile.go` with
`//gomake:import`. The blank identifier (`_`) is required: the package is never
referenced in your code, so a named import would fail to compile.

```go
//go:build gomake

package main

import (
    _ "acme.dev/mk/lint"    //gomake:import
    _ "acme.dev/mk/release" //gomake:import release
)
```

The first import merges its targets into the root namespace. The second places
them under the `release:` prefix — the word after `//gomake:import`.

```shell
$ gomake --list
build            compiles the project                     # local
lint             runs the shared golangci-lint config     # imported
release:tag      tags the next release                    # imported
release:publish  publishes the build artifacts            # imported
```

### Import resolution

GoMake resolves `//gomake:import` packages by running `go list` in the project
directory, so an imported package must be in the project's module graph —
present in `go.mod` / `go.sum`. Add it with `go get acme.dev/mk/lint`. Multiple
imports resolve concurrently, and an error in any one aborts the build.
Packages inside the current module are resolved in-process, and external,
versioned modules are cached on disk, so repeat runs stay fast.

The namespace argument is lowercased regardless of how it is written
(`//gomake:import DB` → `db:`), and a space after `//` is tolerated
(`// gomake:import`).

---

## Builtin targets (targets.yaml)

`targets.yaml` at the gomake source root compiles external Go packages
permanently into the gomake binary at install time. These become
**built-in** targets — available in every project, with no `makefile.go`
required.

This differs from `//gomake:import`, which is declared per-project inside a
`makefile.go`:

| Mechanism           | Where declared          | Available in                |
|---------------------|-------------------------|-----------------------------|
| `targets.yaml`      | gomake repo root        | Every project, always       |
| `//gomake:import`   | project's `makefile.go` | That project only           |

### Schema

```yaml
imports:
  - import: example.com/myorg/targets/docker
  - import: example.com/myorg/targets/release
    namespace: release
  - import: example.com/myorg/targets/db
    namespace: db
    config:
      host: db.internal
      port: 5432
```

| Field       | Required | Description                                                   |
|-------------|----------|---------------------------------------------------------------|
| `import`    | Yes      | Go import path of the target package                          |
| `namespace` | No       | CLI prefix for all targets from this package (e.g. `release`) |
| `config`    | No       | Arbitrary YAML passed to targets via Ring metadata at runtime |

### How it works

Running `go run github.com/ctx42/gomake/cmd/install@latest --targets=./targets.yaml`:

1. Reads `targets.yaml`.
2. Runs `go get <import-path>` for each entry.
3. Parses each package for target functions and generates
   `internal/builtin/targets.go`.
4. Compiles the generated file into the binary.

Each entry is announced during install:

```text
adding external target example.com/myorg/targets/docker
adding external target example.com/myorg/targets/release
```

To regenerate `internal/builtin/targets.go` without a full reinstall:

```shell
go generate ./internal/builtin/
```

### The config field

A target reads `config` from `ring.Ring` metadata as a JSON string. The key is
the `namespace` value, or — when `namespace` is absent — the last segment of the
import path. Targets read it via `rng.MetaGet`:

```go
func Migrate(ctx context.Context, rng *ring.Ring) error {
    raw := rng.MetaGet("db") // key == "db" (the namespace)
    // raw is the config JSON as a string; unmarshal as needed
    _ = raw
    return nil
}
```

Note *where* `config` is read: only target code is compiled into the binary,
not the `config` values. At runtime GoMake loads `config` from a `targets.yaml`
in the project's source directory (the `--src` directory, default: cwd). So to
feed config to a built-in target, place a `targets.yaml` with the `config` block
in that project; the gomake repo's install-time `config` is not embedded.

The `import` path may also pin a version (e.g. `.../release@v1.3.0`), and the
file is parsed strictly — an unknown field fails the load.

---

## Testing targets

### The testing advantage

Because every target receives its I/O, environment, and arguments through
`ring.Ring` rather than reading from global state (`os.Stdout`, `os.Getenv`,
`os.Args`), targets are functions in the truest sense: given the same `Ring`,
they produce the same output. No monkey-patching, no subprocess execution, no
temporary files — just a function call in a test.

```go
// The target under test.
func Greet(ctx context.Context, rng *ring.Ring) error {
    name := rng.EnvGet("GREETER_NAME")
    if name == "" {
        name = "World"
    }
    if args := rng.Args(); len(args) > 0 {
        name = args[0]
    }
    out := rng.Stdout()
    _, err := fmt.Fprintf(out, "Hello, %s!\n", name)
    return err
}
```

```go
// A complete, hermetic test — no I/O, no env, no OS interaction.
func TestGreet(t *testing.T) {
    // --- Given ---
    var out bytes.Buffer
    rng := ring.New(ring.WithArgs([]string{"Alice"}))
    rng.SetStdout(&out)

    // --- When ---
    err := Greet(t.Context(), rng)

    // --- Then ---
    assert.NoError(t, err)
    assert.Equal(t, "Hello, Alice!\n", out.String())
}
```

### Capturing output

Redirect stdout and stderr to `bytes.Buffer` to inspect what a target writes:

```go
func TestBuild_OutputsFilename(t *testing.T) {
    // --- Given ---
    var stdout, stderr bytes.Buffer
    rng := ring.New()
    rng.SetStdout(&stdout)
    rng.SetStderr(&stderr)

    // --- When ---
    err := Build(t.Context(), rng)

    // --- Then ---
    assert.NoError(t, err)
    assert.Contains(t, stdout.String(), "dist/myapp")
    assert.Empty(t, stderr.String())
}
```

Targets that correctly route all output through `rng.Stdout()` / `rng.Stderr()`
make this trivially easy. Targets that write directly to `os.Stdout` cannot be
tested this way — which is a good reason to always use the Ring.

### Injecting arguments

Pass arguments exactly as they would arrive from the command line via
`ring.WithArgs`:

```go
func TestDeploy_EnvFlag(t *testing.T) {
    // --- Given ---
    var out bytes.Buffer
    rng := ring.New(ring.WithArgs([]string{"--env", "production"}))
    rng.SetStdout(&out)

    // --- When ---
    err := Deploy(t.Context(), rng)

    // --- Then ---
    assert.NoError(t, err)
    assert.Contains(t, out.String(), "production")
}
```

### Injecting environment variables

Provide a controlled environment with `ring.WithEnv`. Only the variables you
specify exist — no leakage from the test runner's process environment:

```go
func TestGreet_FromEnv(t *testing.T) {
    // --- Given ---
    var out bytes.Buffer
    rng := ring.New(ring.WithEnv([]string{"GREETER_NAME=Bob"}))
    rng.SetStdout(&out)

    // --- When ---
    err := Greet(t.Context(), rng)

    // --- Then ---
    assert.NoError(t, err)
    assert.Equal(t, "Hello, Bob!\n", out.String())
}
```

Add or update individual variables on an existing Ring with `EnvSet`:

```go
rng := ring.New()
rng.EnvSet("DATABASE_URL", "postgres://localhost/testdb")
rng.EnvSet("DEBUG", "true")
```

### Using ringtest.Tester

The `ringtest` package provides `Tester`, a thin helper that wires up
test-friendly I/O buffers and catches unexpected writes:

```go
import "github.com/ctx42/ring/pkg/ring/ringtest"

func TestBuild(t *testing.T) {
    // --- Given ---
    tst := ringtest.New(t)
    tst.WetStdout()

    // --- When ---
    err := Build(t.Context(), tst.Ring())

    // --- Then ---
    assert.NoError(t, err)
    assert.Contains(t, tst.Stdout(), "compiled")
}
```

Key `Tester` methods:

| Method                     | Description                                                                                  |
|----------------------------|----------------------------------------------------------------------------------------------|
| `tst.Ring(args ...string)` | Returns a `*ring.Ring` wired to the test buffers, with `args` as `rng.Args()`                |
| `tst.WetStdout()`          | Mark stdout as expected to receive writes (dry by default — unexpected writes fail the test) |
| `tst.WetStderr()`          | Mark stderr as expected to receive writes                                                    |
| `tst.Stdout()`             | Return everything written to stdout so far                                                   |
| `tst.Stderr()`             | Return everything written to stderr so far                                                   |
| `tst.ResetStdout()`        | Clear the stdout buffer between sub-tests                                                    |
| `tst.ResetStderr()`        | Clear the stderr buffer between sub-tests                                                    |
| `tst.SetStdin(buf)`        | Provide content for targets that read from stdin                                             |

The "dry buffer" default is the key insight: if a target writes to stderr when
it shouldn't (e.g. on a successful path), the test fails (reported via
`t.Error` at test cleanup) — no `assert.Empty(t, stderr)` needed.

### Injecting metadata

Targets can read typed metadata from the Ring (e.g. configuration structs
passed by the runner). Inject it in tests with `ring.WithMeta` or `MetaSet`:

```go
type DeployConfig struct {
    Registry string
    ImageTag string
}

func Deploy(ctx context.Context, rng *ring.Ring) error {
    raw := rng.MetaGet("deploy.config")
    cfg, ok := raw.(DeployConfig)
    if !ok {
        return errors.New("deploy: missing config")
    }
    fmt.Fprintf(rng.Stdout(), "pushing %s/%s\n", cfg.Registry, cfg.ImageTag)
    return nil
}
```

```go
func TestDeploy_WithConfig(t *testing.T) {
    // --- Given ---
    var out bytes.Buffer
    rng := ring.New()
    rng.SetStdout(&out)
    rng.MetaSet("deploy.config", DeployConfig{
        Registry: "registry.example.com",
        ImageTag: "v1.2.3",
    })

    // --- When ---
    err := Deploy(t.Context(), rng)

    // --- Then ---
    assert.NoError(t, err)
    assert.Equal(t, "pushing registry.example.com/v1.2.3\n", out.String())
}
```

### Injecting a filesystem

Targets that read files can accept an `fs.FS` through the Ring instead of
calling `os.Open` directly. This makes them testable without touching the disk:

```go
func ReadConfig(ctx context.Context, rng *ring.Ring) error {
    fsys, err := rng.FS()
    if err != nil {
        return err
    }
    data, err := fs.ReadFile(fsys, "config.yaml")
    if err != nil {
        return err
    }
    fmt.Fprintf(rng.Stdout(), "loaded %d bytes\n", len(data))
    return nil
}
```

```go
func TestReadConfig(t *testing.T) {
    // --- Given ---
    fsys := fstest.MapFS{
        "config.yaml": {Data: []byte("key: value\n")},
    }
    var out bytes.Buffer
    rng := ring.New(ring.WithFS(fsys))
    rng.SetStdout(&out)

    // --- When ---
    err := ReadConfig(t.Context(), rng)

    // --- Then ---
    assert.NoError(t, err)
    assert.Equal(t, "loaded 11 bytes\n", out.String())
}
```

### Table-driven tests

`ring.Clone()` creates an independent copy of a Ring — same environment and
metadata, separate I/O buffers — which is ideal for table-driven tests:

```go
func TestGreet_Cases(t *testing.T) {
    cases := []struct {
        name    string
        args    []string
        env     []string
        wantOut string
    }{
        {
            name:    "arg wins over env",
            args:    []string{"Alice"},
            env:     []string{"GREETER_NAME=Bob"},
            wantOut: "Hello, Alice!\n",
        },
        {
            name:    "env used when no arg",
            env:     []string{"GREETER_NAME=Bob"},
            wantOut: "Hello, Bob!\n",
        },
        {
            name:    "default fallback",
            wantOut: "Hello, World!\n",
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            // --- Given ---
            var out bytes.Buffer
            rng := ring.New(
                ring.WithArgs(tc.args),
                ring.WithEnv(tc.env),
            )
            rng.SetStdout(&out)

            // --- When ---
            err := Greet(t.Context(), rng)

            // --- Then ---
            assert.NoError(t, err)
            assert.Equal(t, tc.wantOut, out.String())
        })
    }
}
```

### Testing error paths

Return a non-nil error from a target to verify that callers handle it:

```go
func TestDeploy_MissingConfig(t *testing.T) {
    // --- Given ---
    tst := ringtest.New(t)
    // No WetStdout() — success output must not appear.
    // No WetStderr() — error should come from the return value, not stderr.

    // --- When ---
    err := Deploy(t.Context(), tst.Ring())

    // --- Then ---
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "missing config")
    assert.Empty(t, tst.Stdout())
}
```

Test targets that call `os/exec` commands by injecting a fake command via
environment or by extracting the command name into a variable the test can
override — or simply by pointing the target at a test binary using
`GOFLAGS` and `exec.LookPath`. Targets with pure Go logic (no subprocesses)
are testable with no extra effort.

---

## CLI reference

```text
Usage: gomake [flags] [target] [target-args...]
```

| Flag                   | Description                                                                 |
|------------------------|-----------------------------------------------------------------------------|
| `--list`               | List all available targets with their synopsis                              |
| `--help`, `-h`         | Show usage, or show help for a specific target                              |
| `--version`            | Print the gomake version                                                    |
| `--src <dir>`          | Directory to look for `makefile*.go` files (default: cwd)                   |
| `--wd <dir>`           | Working directory for the compiled makefile binary (default: cwd)           |
| `--bin <path>`         | Compile the makefile and write the binary to `<path>` instead of running it |
| `--tmp <dir>`          | Override the temporary build directory                                      |
| `--timeout <duration>` | Target execution timeout (e.g. `30s`, `5m`). Default: no limit              |
| `--complete`           | Install bash shell completion (exclusive with all other flags)              |

### Environment variables

| Variable         | Description                                                          |
|------------------|----------------------------------------------------------------------|
| `GOMAKE_TMP_DIR` | Overrides the default temporary directory (`--tmp` takes precedence) |

### Examples

```shell
# Run a target
gomake build

# Run with arguments
gomake deploy --env production

# List all targets
gomake --list

# Help for a target
gomake --help deploy

# Compile binary without running
gomake --bin /usr/local/bin/project-make

# Run targets from a different source directory
gomake --src /path/to/project build

# Set a timeout
gomake --timeout 10m test
```

---

## Configuration

GoMake reads `makefile*.go` files from the current directory by default.
Point it to a different directory with `--src`:

```shell
gomake --src ./build/targets test
```

The compiled binary runs in the current directory by default. Override with
`--wd`:

```shell
gomake --wd /var/deploy deploy
```

### Temporary directory

The intermediate build directory lives inside a temp directory. Defaults to
`<os-temp>/gomake/`. Override with:

```shell
export GOMAKE_TMP_DIR=/fast/nvme/tmp
# or
gomake --tmp /fast/nvme/tmp build
```

### Compiling a standalone binary

Use `--bin` to produce a self-contained binary that embeds all your targets.
This is useful for CI or Docker images that should not require the gomake
binary:

```shell
gomake --bin ./dist/project-make
./dist/project-make build
./dist/project-make --list
```

---

## Binary cache

GoMake caches compiled makefile binaries under `~/.cache/gomake/bin/`. The
cache key is a SHA-256 hash of:

- all `makefile*.go` file contents (sorted by name)
- the project's `go.sum` file
- the gomake version
- `GOOS` and `GOARCH`

When the cache key matches an existing binary, compilation is skipped entirely.
The cache is invalidated automatically whenever any of the above inputs change.

Progress messages (`Analyzing sources...`, `Compiling makefile...`) are shown
on stderr only when an operation takes longer than 500 ms, keeping fast
(cached) invocations silent.

---

## Exit codes

| Code    | Constant        | Meaning                                      |
|---------|-----------------|----------------------------------------------|
| `0`     | —               | Success                                      |
| `1`     | —               | General error                                |
| `125`   | —               | Target exceeded the `--timeout` deadline     |
| `126`   | `ErrPickTarget` | No target given and no default target is set |
| `127`   | `ErrUnkTarget`  | Unknown target name                          |
| `129`   | —               | Makefile or generated code failed to compile |
| `128+n` | —               | Terminated by fatal signal `n`               |

---

## Bash completion

Run `--complete` once to install completion (bash only — it checks `$SHELL`):

```shell
gomake --complete
```

This writes the completion script to `~/.bash_completion.d/gomake` and appends a
`source` line to `~/.bashrc`. Activate it in the current shell with:

```shell
source ~/.bashrc
```

Completion covers built-in targets only, so no compilation is needed. User
targets from a project's `makefile.go` are not completed.

---

## Target author toolkit

GoMake is a tool, not a library — its internals live in `internal/` packages
that other modules cannot import. The one public package, `pkg/gomake`, is a
small set of helpers for the code you write *as* targets (a project's
`makefile.go`, or external and built-in target packages):

```go
import "github.com/ctx42/gomake/pkg/gomake"
```

It provides the running target's name (`TargetName`), the module root (`Root`),
interactive input (`ReadLine`, `ReadChar`), and small filesystem and
environment helpers used inside a target:

```go
func Info(ctx context.Context, rng *ring.Ring) error {
    name, _ := gomake.TargetName(ctx)
    goos := gomake.GetGOOS(rng.EnvAll())
    fmt.Fprintf(rng.Stdout(), "target=%s goos=%s\n", name, goos)
    return nil
}
```

**Browse the full API:**

```shell
go doc github.com/ctx42/gomake/pkg/gomake
```

See the [Target Author Toolkit](docs/content/docs/library.md) docs for the full
list of helpers.

---

## At a glance

Everything GoMake provides out of the box — one signature, plain Go, and a
compiled-and-cached binary:

| Capability            | How GoMake does it                              |
|-----------------------|-------------------------------------------------|
| Target signature      | `func(ctx, *ring.Ring) error` — one form        |
| I/O access            | `rng.Stdout()` / `rng.Stderr()` — injectable     |
| Arguments             | `rng.Args()` — the full `[]string`              |
| Environment           | `rng.EnvGet` — isolated and seedable in tests   |
| Testing               | Call the target directly with a controlled Ring |
| Namespaces            | Methods on `//gomake:ns_root` structs, nestable |
| Cross-package imports | `//gomake:import` comment tag                   |
| Built-in targets      | `targets.yaml` — compiled into the binary       |
| Binary cache          | SHA-256 of sources + `go.sum` + version + arch  |
| Progress feedback     | Automatic, after 500 ms                         |
| Cross-platform        | Go build tags + `GOOS`/`GOARCH`; `go.work` too  |
| Standalone binary     | `gomake --bin ./make`                           |

**GoMake fits when:**

- You want structured I/O that is easy to test and redirect
- You share targets across multiple repositories via `//gomake:import`
- You need namespaced target hierarchies that map cleanly to CLI names
- You prefer keeping I/O and environment isolated from global state

---

Bug reports and patches are welcome via the issue tracker; new functionality
must include tests (`go test ./...`).
