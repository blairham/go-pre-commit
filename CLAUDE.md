# CLAUDE.md

@AGENTS.md

<!--
AGENTS.md (imported above) is the cross-tool single source of truth for this repo
— project overview, build/test commands, structure, the upstream-parity contract,
and CI/CD. Claude Code does not read AGENTS.md natively, so this file imports it
and holds only Claude Code-specific extras. Put repo guidance in AGENTS.md, not
here.
-->

## Claude Code-specific notes

- **`make check` is `fmt vet test` — it does not lint.** Run `make check` *and* `make lint` before proposing a PR.
- **The repo-root `action.yml` is public API.** Changing its inputs breaks `aws-sso-config`, `aws-config-management`, and `ghorg`, which consume it in CI. Grep those repos' workflows before touching it.
- **Behavior questions are settled by upstream, not by taste** — when unsure how a hook, flag, or cache path should behave, check Python pre-commit's source or CHANGELOG rather than choosing something reasonable-looking.
- Parity tests need real Python pre-commit and don't run by default: `go test -v -tags=integration -timeout=600s ./test/integration/`.
- The permission allowlist is in `.claude/settings.json`; the tree-level `~/Developer/github.com/blairham/.claude/settings.json` applies too. `.claude/settings.local.json` holds untracked, machine-local grants.
- Commits and PRs carry no AI-attribution trailers (see the tree-level AGENTS.md).
