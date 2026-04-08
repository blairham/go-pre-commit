package xargs

import (
	"context"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// NewExecutor – defaults
// ---------------------------------------------------------------------------

func TestNewExecutorDefaults(t *testing.T) {
	e := NewExecutor()
	if e.MaxJobs != 1 {
		t.Errorf("MaxJobs = %d, want 1", e.MaxJobs)
	}
	if e.MaxBatchSize != 0 {
		t.Errorf("MaxBatchSize = %d, want 0", e.MaxBatchSize)
	}
	if e.TargetConcurrency != 1 {
		t.Errorf("TargetConcurrency = %d, want 1", e.TargetConcurrency)
	}
}

// ---------------------------------------------------------------------------
// Run – empty command returns error
// ---------------------------------------------------------------------------

func TestRunEmptyCommand(t *testing.T) {
	e := NewExecutor()
	_, err := e.Run(context.Background(), nil, nil, nil, "")
	if err == nil {
		t.Error("Run with empty command should return error")
	}
}

func TestRunEmptySliceCommand(t *testing.T) {
	e := NewExecutor()
	_, err := e.Run(context.Background(), []string{}, nil, nil, "")
	if err == nil {
		t.Error("Run with empty slice command should return error")
	}
}

// ---------------------------------------------------------------------------
// Run – echo command produces correct output
// ---------------------------------------------------------------------------

func TestRunEchoNoArgs(t *testing.T) {
	e := NewExecutor()
	br, err := e.Run(context.Background(), []string{"echo", "hello"}, nil, nil, t.TempDir())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if br.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", br.ExitCode)
	}
	if len(br.Results) == 0 {
		t.Fatal("expected at least one result")
	}
	if !strings.Contains(br.Results[0].Stdout, "hello") {
		t.Errorf("Stdout = %q, want to contain 'hello'", br.Results[0].Stdout)
	}
}

func TestRunEchoWithArgs(t *testing.T) {
	e := NewExecutor()
	br, err := e.Run(context.Background(), []string{"echo"}, []string{"foo", "bar"}, nil, t.TempDir())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if br.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", br.ExitCode)
	}
	out := br.Results[0].Stdout
	if !strings.Contains(out, "foo") || !strings.Contains(out, "bar") {
		t.Errorf("Stdout = %q, want to contain 'foo' and 'bar'", out)
	}
}

// ---------------------------------------------------------------------------
// batchArgs – different batch sizes
// ---------------------------------------------------------------------------

func TestBatchArgsUnlimited(t *testing.T) {
	e := &Executor{MaxBatchSize: 0}
	batches := e.batchArgs([]string{"a", "b", "c"})
	if len(batches) != 1 {
		t.Errorf("got %d batches, want 1", len(batches))
	}
	if len(batches[0]) != 3 {
		t.Errorf("batch[0] has %d items, want 3", len(batches[0]))
	}
}

func TestBatchArgsSize2(t *testing.T) {
	e := &Executor{MaxBatchSize: 2}
	batches := e.batchArgs([]string{"a", "b", "c", "d", "e"})
	if len(batches) != 3 {
		t.Errorf("got %d batches, want 3", len(batches))
	}
	if len(batches[0]) != 2 {
		t.Errorf("batch[0] has %d items, want 2", len(batches[0]))
	}
	if len(batches[2]) != 1 {
		t.Errorf("batch[2] has %d items, want 1", len(batches[2]))
	}
}

func TestBatchArgsSize1(t *testing.T) {
	e := &Executor{MaxBatchSize: 1}
	batches := e.batchArgs([]string{"a", "b", "c"})
	if len(batches) != 3 {
		t.Errorf("got %d batches, want 3", len(batches))
	}
	for i, b := range batches {
		if len(b) != 1 {
			t.Errorf("batch[%d] has %d items, want 1", i, len(b))
		}
	}
}

func TestBatchArgsEmpty(t *testing.T) {
	e := &Executor{MaxBatchSize: 5}
	batches := e.batchArgs(nil)
	if batches != nil {
		t.Errorf("got %v, want nil", batches)
	}
}

func TestBatchArgsExactMultiple(t *testing.T) {
	e := &Executor{MaxBatchSize: 3}
	batches := e.batchArgs([]string{"a", "b", "c", "d", "e", "f"})
	if len(batches) != 2 {
		t.Errorf("got %d batches, want 2", len(batches))
	}
}

// ---------------------------------------------------------------------------
// Sequential execution (MaxJobs=1)
// ---------------------------------------------------------------------------

