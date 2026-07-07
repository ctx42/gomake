# Developer notes

## Documentation

The documentation site is a Hugo project under `docs/`.

### Prerequisites

- **Hugo extended** — <https://gohugo.io/installation/> (must be the extended
  variant; the standard build lacks the CSS processing required by Tailwind)
- **Node.js ≥ 18** — <https://nodejs.org/en/download/>

### First-time setup

Install the Node dependencies (provides the Tailwind CSS CLI that Hugo
requires at build time):

```
cd docs && npm install
```

### Start the dev server

```
cd docs && npm run dev
```

The site is served at <http://localhost:1313/> with draft pages included and
fast render disabled so every change triggers a full rebuild.

## Internal architecture

This section is for contributors working on gomake itself. It explains how the
tool works internally — at runtime (a user running `gomake` in their module)
and at install time.

For project conventions, commands, gotchas, and the per-package responsibility
summary, see `AGENTS.md`. For *using* gomake (writing, testing, and importing
targets, the CLI reference, namespaces), see the user docs under `docs/`. This
section does not repeat either.

It names the functions, types, and constants that implement each step — search
for those names rather than relying on line numbers, which drift with every
edit.

### Package map

- `cmd/gomake` — the `gomake` binary entry point.
- `cmd/install` — the installer (`go run .../cmd/install@latest`).
- `internal/cli` — the CLI: configuration, out-of-source builds, makefile
  preparation, and target execution.
- `internal/builtin` — built-in targets, code generation for compiled-in
  external targets, and the embedded main-package templates.
- `internal/parser` — makefile parsing and Go code generation.
- `internal/mkf` — runtime types (`Target`, `Makefile`, helpers) plus the
  embedded source of those types reused in generated makefiles.
- `pkg/gomake` — the module's only public package: env/path helpers and shared
  constants reused by the CLI and by downstream target modules.
- `internal/install` — building the binary and reading version metadata from
  the Go build info.
- `internal/osarch` — generated, version-independent list of supported
  GOOS/GOARCH values used to validate makefile filename suffixes.

### Runtime flow: `gomake <target>` inside a user module

gomake compiles the user's makefile (`makefile.go` plus any OS/arch variants)
into a throwaway binary and runs it. The build happens out-of-source in a temp
directory; the user's tree is never written to.

1. `cmd/gomake/gomake.go` `main` builds the version string, calls
   `builtin.Generated()` to get the built-in target provider, and hands off to
   `cli.Main`.
2. `cli.Main` builds the `config` (`newConfig`), which parses the CLI flags and
   splits out the target name and its arguments. When the `COMP_LINE`
   environment variable is set it serves bash completion (`complete`) for
   built-in targets only — user targets would require a compile, which is too
   slow for the completion path.
3. `builtinTargets` (`internal/cli/core_builtin.go`) returns the provider's
   targets — empty for a stock binary with no compiled-in external targets.
4. Fast path: if the requested target is a built-in (`mkf.FindTarget`),
   `runWithoutCompile` executes it directly, with no makefile parsing or
   compilation.
5. Otherwise `newGoMake` calls `prepare` (`internal/cli/helpers.go`): it creates
   an ephemeral build dir under the temp root, copies the user's valid makefiles
   (see discovery below), adds an empty user-makefile placeholder, and
   copies/edits `go.mod`/`go.work`/`go.sum` so the build resolves the gomake
   module. The result is a `compUnit` describing the build dir and the paths of
   the files to generate and compile.
6. Still in `newGoMake` (`internal/cli/cli.go`): it parses the build-dir
   package (`parser.MakefileFromPackage`) and generates the user-targets
   file — a `targetsUser()` plus `init()` registration — via
   `parser.NewGenerator` and `parser.CreateFile`.
7. `genMain` (`internal/cli/gen_main.go`) generates the glue file, inlining the
   embedded core source (`mkf.MakefileSrc`, `mkf.TargetSrc`, `mkf.HelpersSrc`)
   and a `main()`.
8. `goMake.Execute` checks the binary cache (`lookupBinaryCache`, keyed by
   source dir, version, GOOS, and GOARCH); on a miss it `compile`s (`go build`)
   the whole build dir and caches the result (`storeBinaryCache`). The compiled
   binary then runs as a subprocess in the user's working directory.
9. Inside that binary, the generated `main()` calls `mkf.NewMakefile` and
   `Makefile.Execute` (`internal/mkf/makefile.go`), which pick and run the
   target. The build dir is removed on return (`removeBuildDir`).

The generated and reserved makefile filenames are defined as constants in
`internal/mkf`: `MakefileMain` (`makefile.go`) is the user-written input;
`MakefileGen`, `MakefileUser`, and `MakefileBin` are produced during the build
and must not already exist in the user's source dir.

### Makefile discovery and validation

`prepare` accepts only `makefile.go` and OS/arch variants whose suffix is a
real `GOOS`, `GOARCH`, or `GOOS_GOARCH` token (in that order), mirroring Go's
own filename build constraints. `validMakefileName`/`selectMakefiles`
(`internal/cli/helpers.go`) enforce this via `osarch.IsGOOS`/`osarch.IsGOARCH`;
any other `makefile_*` file (e.g. `makefile_db.go`) is skipped with a warning on
stderr. The same validated file list drives the binary cache key
(`internal/cli/cache.go`), so ignored files never affect caching.

