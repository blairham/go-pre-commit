#!/usr/bin/env bash
# Performance benchmark: Go pre-commit vs Python pre-commit
set -euo pipefail

GO_BIN="./build/pre-commit"
RUNS=5

cd "$(dirname "$0")"

echo "================================================================"
echo "  Pre-commit Performance Benchmark"
echo "================================================================"
echo ""
echo "  Go binary:    $GO_BIN"
echo "  Python:       python3 -m pre_commit"
echo "  Iterations:   $RUNS"
echo ""

# Warm up caches.
echo "--- Warming caches ---"
${GO_BIN} run --all-files > /dev/null 2>&1 || true
python3 -m pre_commit run --all-files > /dev/null 2>&1 || true
echo "  Done"
echo ""

run_precise() {
    local label="$1"
    local go_args="$2"
    local py_args="$3"

    echo "================================================================"
    echo "  Benchmark: $label"
    echo "================================================================"
    echo ""

    local go_times=""
    local py_times=""

    for i in $(seq 1 "$RUNS"); do
        go_elapsed=$(python3 -c "
import subprocess, time
s = time.time()
subprocess.run('${GO_BIN} ${go_args}'.split(), capture_output=True)
print(f'{time.time()-s:.3f}')
")
        py_elapsed=$(python3 -c "
import subprocess, time, sys
s = time.time()
subprocess.run([sys.executable, '-m', 'pre_commit'] + '${py_args}'.split(), capture_output=True)
print(f'{time.time()-s:.3f}')
")
        printf "  Run %d/%d — Go: %ss  Python: %ss\n" "$i" "$RUNS" "$go_elapsed" "$py_elapsed"
        go_times="$go_times $go_elapsed"
        py_times="$py_times $py_elapsed"
    done

    echo ""
    python3 -c "
go = [float(x) for x in '$go_times'.split()]
py = [float(x) for x in '$py_times'.split()]
ga, gn, gx = sum(go)/len(go), min(go), max(go)
pa, pn, px = sum(py)/len(py), min(py), max(py)
print(f'  Go:     avg={ga:.3f}s  min={gn:.3f}s  max={gx:.3f}s')
print(f'  Python: avg={pa:.3f}s  min={pn:.3f}s  max={px:.3f}s')
print()
s = pa / ga if ga > 0 else float('inf')
print(f'  Speedup: {s:.1f}x {\"faster\" if s > 1 else \"slower\"} (Go vs Python)')
"
    echo ""
}

run_precise "run --all-files (all hooks, all files)" "run --all-files" "run --all-files"
run_precise "run (no staged files — startup overhead)" "run" "run"
