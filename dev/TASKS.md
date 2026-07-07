# Development Tasks

Backlog of code-quality and correctness tasks discovered during development
and review. Each task is separated by `---` and carries a title, the relevant
source-code locations, and the background known when it was found.

---

- errors should have gomake prefix, but they should be added on the top of the call chain.

---

- Enable GitHub private vulnerability reporting after making the repo public.

  The repo is currently private, so the "Private vulnerability reporting"
  toggle does not exist on Settings -> Advanced Security. Both SECURITY.md and
  CODE_OF_CONDUCT.md tell reporters to use the Security tab's "Report a
  vulnerability" button, which only appears once this is enabled. Right after
  flipping the repo to public, turn it on at
  https://github.com/ctx42/gomake/settings/security_analysis.
