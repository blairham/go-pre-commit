# Should you use this instead of Python pre-commit?

Often, no. This page exists to make that easy to determine, because a
reimplementation that oversells itself wastes your afternoon and earns nothing.

## Keep using Python pre-commit if

- **It is working for you.** There is no problem here that you have and it
  solves; the tool you already have is the reference implementation, maintained
  by the people who designed the thing.
- **Your team already has Python everywhere.** The install cost this removes is
  a cost you are not paying.
- **You rely on a language backend outside the well-trodden set.** Python,
  system and pygrep are measured against upstream on every pull request; golang,
  node, ruby and rust run real hooks in CI; script and fail have unit tests
  only. Julia, Swift, R, Haskell and friends are implemented but effectively
  unexercised — see [parity.md](parity.md) for the grading.
- **You need Windows beyond `python`, `node`, `golang`, `ruby` and `rust`.**
  Those five run on Windows in CI on every pull request (up to v4.6.9 every
  hook that installs an environment failed there). The other
  environment-building languages have never been run on Windows, and upstream
  has years of Windows users behind it.
- **You want the guarantee that the tool matches the docs at pre-commit.com.**
  Only one implementation can promise that, and it is not this one.

## Consider this if

- **CI spends real time installing Python to run three whitespace hooks.** The
  common case for adopting this is a repo with no other reason to have a Python
  toolchain — a Go, Rust, or Node service whose lint hooks are the only Python
  in the build.
- **You want one binary.** No interpreter, no virtualenv, no `pip`, no
  `python_version` skew between a laptop and a runner.
- **Your hooks are already mostly `system`, `script` or Go tools.** That path is
  both the best-tested here and the one where the interpreter is pure overhead.

## What it is not

- **Not a fork.** It is an independent Go implementation of upstream's
  behavior, and it can be wrong in ways upstream is not. What it takes from
  upstream — the design, and `identify`'s file-type tables as data — is
  credited in [NOTICE](../NOTICE).
- **Not affiliated with the pre-commit project.** Do not file its bugs on their
  tracker. Do not ask them about it.
- **Not a different design.** There is deliberately no feature here that
  upstream lacks. New behavior would be a compatibility break with extra steps;
  divergence is a bug. If you want something pre-commit does not do, the useful
  place to ask for it is upstream.
- **Not faster at running your hooks.** It saves a roughly fixed 0.15–0.2s of
  interpreter startup per run. That is most of the time for a trivial hook and
  none of it for a real one. If your slow hook is `golangci-lint` or `eslint`,
  the tool doing the work is the same tool, and the number will not move. The
  README's benchmark shows exactly that: 3.5× on `trailing-whitespace`, 1.0× on
  a `go vet` that has to compile.

## Against other options

| Option | When it is the better answer |
|---|---|
| **Python pre-commit** | Almost always, if you already have it. The reference implementation of the thing this copies. |
| **`pre-commit.ci`** | You want hooks fixed and pushed automatically on PRs. That is a hosted service; this is a binary and does not compete with it. |
| **`lefthook`, `husky`, `overcommit`** | You want a git-hook runner on its own terms and you are not attached to the pre-commit config format or its hook ecosystem. They are not drop-ins — different config, different hook repos. |
| **A shell script in `.git/hooks`** | Your needs are one or two checks and you do not want a framework. This is a fine answer and you should not feel talked out of it. |
| **This** | You want the pre-commit config format and its hook ecosystem, without an interpreter in the way. |

## The honest summary

The pitch is narrow on purpose: **the same tool, the same config, the same hook
repositories, without needing Python installed to run them.** Everything else —
the startup time, the single binary — follows from that one thing. If the
Python dependency is not costing you anything, this project has nothing to sell
you, and that is a fine outcome.
