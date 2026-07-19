## v0.26.1 (Sun, 19 Jul 2026 19:57:33 UTC)
- fix(cli): expand leading ~ in --targets path.

## v0.26.0 (Thu, 16 Jul 2026 11:54:31 UTC)
- fix(cli): pass target name to HelpUsage.
- fix(cli): widen makefile binary cache key.
- fix: deliver GOMAKE_VERSION and PROJECT_DIR via env.
- fix(gomake): treat any ExitError as HasRun.
- fix(mkf): ignore deadline after target result.
- fix(mkf): surface cwd restore errors and cover SIGTERM.
- fix(cli): run PreRuns for builtins and MkdirAll tmp.
- fix(cli): read GOMAKE_INSTALL_PATH from ring env.
- fix(parser): harden import resolve, signatures, and cache.
- fix(gomake): tighten GetCfgDefault, env split, and ReadLine.
- fix(install): surface download stdout and restore errors.
- fix(builtin): clone Targets and PreRuns slices.
- fix(cli): cancel HTTP targets.yaml fetch via context.
- fix(version): sanitize ldflag values with both quote types.
- fix(clitest): top-level makefiles and fatal goCache.
- fix(parser): treat empty file list as empty AST.
- fix(cli): hash workspace and replace trees in cache key.
- fix(cli): surface ErrNoGoMod when a target is named.
- fix(cli): use exit 129 for --bin compile failures.
- fix(cli): absolutize all relative go.work use paths.
- fix(cli): locate go.work via walk and GOWORK.
- fix(parser): alias imports when package names collide.
- fix(gomake): deep-copy GetCfg any map and slice values.
- fix(mkf): reject negative --timeout values.
- fix(mkf): prefer finished target over cancel race.
- fix(gomake): decode config numbers with json.Number.
- fix(cli): strip gomake build tags from CRLF sources.
- fix(install): trim whitespace-only --targets values.
- fix(parser): use meta build tag in importDir.
- fix(builtintest): clone PreRuns slice on return.
- test(install): clear process PATH for go LookPath case.
- fix(osarch): parse prerelease minors in majorMinor.
- docs(gomake): fix target signature in package README.
- fix(cli): map only missing makefile.go to errNoMakefile.
- fix(mkf): correct ExitCodeCompile godoc and done channel.
- docs(gomake): mention target config in package godoc.
- test(version): assert PopulateVersion build date from clock.
- fix(cli): pin GOWORK so editGoWork cannot touch the user file.
- fix(parser): unique import aliases and method receiver vars.
- fix(cli): pin GOWORK for compile and align binary cache.
- fix(mkf): keep 128+n exit when signal races cooperative cancel.
- fix(gomake): reject JSON null for typed GetCfg.
- fix(parser): MarkDefault sets only the first matching DefRef.
- fix(parser): write go list cache with rename.
- fix(install): isolate GOWORK and restore created snapshot paths.
- fix(cli): harden workspace discovery and go.work cache trees.
- fix(parser): resolve Default import aliases and reserve names.
- fix(gomake): map signal-killed processes to 128+n exit codes.
- fix(install): treat HTTP targets schemes case-insensitively.
- fix(version): quote ldflags that contain newlines.
- fix(clitest): CRLF tag strip and fatal findMakefiles errors.
- fix(cli): wire ring Stdin into makefile subprocess.
- fix(parser): reserve targets and tgt import aliases.
- fix(gomake): keep UseNumber when GetCfg decodes composites.
- fix(cli): use unique temps when storing binary cache.
- fix(parser): include GOFLAGS in list cache and unique temps.
- fix(cli): accept HTTP targets URLs case-insensitively.
- fix(parser): soft-skip empty gomake:import packages.
- fix(parser): apply GOFLAGS -tags in importDir.
- fix(cli): surface makefile Stat errors and compile cancel codes.
- fix(cli): check user-level targets tree in --check-config.
- fix(parser): accept spaced // gomake:ns_root comments.
- fix(builtin): clone PreRuns at construction time.
- fix(clitest): stop after Fatal on makefile read/write and goCache.
- fix(gomake): surface non-NotExist Stat errors in Root.
- fix(gomake): accept json.RawMessage in TargetConfig.
- docs(gomake): clarify PathExists/FileExists/DirExists Stat errors.
- fix(parser): strip * from pointer method receivers.
- fix(parser): reject list-cache hits with wrong ImpSpec.
- fix(parser): hash active go.work content in list cache.
- fix(install): resolve devel source from module Root.
- fix(cli): include toolchain env in binary cache key.
- fix(cli): reject negative timeout from flags and yaml.
- fix(cli): skip project yaml load for --version.
- fix(cli): drop dead flag.ErrHelp check after Execute.
- fix(mkf): wrap restore-cwd error with %w.
- fix(cli): absolutize local go.mod/go.work replaces.
- fix(cli): strip compound //go:build lines that mention gomake.
- fix(cli): list/help only analyze valid makefile names.
- docs(cli): align goEditErr comment with %w wrap.
- fix(cli): drop all go.work uses before re-adding.
- docs: align cache, config, and agent notes with runtime.

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

