# Stability and versioning

## The version number is not about this codebase

`v4.6.6` does **not** mean "the sixth patch of the sixth minor of this
project". It means: *this behaves like Python pre-commit 4.6.x*.

| Part | Meaning |
|---|---|
| `4.6` | The Python pre-commit line whose behavior is targeted. |
| `.6` | Releases of this implementation against that line — fixes, coverage, packaging. |

So `v4.6.6` and `v4.6.7` both target upstream 4.6. When upstream releases 4.7,
this project's next version is `v4.7.0`, and it means *parity with 4.7*, not
"new features from us".

This is unusual, and it has one consequence worth stating plainly: **a bump in
the minor version is not a promise about this project's maturity.** `v4` here is
inherited from upstream's numbering, not a claim to four major versions of
stability. Judge maturity from [parity.md](parity.md), which is evidence.

The CI parity harness pins the Python version it measures against to the
declared line and refuses any other, so this number cannot quietly drift away
from what it claims.

## What is frozen

These are the contracts. Breaking any of them requires a release that says so
in its notes, prominently.

- **The CLI.** Command names, flags, exit codes, and the meaning of each. They
  are upstream's, and they change when upstream changes them.
- **The config format.** `.pre-commit-config.yaml` and hook manifests are
  upstream's schemas. This project does not add keys to them.
- **The cache contract.** `PRE_COMMIT_HOME`, `XDG_CACHE_HOME`, and the default
  `~/.cache/pre-commit`.
- **The binary name.** `pre-commit`. Installed git hooks invoke it by that name,
  so renaming it would break every repo that ran `install`.
- **The composite action's inputs.** `version`, `extra_args`, `cache`,
  `install-only`.

## What is not frozen

- Anything under `internal/`. It is not an API; import paths there can change in
  any release.
- Log and progress output that is not part of upstream's documented output.
- The set of platforms with published binaries.
- Which languages are *proven*, in the sense of [parity.md](parity.md) — that
  table should only ever improve, but it is a status report, not a promise.

## Upstream changes win

When Python pre-commit changes behavior in a way this project has copied, this
project follows — even if the old behavior was nicer. That is the whole
proposition. If you need behavior upstream does not have, this is the wrong tool
to ask, and asking upstream is more likely to help everyone.

## Support

Single maintainer, personal project, no company behind it, no SLA. What that
buys you if it stops being maintained: your `.pre-commit-config.yaml` is
upstream's format and your hook repositories are upstream's, so the exit is
`pip install pre-commit` and deleting one line from a workflow. That is
deliberate — there is no state, no service, and nothing proprietary anywhere in
the path.
