# Security Policy

## Supported versions

The latest release, and nothing else. This is a single-maintainer personal
project with no SLA — see [docs/stability.md](docs/stability.md#support).

| Version | Supported |
|---|---|
| Latest release | ✅ |
| Anything older | ❌ — upgrade first, then report if it persists |

## Reporting a vulnerability

**Do not open a public issue.**

Use GitHub's [private vulnerability reporting](https://github.com/blairham/go-pre-commit/security/advisories/new),
which is enabled on this repository. If that is not available to you, email
**blairham@me.com** with `go-pre-commit` in the subject.

Please include what you would want to receive: the version, the platform, the
smallest reproduction you have, and what an attacker gets out of it.

Expect an acknowledgement within a week. This is a side project, so that is a
best effort and not a commitment. If a fix is warranted it ships in the next
release with an advisory; you will be credited unless you ask not to be.

## What is in scope

This tool downloads and executes code by design — that is what a hook framework
does — so the interesting boundary is *whose* code, and whether the tool can be
made to run something the user did not ask for.

- **Supply-chain integrity of the installer.** The composite action verifies
  the release archive against `checksums.txt` before extracting it. A way to
  make it skip, or pass, that check is in scope.
- **Cache and store handling.** Path traversal out of `~/.cache/pre-commit`
  when cloning a hook repository or naming an environment directory; a
  malicious repo or manifest that writes outside the store.
- **Config parsing.** A `.pre-commit-config.yaml` or `.pre-commit-hooks.yaml`
  that causes execution the config does not describe — command injection
  through a hook's `entry`, `args`, or filename arguments.
- **Git hook installation.** `install` writing something other than what it
  reports, or clobbering an existing hook without the documented backup.
- **Divergence from upstream that has a security consequence** — for example,
  honoring something upstream deliberately rejects.

## What is not in scope

- **Hooks doing what they were configured to do.** If your
  `.pre-commit-config.yaml` points at a repository, this tool clones it and
  runs it. That is the design, it is upstream's design, and a hostile entry in
  your own config is not a vulnerability in the runner. Review what you add.
- **Behavior inherited from upstream by intent.** `clean` removes the entire
  store, including environments Python pre-commit built — that is documented
  and matches upstream. Report it upstream if you think it is wrong there.
- **The shared cache directory.** Sharing `~/.cache/pre-commit` with the Python
  tool is deliberate and documented in
  [docs/parity.md](docs/parity.md#deliberate-behaviors-that-surprise-people).
- **Vulnerabilities in hooks you run, or in the repositories they come from.**
  Those belong to their maintainers.
- **Anything requiring an attacker who already has write access** to your repo,
  your config, or your machine.
