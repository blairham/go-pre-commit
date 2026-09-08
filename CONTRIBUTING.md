# Contributing

Thanks for looking. This is a personal project with a single maintainer, so the
most useful thing you can do is usually smaller than you think.

## The most valuable contribution is a divergence report

This project has one job: behave like Python pre-commit. So the highest-value
bug report is not "this crashed" — it is **"upstream does X, this does Y"**.

That report is worth more than a patch, because it is the thing this project
cannot generate for itself. A good one is two commands and two outputs:

```console
$ pre-commit run trailing-whitespace --all-files   # this tool
...

$ python -m pre_commit run trailing-whitespace --all-files   # upstream 4.6.2
...
```

Include the versions of both, and the `.pre-commit-config.yaml` if the behavior
depends on it. See [docs/parity.md](docs/parity.md#reporting-a-divergence).

**Divergence is a bug, never a feature.** If you want behavior Python
pre-commit does not have, this is the wrong tracker — asking
[upstream](https://github.com/pre-commit/pre-commit) helps everyone, and if
they ship it, it arrives here as parity work. A PR that adds a flag upstream
does not have will be declined, however good the flag is.

## Before you open a pull request

Open an issue first for anything that is not a small, obvious fix. This is a
single-maintainer project and an unsolicited large PR is likely to sit, or to
be declined for a reason that a five-line issue would have surfaced first.

## Development

Go is pinned in `go.mod` and `.tool-versions` (both must match exactly), and
runtime versions are managed with [asdf](https://asdf-vm.com).

```bash
git clone https://github.com/blairham/go-pre-commit
cd go-pre-commit
asdf install          # optional, if you use asdf
pre-commit install    # or: go run . install

make build            # build/pre-commit
make test             # go test -race ./...
make check            # fmt + vet + test
make lint             # go tool golangci-lint run ./...
```

**`make check` does not run `lint`.** It is `fmt + vet + test`. Run `make lint`
as well before pushing, or let CI tell you.

Formatting and linting are pinned as Go tools in `go.mod` — `go tool gofumpt`
and `go tool golangci-lint`, not separately installed binaries — so the version
you run is the version CI runs.

### The parity harness

The claim on the front page is generated, not remembered. `docs/parity.md` is
written from a report produced by the differential suite, which runs this tool
and Python pre-commit against the same inputs and diffs them:

```bash
go test ./test/integration/... -run Parity -v
```

It pins the upstream version it measures against and refuses any other, so the
number cannot quietly drift. If you change behavior, this suite is the thing
that decides whether you were right.

### Tests must not touch your real state

Hooks and caches are the whole subject matter here, which makes it very easy to
write a test that quietly eats a developer's `~/.cache/pre-commit` or their git
config. Redirect with `t.TempDir()` and `t.Setenv` — including `PRE_COMMIT_HOME`,
`XDG_CACHE_HOME` and `HOME` — and never assume the temp dir is a safe default.

## Commits and pull requests

- Work on a branch; do not commit to `main`.
- Commit messages explain **why**, not a restatement of the diff.
- Never bypass hooks with `--no-verify`.
- Put `Closes #N` in the PR body for the issues it resolves.
- No AI-attribution trailers in commit messages or PR bodies.

## Adding a language backend

Ten of the 22 language backends are implemented but effectively unexercised —
see the [grading table](docs/parity.md#language-support-graded). Moving one of
those rows from "untested" to "proven" is genuinely wanted work, and it is
mostly test-writing rather than implementation. Start by adding it to the
differential suite and seeing what breaks.

## Scope

- **In scope:** parity with Python pre-commit, the platforms we publish
  binaries for, performance, and evidence that any of the above is true.
- **Out of scope:** features upstream does not have, changes to the config
  format, and anything that would make a `.pre-commit-config.yaml` written for
  this tool fail on the Python one.

## License

By contributing you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE), and that you have the right to submit them.
