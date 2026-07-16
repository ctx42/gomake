## v0.25.0 (Thu, 16 Jul 2026 07:48:12 UTC)
- docs: document installing from a local clone.

## v0.24.0 (Wed, 15 Jul 2026 20:01:44 UTC)
- fix(install): resolve external targets against the build tree.

## v0.23.0 (Tue, 14 Jul 2026 10:38:19 UTC)
- feat(install): report empty --targets= instead of proceeding silently.
- refactor(install): detect empty --targets= via xflag WasSet.

## v0.22.0 (Sun, 12 Jul 2026 19:29:08 UTC)
- feat(install): resolve local --targets modules via a Go workspace.
- test(clitest): isolate user config in TestEnv.
- docs: remove dev/ maintainer notes.
- docs: document local target development via workspace.

## v0.21.0 (Sun, 12 Jul 2026 12:44:40 UTC)
- build(deps): bump xdef to v0.5.0 and testkit to v0.8.0.
- refactor(cli): split concatenated WriteString calls.

## v0.20.1 (Fri, 10 Jul 2026 20:26:03 UTC)
- fix(builtin): honor namespace for external target imports.

## v0.20.0 (Fri, 10 Jul 2026 19:29:09 UTC)
- feat(config): accept a []byte config block in TargetConfig.

## v0.19.0 (Fri, 10 Jul 2026 14:30:22 UTC)
- docs(config): document target config internals and fix template.
- feat(config)!: add typed path-based config value access.
- docs(dev): fix table formatting and align columns in markdown files.

## v0.18.0 (Thu, 09 Jul 2026 20:51:42 UTC)
- feat(config)!: nest gomake.yaml targets by import path and name.

## v0.17.0 (Thu, 09 Jul 2026 15:06:45 UTC)
- fix(cli): print error when tmp dir cannot be prepared.
- refactor(version): source placeholders and var names from xdef.

## v0.16.0 (Wed, 08 Jul 2026 14:37:40 UTC)
- feat(docs): add favicons, social preview image, and meta.
- docs: update TASKS.md with stdout/stderr check and GitHub vulnerability toggle steps.
- docs: update TASKS.md with stdout/stderr check and GitHub vulnerability toggle steps.
- feat(cli): prefix command errors with "gomake:".
- docs: add terminal-output inventory and error-sink task.

## v0.15.0 (Tue, 07 Jul 2026 21:23:05 UTC)
- chore: initial empty commit.
- feat: initial public release.
- test: make cli progress output deterministic.
- docs: fix Hugo config for v0.128+ (paginate -> pagination.pageSize).
- ci: bump deprecated docs workflow actions to Node 24 majors.
- ci: bump remaining docs Pages actions to Node 24 majors.
- docs: add CNAME for gomake.ctx42.com custom domain.

