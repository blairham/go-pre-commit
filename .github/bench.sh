#!/usr/bin/env bash
# Benchmark: this tool vs Python pre-commit, on a scratch repository.
#
# It used to time `run --all-files` over this repo's own config, which runs
# golangci-lint and whatever Python happened to be installed. This builds a
# small repo with a fixed config instead, so the numbers in the README can be
# reproduced, and it refuses to run against a Python pre-commit that is not the
# version the parity suite targets.
#
# Usage: PRE_COMMIT_PY=/path/to/python-pre-commit bash .github/bench.sh
#   (default: `pre-commit` from `python3 -m pre_commit`)
set -euo pipefail

cd "$(dirname "$0")/.."
want="$(grep -oE "pre-commit==[0-9][0-9.]*" .github/requirements/parity.in | head -1 | cut -d= -f3)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

ver="$(git describe --tags --always --dirty | sed 's/^v//')"
go build -ldflags "-X github.com/blairham/go-pre-commit/v4/internal/config.Version=$ver" -o "$work/go-pc" .
if [ -n "${PRE_COMMIT_PY:-}" ]; then
  py=("$PRE_COMMIT_PY")
else
  py=(python3 -m pre_commit)
fi
got="$("${py[@]}" --version 2>&1)"
case "$got" in
  "pre-commit $want") ;;
  *) echo "need Python pre-commit $want (CI's parity pin), got: $got" >&2; exit 1 ;;
esac

repo="$work/repo"
git init -q "$repo"
cd "$repo"
git config user.email bench@example.com
git config user.name bench
git config commit.gpgsign false
printf 'module x\n\ngo 1.26\n' > go.mod
printf 'package main\n\nfunc main() {}\n' > main.go
for i in $(seq 1 40); do
  printf 'line %s\n' "$i" > "f$i.txt"
  printf 'k%s: v\n' "$i" > "y$i.yaml"
done
cat > .pre-commit-config.yaml <<'YAML'
repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v5.0.0
    hooks:
    -   id: trailing-whitespace
    -   id: end-of-file-fixer
    -   id: check-yaml
    -   id: check-added-large-files
    -   id: check-merge-conflict
-   repo: local
    hooks:
    -   id: go-vet
        name: go vet
        entry: go vet ./...
        language: system
        pass_filenames: false
        types: [go]
YAML
git add -A
git commit -qm init

# Separate stores, so neither tool times the other's environment builds, and
# both warmed before anything is measured.
for t in go py; do
  if [ "$t" = go ]; then exe=("$work/go-pc"); else exe=("${py[@]}"); fi
  PRE_COMMIT_HOME="$work/home-$t" "${exe[@]}" install-hooks > /dev/null
  PRE_COMMIT_HOME="$work/home-$t" "${exe[@]}" run --all-files > /dev/null
done

echo "this tool:  $("$work/go-pc" --version 2>&1)"
echo "python:     $got"
echo "machine:    $(uname -sm), load $(uptime | sed 's/.*load averages*: //')"
echo

python3 - "$work" "${py[@]}" <<'PY'
import os, statistics, subprocess, sys, tempfile, time

work, py = sys.argv[1], sys.argv[2:]
tools = {"go": [os.path.join(work, "go-pc")], "py": py}
repo = os.path.join(work, "repo")


def timed(tool, args, extra_env=None):
    env = dict(os.environ, PRE_COMMIT_HOME=os.path.join(work, "home-" + tool), **(extra_env or {}))
    start = time.perf_counter()
    p = subprocess.run(tools[tool] + args, cwd=repo, env=env, capture_output=True)
    elapsed = time.perf_counter() - start
    if p.returncode != 0:
        sys.exit(f"{tool} {args} exited {p.returncode}:\n{p.stdout.decode()}")
    return elapsed


def compare(label, args, runs=15, fresh_gocache=False):
    times = {"go": [], "py": []}
    for i in range(runs):
        # Alternate which tool goes first, so neither always runs warm.
        for tool in ("go", "py") if i % 2 == 0 else ("py", "go"):
            extra = {"GOCACHE": tempfile.mkdtemp(dir=work)} if fresh_gocache else None
            times[tool].append(timed(tool, args, extra))
    g, p = statistics.median(times["go"]), statistics.median(times["py"])
    print(f"| {label} | {g:.3f}s | {p:.3f}s | {p / g:.1f}x |")


print(f"| Case (median) | this tool | Python | |")
print(f"|---|---|---|---|")
compare("startup (`run`, nothing staged)", ["run"])
for hook in ["trailing-whitespace", "end-of-file-fixer", "check-yaml", "check-added-large-files", "check-merge-conflict"]:
    compare(f"`{hook}`", ["run", hook, "--all-files"])
compare("`go vet`, warm build cache", ["run", "go-vet", "--all-files"])
compare("`go vet`, cold build cache", ["run", "go-vet", "--all-files"], runs=6, fresh_gocache=True)
PY
