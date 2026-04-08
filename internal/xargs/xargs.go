// Package xargs provides batched parallel command execution similar to xargs.
package xargs

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Result holds the output and exit code of a single batch execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// BatchResult aggregates results from all batches.
type BatchResult struct {
	Results  []Result
	ExitCode int
}

// Executor runs commands in batches with optional parallelism.
type Executor struct {
	// MaxJobs is the maximum number of parallel executions (0 = no limit).
	MaxJobs int
	// MaxBatchSize is the maximum number of args per batch (0 = unlimited).
	MaxBatchSize int
	// TargetConcurrency sets the default concurrency if MaxJobs is 0.
	TargetConcurrency int
}

// DefaultMaxBatchSize returns the default maximum number of file arguments per batch.
// A value of 0 means unlimited (all files in one batch).
func DefaultMaxBatchSize() int {
	return 0
}

// NewExecutor creates an Executor with default settings.
func NewExecutor() *Executor {
	return &Executor{
		MaxJobs:           1,
		MaxBatchSize:      0,
		TargetConcurrency: 1,
	}
}

// Run executes a command with batched arguments.
// cmd is the base command, args are split into batches.
// env is additional environment variables, dir is the working directory.
func (e *Executor) Run(ctx context.Context, cmd []string, args []string, env []string, dir string) (*BatchResult, error) {
	if len(cmd) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	batches := e.batchArgs(args)
	if len(batches) == 0 {
		// No args, run once with no file arguments.
		batches = [][]string{{}}
	}

	jobs := e.MaxJobs
	if jobs <= 0 {
		jobs = e.TargetConcurrency
	}
	if jobs <= 0 {
		jobs = 1
	}

	// If serial or single batch, run sequentially.
	if jobs == 1 || len(batches) == 1 {
		return e.runSequential(ctx, cmd, batches, env, dir)
	}

	return e.runParallel(ctx, cmd, batches, env, dir, jobs)
}

func (e *Executor) batchArgs(args []string) [][]string {
	if len(args) == 0 {
		return nil
	}

	batchSize := e.MaxBatchSize
	if batchSize <= 0 {
		// Default: all args in one batch.
		return [][]string{args}
	}

	var batches [][]string
	for i := 0; i < len(args); i += batchSize {
		end := i + batchSize
		if end > len(args) {
			end = len(args)
		}
		batch := make([]string, end-i)
		copy(batch, args[i:end])
		batches = append(batches, batch)
	}
	return batches
}

func (e *Executor) runSequential(ctx context.Context, cmd []string, batches [][]string, env []string, dir string) (*BatchResult, error) {
	br := &BatchResult{}

	for _, batch := range batches {
		result, err := e.runBatch(ctx, cmd, batch, env, dir)
		if err != nil {
			return nil, err
		}
		br.Results = append(br.Results, *result)
		if result.ExitCode != 0 {
			br.ExitCode = result.ExitCode
		}
	}

	return br, nil
}

func (e *Executor) runParallel(ctx context.Context, cmd []string, batches [][]string, env []string, dir string, jobs int) (*BatchResult, error) {
	br := &BatchResult{
		Results: make([]Result, len(batches)),
	}

	sem := make(chan struct{}, jobs)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error

	for i, batch := range batches {
		wg.Add(1)
		go func(idx int, b []string) {
			defer wg.Done()

			// Acquire semaphore.
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				if firstErr == nil {
					firstErr = ctx.Err()
				}
				mu.Unlock()
				return
			}

			result, err := e.runBatch(ctx, cmd, b, env, dir)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			br.Results[idx] = *result
			if result.ExitCode != 0 {
				br.ExitCode = result.ExitCode
			}
		}(i, batch)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return br, nil
}

func (e *Executor) runBatch(ctx context.Context, cmdParts []string, args []string, env []string, dir string) (*Result, error) {
	fullArgs := make([]string, 0, len(cmdParts)-1+len(args))
	fullArgs = append(fullArgs, cmdParts[1:]...)
	fullArgs = append(fullArgs, args...)

	cmd := exec.CommandContext(ctx, cmdParts[0], fullArgs...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := &Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("running command %s: %w", strings.Join(cmdParts, " "), err)
		}
	}

	return result, nil
}
