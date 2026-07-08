# Development Tasks

Backlog of code-quality and correctness tasks discovered during development
and review. Each task is separated by `---` and carries a title, the relevant
source-code locations, and the background known when it was found.

---

T1: List where things are being printed to a terminal (stdout, stderr) places
like`install.Main`, `cli.Main`, and top level function running tasks are places
we know (list them too) are there any others? List only functions not each
instance of printing to a terminal.

---

T2: Enable GitHub private vulnerability reporting after making the repo public.

  The repo is currently private, so the "Private vulnerability reporting"
  toggle does not exist on Settings -> Advanced Security. Both SECURITY.md and
  CODE_OF_CONDUCT.md tell reporters to use the Security tab's "Report a
  vulnerability" button, which only appears once this is enabled. Right after
  flipping the repo to public, turn it on at
  https://github.com/ctx42/gomake/settings/security_analysis.

---

T3: Centralize CLI error printing behind `fail`/`failCode` helpers. Locations:
`internal/cli/helpers.go` (new helpers near `binName`), `internal/cli/main.go`
(`Main`, `runWithoutCompile`).

  `cli.Main` is the single error sink for the gomake binary; ~13 branches each
  do `fmt.Fprintln(rng.Stderr(), err[.Error()])` followed by a `return`, with
  an `err` vs `err.Error()` inconsistency. `fail(rng, err)` centralizes the
  presentation (prefix `gomake: `, casing, wrapping) and `failCode(rng, err)`
  folds in the common `return mkf.ExitCode(err)`. Informational stderr writes
  (version, help, list, `ignoreWarning`, `withProgress`) are intentionally left
  alone so they are not decorated as errors.

  Decision: `fail` prefixes every error with `gomake: ` (chosen over a
  behavior-preserving no-prefix refactor); affected tests were updated to
  expect the prefix.

  Four sites needed judgment rather than a mechanical swap:

  - `main.go:52` panic recovery used a bespoke `"panicked with: %s\n"` line.
    Routed through `fail` as `fmt.Errorf("panicked with: %w", err)` so it also
    carries the `gomake: ` prefix.
  - `main.go:172` `ErrUnkTarget` returns `mkf.ExitCodeUnkTarget`, not
    `mkf.ExitCode(err)`, so it uses `fail` + its own return, not `failCode`.
  - `main.go:231`/`:238` is the task-run branch (out of scope for error-sink
    work) and `:238` is guarded by `if !gomake.HasRun(err)` returning
    `gomake.ExitStatus(err)`; only the print form was swapped to `fail`, the
    exit-code logic left intact.
  - `main.go:106` `"<tmp> must be a directory"` was a message, not an `error`
    value; wrapped in `fmt.Errorf` and routed through `fail` (also fixes a
    previously missing trailing newline).
