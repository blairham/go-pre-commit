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

## Verifying what you downloaded

Releases are built by `.github/workflows/release.yml`, which runs the shared
release workflow in [blairham/.github](https://github.com/blairham/.github)
(`.github/workflows/go-release.yml`). It signs `checksums.txt` with
[cosign](https://github.com/sigstore/cosign) keyless signing (GitHub OIDC):
the signature is tied to the workflow that built the release, not to a key
someone could leak. The signing identity is that shared workflow; the
certificate also names this repository and the tag, so verify all three — not
just "anything in this repository". `checksums.txt` lists the digest of every
archive, so verify the signature, then the archives against it, then the
provenance:

```sh
VERSION=v4.6.17
cosign verify-blob \
  --certificate-identity-regexp '^https://github\.com/blairham/\.github/\.github/workflows/go-release\.yml@' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-github-workflow-repository blairham/go-pre-commit \
  --certificate-github-workflow-ref "refs/tags/$VERSION" \
  --bundle checksums.txt.sigstore.json checksums.txt
sha256sum --check --ignore-missing checksums.txt
gh attestation verify pre-commit_Linux_x86_64.tar.gz --repo blairham/go-pre-commit \
  --signer-workflow blairham/.github/.github/workflows/go-release.yml
```

A release re-run by hand (the workflow's `workflow_dispatch` input) is signed
for the ref it was dispatched from, usually `refs/heads/main`, rather than the
tag, so use that in `--certificate-github-workflow-ref` for such a release.

The SLSA build provenance ties each archive to the exact workflow run and
commit that built it. GitHub stores it, and the same attestation is attached
to the release as `go-pre-commit-$VERSION.intoto.jsonl`, for checking without
a round trip to GitHub's attestation store:

```sh
gh attestation verify pre-commit_Linux_x86_64.tar.gz --repo blairham/go-pre-commit \
  --signer-workflow blairham/.github/.github/workflows/go-release.yml \
  --bundle "go-pre-commit-$VERSION.intoto.jsonl"
```

**Tags released before the move to blairham/.github** (`v4.6.16` and earlier)
were signed by this repository's own `goreleaser.yml`. Verify those with
`--certificate-identity "https://github.com/blairham/go-pre-commit/.github/workflows/goreleaser.yml@refs/tags/$VERSION"`
in place of the three identity flags above, and without `--signer-workflow`.
Signatures start at `v4.6.8` and provenance at `v4.6.14`; `v4.6.14` itself
carries the older slsa-github-generator provenance (`multiple.intoto.jsonl`),
which verifies with [slsa-verifier](https://github.com/slsa-framework/slsa-verifier)
instead.

The macOS builds are additionally Developer ID signed and notarized.

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
