// Package cache provides file-based locking functionality matching Python pre-commit
package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// FileLock represents a file-based lock using the same mechanism as Python pre-commit
type FileLock struct {
	file     *os.File
	lockPath string
}

// NewFileLock creates a new file lock for the given directory
// This matches Python pre-commit's file_lock.lock() function
func NewFileLock(cacheDir string) *FileLock {
	return &FileLock{
		lockPath: filepath.Join(cacheDir, ".lock"),
	}
}

// Lock acquires the file lock using fcntl.flock (same as Python pre-commit)
func (fl *FileLock) Lock(ctx context.Context) error {
	// Open the lock file in append mode (same as Python: 'a+')
	file, err := os.OpenFile(fl.lockPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}

	fl.file = file

	// Try non-blocking lock first (like Python's LOCK_EX | LOCK_NB)
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		// Got the lock immediately
		return nil
	}

	// If we couldn't get the lock immediately, check if context is canceled
	select {
	case <-ctx.Done():
		_ = fl.file.Close() //nolint:errcheck // Best effort close, context error is more important
		fl.file = nil
		return ctx.Err()
	default:
	}

	// Fall back to blocking lock (like Python's LOCK_EX)
	// Use a goroutine to make it cancellable
	done := make(chan error, 1)
	go func() {
		done <- syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
	}()

	select {
	case err := <-done:
		if err != nil {
			_ = fl.file.Close() //nolint:errcheck // Best effort close, flock error is more important
			fl.file = nil
			return fmt.Errorf("failed to acquire file lock: %w", err)
		}
		return nil
	case <-ctx.Done():
		_ = fl.file.Close() //nolint:errcheck // Best effort close, context error is more important
		fl.file = nil
		return ctx.Err()
	}
}

// Unlock releases the file lock
func (fl *FileLock) Unlock() error {
	if fl.file == nil {
		return nil
	}

	// Unlock the file (like Python's LOCK_UN)
	err := syscall.Flock(int(fl.file.Fd()), syscall.LOCK_UN)

	// Close the file
	if closeErr := fl.file.Close(); closeErr != nil && err == nil {
		err = closeErr
	}

	fl.file = nil
	return err
}

// WithLock executes a function while holding the file lock
// This matches Python pre-commit's context manager pattern
func (fl *FileLock) WithLock(ctx context.Context, fn func() error) error {
	if err := fl.Lock(ctx); err != nil {
		return err
	}
	defer func() {
		if unlockErr := fl.Unlock(); unlockErr != nil {
			fmt.Printf("⚠️  Warning: failed to unlock file: %v\n", unlockErr)
		}
	}()

	return fn()
}

// WithLockTimeout executes a function while holding the file lock with a timeout
func (fl *FileLock) WithLockTimeout(timeout time.Duration, fn func() error) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return fl.WithLock(ctx, fn)
}