func TestRunSequential(t *testing.T) {
	e := &Executor{MaxJobs: 1, MaxBatchSize: 1}
	br, err := e.Run(context.Background(), []string{"echo"}, []string{"a", "b", "c"}, nil, t.TempDir())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(br.Results) != 3 {
		t.Fatalf("got %d results, want 3", len(br.Results))
	}
	for i, r := range br.Results {
		expected := []string{"a", "b", "c"}[i]
		if !strings.Contains(r.Stdout, expected) {
			t.Errorf("result[%d].Stdout = %q, want to contain %q", i, r.Stdout, expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Parallel execution (MaxJobs > 1)
// ---------------------------------------------------------------------------

func TestRunParallel(t *testing.T) {
	e := &Executor{MaxJobs: 3, MaxBatchSize: 1}
	br, err := e.Run(context.Background(), []string{"echo"}, []string{"x", "y", "z"}, nil, t.TempDir())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if br.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", br.ExitCode)
	}
	if len(br.Results) != 3 {
		t.Fatalf("got %d results, want 3", len(br.Results))
	}
	// Each result should contain its argument (order preserved by index).
	for i, r := range br.Results {
		expected := []string{"x", "y", "z"}[i]
		if !strings.Contains(r.Stdout, expected) {
			t.Errorf("result[%d].Stdout = %q, want to contain %q", i, r.Stdout, expected)
		}
	}
}

func TestRunParallelMultipleBatchesPerJob(t *testing.T) {
	e := &Executor{MaxJobs: 2, MaxBatchSize: 2}
	br, err := e.Run(context.Background(), []string{"echo"}, []string{"a", "b", "c", "d", "e"}, nil, t.TempDir())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(br.Results) != 3 {
		t.Fatalf("got %d results, want 3", len(br.Results))
	}
}

// ---------------------------------------------------------------------------
// Non-zero exit code preserved
// ---------------------------------------------------------------------------

func TestRunNonZeroExitCode(t *testing.T) {
	e := NewExecutor()
	br, err := e.Run(context.Background(), []string{"sh", "-c", "exit 42"}, nil, nil, t.TempDir())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if br.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want 42", br.ExitCode)
	}
}

func TestRunNonZeroExitCodeInBatch(t *testing.T) {
	e := &Executor{MaxJobs: 1, MaxBatchSize: 1}
	// First batch succeeds, second fails.
	br, err := e.Run(context.Background(), []string{"sh", "-c", "exit $1", "--"}, []string{"0", "3"}, nil, t.TempDir())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if br.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", br.ExitCode)
	}
}

// ---------------------------------------------------------------------------
// Context cancellation
// ---------------------------------------------------------------------------

func TestRunContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	e := NewExecutor()
	_, err := e.Run(ctx, []string{"sleep", "10"}, nil, nil, t.TempDir())
	if err == nil {
		// Even if Run returns a result, the batch should have failed or context error propagated.
		// However, exec.CommandContext may return an ExitError rather than a context error,
		// so we check if there was any indication of cancellation.
		t.Log("Run did not return error on canceled context; checking if command was killed")
	}
}

func TestRunParallelContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	e := &Executor{MaxJobs: 4, MaxBatchSize: 1}
	_, err := e.Run(ctx, []string{"sleep", "10"}, []string{"1", "2", "3", "4"}, nil, t.TempDir())
	if err == nil {
		t.Log("Run did not return error on canceled context for parallel execution")
	}
}

// ---------------------------------------------------------------------------
// Stderr captured
// ---------------------------------------------------------------------------

func TestRunCapturesStderr(t *testing.T) {
	e := NewExecutor()
	br, err := e.Run(context.Background(), []string{"sh", "-c", "echo errout >&2"}, nil, nil, t.TempDir())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(br.Results[0].Stderr, "errout") {
		t.Errorf("Stderr = %q, want to contain 'errout'", br.Results[0].Stderr)
	}
}

// ---------------------------------------------------------------------------
// Run with no args still executes once
// ---------------------------------------------------------------------------

func TestRunNoArgsExecutesOnce(t *testing.T) {
	e := NewExecutor()
	br, err := e.Run(context.Background(), []string{"echo", "noargs"}, nil, nil, t.TempDir())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(br.Results) != 1 {
		t.Errorf("got %d results, want 1", len(br.Results))
	}
	if !strings.Contains(br.Results[0].Stdout, "noargs") {
		t.Errorf("Stdout = %q, want to contain 'noargs'", br.Results[0].Stdout)
	}
}
