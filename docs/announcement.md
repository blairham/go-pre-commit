# Launch playbook

Written before anything is posted, because the decisions are easier to make now
than in a comment thread at 200 upvotes.

## What is being claimed

One sentence, and it is the only one that matters:

> Run your existing pre-commit hooks without installing Python.

Not *faster*. Not *better*. Not *a replacement for*. The claim is that a repo
whose only reason to install a Python toolchain is its hooks can stop doing
that. Everything else is a detail of how well it holds.

## What is not being claimed

These are the sentences that turn a launch into an argument, so they do not get
written:

- **"Faster than pre-commit."** It starts faster and it is faster on trivial
  hooks. If your slow hook is `golangci-lint`, the tool doing the work is the
  same tool and the number does not move. The README benchmark says exactly
  that — 4× on `trailing-whitespace`, 1.0× on `golangci-lint` — and the honest
  framing leads with the 1.0×.
- **"A replacement for pre-commit."** It is an independent reimplementation of
  someone else's design. The reference implementation is theirs.
- **"100% compatible."** The number is 82 differential checks, measured, with a
  named upstream version. That is a much smaller and much more defensible
  claim, and it is the one on the front page.
- **Anything about Windows.** See below.

## Known weak points, stated before someone finds them

A port gets exactly one chance to look honest. Each of these is already in the
docs; the point of listing them here is that the launch post links to them
rather than waiting to be asked.

| Weak point | Where it is documented |
|---|---|
| Windows: hooks that install an environment do not work | [platform support](parity.md#platform-support), [#53](https://github.com/blairham/go-pre-commit/issues/53) |
| Ten of 22 language backends are effectively unexercised | [language grading](parity.md#language-support-graded) |
| The binary is named `pre-commit` and shadows the Python one | [deliberate behaviors](parity.md#deliberate-behaviors-that-surprise-people) |
| The cache is shared with Python pre-commit, and `clean` is destructive | same |
| `v4.6.x` is upstream's number, not a maturity claim | [stability](stability.md) |
| Single maintainer, no SLA | [stability](stability.md#support) |

**If a comment finds one of these, the answer is "yes, that is in the docs,
here is the link" — never a defense.** They are known, written down, and the
writing down is the point.

## Where, and in what order

Order matters: the places with the least tolerance for self-promotion come
last, after the first wave has produced real questions and real fixes.

1. **`r/golang`** — the audience most likely to have the exact problem (a Go
   repo installing Python for three whitespace hooks). Lead with the problem,
   not the tool.
2. **Hacker News** *Show HN* — only if 1 goes without a factual correction. HN
   is where the "why not just use lefthook" and "this is a fork" questions
   arrive; both have answers in [comparison.md](comparison.md), which is why it
   was written first.
3. **Lobsters** — needs a tag and a genuine reason; skip rather than force it.

**Not upstream's issue tracker, discussions, or Discord.** Announcing a
reimplementation in the original project's space is rude, and it is how a port
earns a reputation before it earns users. If upstream maintainers find it and
want it mentioned somewhere, that is their call to make, not ours to ask for.

## The first comment to expect, and the answer

- *"Why not just use pre-commit?"* — You should. Link
  [comparison.md](comparison.md), which opens with the case for staying.
- *"Is this a fork?"* — No, and it shares no code. Link [NOTICE](../NOTICE).
- *"Does it work on Windows?"* — Not for hooks that install an environment.
  Link [#53](https://github.com/blairham/go-pre-commit/issues/53). Do not
  soften it.
- *"How do you know it is compatible?"* — 82 differential checks against a
  pinned upstream version, run on every PR, and a divergence fails the build.
  Link [parity.md](parity.md).
- *"It shadows the Homebrew formula."* — Deliberate; a drop-in has to answer to
  the name a git hook invokes. Link the parity doc.

## Before posting

- [ ] `main` green, including the macOS and Windows action legs
- [ ] A release exists whose notes name the upstream version it targets
- [ ] `brew install blairham/tap/pre-commit` on a clean machine
- [ ] `uses: blairham/go-pre-commit@v4` resolves and runs
- [ ] Every command in the README quick start, in order, in a scratch repo
- [ ] Issue templates render (open one, cancel it)
- [ ] The Marketplace listing exists and its description matches the README's
      first line

## After posting

Answer questions for the first few hours or do not post that day. An
unattended launch thread is worse than no launch thread — the corrections
arrive whether or not anyone is there to accept them, and the ones that go
unanswered are the ones that get quoted later.

File anything factual as an issue while the thread is live, and link the issue
back into the thread. It is the cheapest possible demonstration that reports go
somewhere.
