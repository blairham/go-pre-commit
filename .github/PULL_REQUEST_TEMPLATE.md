## Why

<!-- What problem does this solve? Explain the why, not a restatement of the diff. -->

## What changed

<!-- Brief summary. -->

## Parity

<!--
This project's contract is "behaves like Python pre-commit". Pick one:

- Matches upstream — and here is the upstream behavior it now matches.
- No behavior change (refactor, docs, CI, tests).
- Deliberate divergence — explain why, because this needs a very good reason.
-->

## Checklist

- [ ] `make check` passes (`fmt` + `vet` + `test` — note this does **not** run lint)
- [ ] `make lint` passes
- [ ] Tests do not touch real user state (`t.TempDir()` + `t.Setenv` for `HOME`, `PRE_COMMIT_HOME`, `XDG_CACHE_HOME`)
- [ ] If behavior changed, the differential parity suite still passes
- [ ] Docs updated if this changes something a user can observe

Closes #
