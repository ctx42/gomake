# Agent Instructions for gomake

This file provides guidance for AI agents (and human contributors) working
on the gomake codebase. It captures project-specific conventions, gotchas,
and workflows.

## Project Overview

gomake is a from-scratch implementation of a make-like build tool using Go
(similar in spirit to Mage). Write ordinary Go functions following a simple
signature and gomake discovers them as runnable targets (with docs, args,
namespaces, cross-package imports, build tags, etc.).

- Target signature: `func(ctx context.Context, rng *ring.Ring) error`
- Makefiles: `makefile.go` (required), plus OS/arch variants only —
  `makefile_<GOOS>.go` / `makefile_<GOARCH>.go` / `makefile_<GOOS>_<GOARCH>.go`
  (other suffixes are ignored with a warning). Optionally tagged
  `//go:build gomake`
- Namespaces: `type Foo struct{} //gomake:ns_root`
- Imports: `import foo "example.com/bar" //gomake:import [ns]`
- Hidden: `// gomake:hidden reason`

See README.md, internal/parser/parser.go (tags), and especially
testdata/projects/* for realistic examples.

## Essential Commands

```shell
# Build gomake binary (project-local; do not leave ./gomake in repo root)
go build -o dist/gomake ./cmd/gomake

# Full install to GOBIN (resolves external targets + regenerates builtins).
# Set GOBIN to install elsewhere, e.g. GOBIN=$PWD/dist go run ./cmd/install
go run ./cmd/install

# Build and vet packages (libraries only)
go build ./...
go vet ./...

# Test (full suite takes ~15-25s because pkg/gomake tests perform real
# compilation and execution of generated makefiles)
go test ./...

# Lint (requires golangci-lint in PATH)
golangci-lint run ./...

# Regenerate built-in target glue (parser + builtin packages use go:generate)
go generate ./internal/parser
go generate ./internal/builtin

# Regenerate the supported GOOS/GOARCH table (run after a Go upgrade)
go generate ./internal/osarch
```

Run targeted tests when iterating:
```shell
go test ./internal/parser -run TestFoo
go test ./pkg/gomake -run 'Test_(Path|File)'
```

## Code Style and Conventions

- **Editor**: See .editorconfig. Go files use **tabs**. Markdown files have
  `max_line_length = 80`.
- **Testing**: Use `github.com/ctx42/testing/pkg/assert` (assert.True,
  assert.NoError, assert.ErrorIs, assert.Equal, etc.). Tests are table-driven
  where practical and use golden files (*.gld) under testdata/ for parser
  output.

### Test Function Naming

Each (exported or unexported) function or method must have a corresponding
test:

- Package-level functions: `Test_<FunctionName>`
- Methods on a type: `Test_<Type>_<Method>`

When using table-driven style, append `_tabular`, or split success and error
cases:

- `Test_goBinPath_success_tabular(t *testing.T)`
- `Test_goBinPath_error_tabular(t *testing.T)`
- `Test_Equal_tabular(t *testing.T)`

See also the rules in `docs` from ctx42/testing for subtest name safety
(avoid `()*#$?<>|:,;` etc in t.Run names; prefer alphanum, space, `-`, `_`,
`/`). Use `// --- Given ---`, `// --- When ---`, `// --- Then ---` inside
subtests.

When testing assertion helpers themselves, use `tester.Spy` instead of raw
`*testing.T`.
- **Errors**: Define exported sentinel vars (`var ErrX = errors.New(...)`).
  Wrap with `%w`. Use `errors.Is` / `errors.As`. See
  internal/mkf/helpers.go:ExitCode plus the `ErrPickTarget`/`ErrUnkTarget`
  sentinels in internal/mkf and `gomake.ErrNoGoMod` in pkg/gomake for patterns.
  Custom error types implement `Unwrap()` and `Error()`.
- **Context & I/O**: Context is always the first parameter on public or
  potentially blocking functions. Never use `os.Stdout` etc. directly in
  library code; thread `*ring.Ring` (provides Env, Stdout/Stderr, Meta, Args).
- **Subprocess env**: When building an `exec.Command` / `exec.CommandContext`
  in the runtime call graph, set `cmd.Env = env.EnvAll()` from the injected
  `ring.Environ` (thread one in as a `ring.Environ` parameter if the function
  lacks it) so a single environment stays authoritative and tests can seed it
  through a Ring. Not required for the `//go:generate` helper in
  `internal/osarch` or for test-only scaffolding. `cmd.Env` governs only the
  child's environment; the `go` binary itself is still resolved on the process
  PATH.
- **Generation & Embeds**: Parser and builtin packages embed templates and
  generate code (see //go:generate and //go:embed). Changes to core templates
  (makefile.go, target.go, helpers.go in pkg/gomake) or generator logic usually
  require regenerating testdata golden files.
- **Build isolation**: gomake uses out-of-source builds under a temp dir
  (GOMAKE_TMP_DIR or system tmp + random subdir). The dir is cleaned on
  success or error paths in most cases.
- **Versioning**: ldflags + Go build info during install (no git). See
  internal/version and internal/install.

## Common Gotchas

- Symlinks in testdata must be real symlinks (git stores mode 120000).
  `pkg/gomake/testdata/file0_link.txt -> file0.txt` is required for sys tests.
  If tests fail with "no such file", restore with `git checkout -- ...`.
- `go mod tidy` will currently upgrade ctx42/ring and ctx42/testing beyond the
  committed pins (go.mod on disk drifts from HEAD). Be aware when diffing.
- `BuildTag` is defined in internal/parser and re-exported by internal/cli
  (`const BuildTag = parser.BuildTag`), so the two cannot drift; `BuildTagLine`
  lives only in internal/parser.
- Several `//nolint:cyclop,gocognit` on large orchestration functions
  (`prepare`, `Main`, etc.). Refactor carefully.
- Context keys: use typed keys. The `targetNameKey` string is deliberately
  duplicated across the inline boundary — `pkg/gomake/targetname.go`
  (`gomake.TargetName`) and `internal/mkf/makefile.go` (`Makefile.Execute`) —
  and both must use the same value.
- Temp dir leaks are possible on certain error paths in `prepare`; review any
  changes to build-dir creation/defer logic.
- `internal/install` assumes `go` is in PATH and behaves reasonably (install no
  longer shells out to `git`; version metadata comes from Go build info).
- Exported `Target` zero value has `Run: nil` (will panic if called). Use
  `NewTarget()` or the parser.
- No `Example*` functions yet; README and godoc examples are limited.

## Architecture Highlights (for agents)

- **internal/parser**: Uses `go/ast` + `go/doc` + `go list -json` to extract
  targets, docs, receivers, namespaces and imported targets. Generates glue
  code.
- **internal/mkf**: Defines `Target`, `Makefile`, execution
  (`Makefile.Execute`), help printers (`HelpTargets`/`HelpUsage`),
  signal/timeout handling, and exit code mapping (`ExitCode`). Its source is
  also embedded and inlined into generated makefiles. Flags use
  `ctx42/xflag` (long/short aliases, typed getters); because the inlined
  runtime imports xflag but user projects do not, `editGoMod` injects the
  xflag `require` + go.sum checksum into the build `go.mod` at compile time
  (version pinned to gomake's own via `xflagVersion`).
- **internal/cli**: High-level `goMake`, `newGoMake`/`Compile`/`Execute`,
  config parsing, `prepare` (the big one that copies sources + edits
  go.mod/go.work + generates), `Main` entry. Runtime `gomake.yaml` handling
  lives in `config_file.go`: schema + strict load, settings-only merge +
  precedence (`applyFileConfig`), a nested `targets:` tree keyed by import path
  then kebab invocation-path names (`localImportPath` fills the empty `ImpSpec`
  of main-package targets), per-invocation nearest-level resolution with
  project-first / user-fallback / in-module skip (`resolveDelivered`,
  `resolveTargetBlock`), delivery of the invoked target's block into `ring.Meta`
  only (`deliverTargetConfig`), and `--check-config` (`runCheckConfig`).
  Delivery: a
  target's config originates solely from
  `ring.Meta`, never the environment. In-process built-in/external targets read
  it directly. For a local target compiled to a subprocess, `Execute` ferries
  the block across as an internal `--gomake-config=<json>` argument; the
  generated `main` (`gen_main.go` template) strips that arg and loads it into
  `gomake.ConfigMetaKey` before the target runs.
- **pkg/gomake**: The module's only public package — env/path helpers
  (`GetGOOS`, `LookupEnv`, `Root`, ...), process helpers (`ExitStatus`,
  `HasRun`), the target-name context key reused by downstream targets, and
  target configuration (`TargetConfig`, `ConfigMetaKey`).
- **internal/builtin**: Built-in target provider (`Generated`/`Empty`,
  `Provider`) for targets compiled into the binary from `targets.yaml`, plus
  their code generation (`GenMain`) and embedded templates. The generated
  `targets.go` is empty by default. Note: `--help`/`--list`/`--version` are CLI
  flags handled in pkg/gomake, not built-in targets.
- **internal/install**: Self-install + version stamping from Go build info
  during install.
- **internal/osarch**: Generated, version-independent list of supported
  GOOS/GOARCH values (from `go tool dist list`, latest Go as a superset) used
  to validate makefile filename suffixes. Regenerate via
  `go generate ./internal/osarch` (set `GOMAKE_GO_VERSION` to pin the Go
  version).
- Test support lives in `internal/cli/clitest` and
  `internal/builtin/builtintest` for use by this module's tests.

Large or complex changes usually touch parser + core + gomake together because
of the codegen + runtime contract.

For the end-to-end runtime and install flow (how `gomake` compiles and runs a
user's makefile, how `cmd/install` works, and how external targets are wired
in), see `dev/README.md`.

## For AI Agents (Grok etc.)

- **Discovery first**: Use `list_dir`, `grep` (with glob excludes for
  testdata/* and *_test.go), targeted `read_file` with offset/limit, and
  terminal commands. Avoid reading multi-thousand-line test files wholesale.
- **Scope discipline**: When the user (or parent orchestrator) supplies
  `packages`, `max_issues`, `depth`, or `plan_only`, respect them exactly.
  Default to producing a plan + estimates before broad godoc, benchmark, or
  systematic fix passes.
- **Verification**: After edits, run `go test ./...` (and targeted if faster).
  Consider the check-work skill for self-review of diffs. Prefer `go test
  -run=...` loops during iteration.
- **Changes**: Use `search_replace` for precise edits. Update CHANGELOG.md
  (newest entry at top) for user-visible or notable internal changes. Keep
  prose under ~80 columns where practical (especially in .md files).
- **AGENTS.md**: Keep this file up to date when project conventions, risky
  areas, or workflows change. It is read during discovery by quality skills.
- **Token hygiene**: For reviews or remediation, follow the `/go-review`
  pattern: plan first, cap findings, use TaskCreate/TaskUpdate for multi-step
  work, report actual files read + rough cost at end.
- Do not start exhaustive work on the whole module without explicit approval
  of scope/budget.

## Recommended Skills / Slash Commands

- `/go-style` before writing or editing any `.go` file — it is the enforced,
  project-specific style guide.
- `/go-review` when edits are complete: a done-time quality review of the diff,
  a package, or the whole module. Scope it with `packages=["internal/parser"]`,
  `max_issues`, `depth`, or `plan_first`; use its opt-in fix path to apply
  findings and run the suite.
- `/go-cover` to raise test coverage one function at a time.
- Use `TaskCreate`/`TaskUpdate` for any task with 3+ steps or touching 4+
  packages.

## Other Notes

- The module path is still github.com/ctx42/gomake (historical). README
  install instructions use `go run .../cmd/install@latest`.
- CI is Jenkins-based (see Jenkinsfile) and uses a custom library + docker
  images with caches. Local `golangci-lint` config for reference lives in
  `tmp/.golangci.yml` (may be stale vs. golangci-lint v2).
- No replace directives in go.mod (good). Testdata contains example go.mod_
  / go.work_ demonstrating workspace + replace support.
- Exit codes are documented in README (126=ErrPickTarget, 127=ErrUnkTarget,
  128+n=signal, etc.).

Update this file when you learn new conventions or gotchas during work.