The token set lives in `internal/osarch` as a single, version-independent list
(GOOS/GOARCH values are only added in newer Go releases, so the latest Go's set
is a safe superset). A `go generate` program (`00_generate_versions.go`) runs
`go tool dist list` and writes the unexported `goos`/`goarch` slices into the
generated `versions_gen.go` (plain Go — no JSON, no runtime parsing). By
default it populates from the latest Go reported by `https://go.dev/VERSION`;
set
`GOMAKE_GO_VERSION` (e.g. `1.28`) to pin the version, whose toolchain is
downloaded on demand via `GOTOOLCHAIN`. A guard test fails if the active
toolchain reports a GOOS/GOARCH missing from the list — run
`go generate ./internal/osarch` after a Go upgrade.

> WIP note: `Provider.Source()` (the embedded `data/targets_main.go_` bytes) is
> currently only consumed in tests. The runtime wiring that would emit those
> bytes to compile external targets into the user's generated main is not yet
> exercised — treat compiled-in external targets as an in-progress path.

### Built-in vs external targets, and code generation

`builtin.GenMain` (`internal/builtin`) produces up to three files:

- `internal/builtin/targets.go` — compiled *into* the `builtin` package;
  `targetsBuiltIn()` returns the external targets baked into the binary.
- `internal/builtin/data/targets_main.go_` — a `main`-package template,
  **not** compiled directly; embedded into the `builtin` package via
  `//go:embed` and meant to be emitted into a generated main that registers
  the targets.
- `internal/builtin/data/targets_main_empty.go_` — the same template for the
  no-external-targets case (skipped by the `WithoutGenEmptySrc` option).

The `.go_` suffix keeps Go from compiling the templates as package source.
Because of `//go:embed`, both `data/*.go_` files must *exist* at compile time
or the `builtin` package will not build.

Generation happens at two moments, both via `GenMain`:

- `//go:generate` (the build-tagged generator program under
  `internal/builtin`): the maintainer regenerates from the module-root
  `targets.yaml` and commits the results. This is why the shipped files are the
  *empty* versions.
- Install time: `cli.PrepareTargets` → `regenBuiltins` calls `GenMain` again
  (see the install flow below).

Note: `GenMain` writes `targets.go` and `data/targets_main.go_` unconditionally
(the writes precede the option check); the `targets` and `main` generator
options exist but are not honored today — only `WithoutGenEmptySrc` has an
effect.

### Install flow (`cmd/install`)

Entry: `main` in `cmd/install` parses the `--targets` flag, reads the Go build
info, and calls `install.Main` (`internal/install/main.go`). `install.Main`
resolves the destination from GOBIN via `cli.GoBinPath` and hands off to the
`installTo` indirection (tests call `installTo` with an explicit destination
to avoid touching the real GOBIN).

`installTo` records the version (`version.PopulateVersion`) and picks a mode
from the Go build info:

- **devel** (`go run ./cmd/install`, module version `(devel)`): build from the
  live working directory. The working tree must never be removed — doing so
  was a destructive bug (a deferred cleanup once deleted the `cmd/install`
  sources during a test run).
- **published** (`go run .../cmd/install@version`): the source lives read-only
  in the module cache, so `moduleCacheDir` (`go mod download -json`) locates it.

`effectiveImports` then resolves which external targets to compile in: a
`--targets` file (path or URL) overrides the source `targets.yaml`; a missing
source file yields an empty config. Two paths follow:

- **Fast path — no imports.** `Build` compiles straight from the read-only
  source. Because the shipped builtins already match an empty config and
  `go build` only reads the tree, the temp copy, `go get`, and builtin
  regeneration are all skipped.
- **Full path — imports present.** Build from a writable tree (`copyToTemp` /
  `copyDir` makes a temp copy for a published build, resetting read-only cache
  permissions; a devel build uses the live tree). Write the effective
  `targets.yaml` when `--targets` was supplied, run `cli.PrepareTargets`
  (`go get` per import, then `regenBuiltins`), then `Build`.

`Build` (`internal/install/install.go`) is the only compile step; it writes the
binary to the resolved GOBIN destination. To install elsewhere (e.g. a `dist`
dir), set GOBIN — `GOBIN=$PWD/dist go run ./cmd/install`.

Version metadata: `version.PopulateVersion` reads only the Go toolchain's build
info; any field it cannot supply becomes `version.NotSet`. Installation never
shells out to git, so it works on machines that have only Go.

### Two ways to add external targets

External targets are declared in `targets.yaml` (the filename is the
`cli.TargetsFile` constant; parsing lives in `internal/cli` —
`LoadExternalTargets`, `ImportsConfig`, `ImportEntry`). Each entry has an import
path, an optional namespace, and an optional config block (YAML that is
converted to JSON). The published module ships an empty import list.

- **Case 1 — edit the project-root `targets.yaml`.** Commit the imports in the
  module's own `targets.yaml` and install with no flag. `PrepareTargets` reads
  that file, runs `go get` for each import, and regenerates the builtins.
- **Case 2 — `--targets=<path|URL>`.** `effectiveImports` loads the supplied
  file (`LoadExternalTargets` handles both an HTTP/HTTPS URL and a local path),
  *overriding* the bundled `targets.yaml`; `installTo` then writes it into the
  build dir. The rest of the flow is identical.

Both cases require a **writable** source tree, because `regenBuiltins` rewrites
`internal/builtin/targets.go` and `internal/builtin/data/targets_main.go_`. That
is why the published path copies the read-only module cache before building.
