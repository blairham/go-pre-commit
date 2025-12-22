// Package xargs provides parallel file argument execution similar to Python pre-commit's run_xargs.
// It partitions file arguments into batches and runs commands in parallel, respecting
// platform command-line length limits and concurrency settings.
package xargs

import (
	"bytes"
	"hash/fnv"
	"math"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
)

const (
	// FixedRandomSeed is used for deterministic shuffling of file arguments
	// Matches Python pre-commit's FIXED_RANDOM_SEED = 1542676187
	FixedRandomSeed = 1542676187

	// MinMaxLength is the minimum max command length (POSIX minimum)
	MinMaxLength = 4096 // 2^12

	// DefaultMaxLength is the default max command length on unknown platforms
	DefaultMaxLength = MinMaxLength

	// WindowsMaxLength is the UNICODE_STRING max minus headroom on Windows
	WindowsMaxLength = 32768 - 2048 // 2^15 - 2048

	// WindowsBatchMaxLength is the max length for batch files on Windows
	WindowsBatchMaxLength = 8192 - 1024

	// MinPartitionArgs is the minimum arguments per partition to avoid tiny partitions
	MinPartitionArgs = 4
)

// Result represents the result of running xargs
type Result struct {
	ExitCode int
	Output   []byte
}

// Config holds configuration for xargs execution
type Config struct {
	// TargetConcurrency is the target number of parallel processes
	TargetConcurrency int
	// Color indicates whether to use pseudo-terminal for color output
	Color bool
	// MaxLength overrides the platform max command length (for testing)
	MaxLength int
	// RequireSerial forces serial execution (no shuffling, no parallelism)
	RequireSerial bool
	// Env contains additional environment variables
	Env map[string]string
}

// CPUCount returns the number of available CPUs
// This matches Python pre-commit's xargs.cpu_count()
func CPUCount() int {
	return runtime.NumCPU()
}

// TargetConcurrency returns the target concurrency based on environment variables
// This matches Python pre-commit's lang_base.target_concurrency()
func TargetConcurrency() int {
	// Check PRE_COMMIT_NO_CONCURRENCY
	if os.Getenv("PRE_COMMIT_NO_CONCURRENCY") != "" {
		return 1
	}

	// Travis has many CPUs but we can't use them all
	if os.Getenv("TRAVIS") != "" {
		return 2
	}

	return CPUCount()
}

// GetPlatformMaxLength returns the maximum command line length for the current platform
// This matches Python pre-commit's xargs._get_platform_max_length()
func GetPlatformMaxLength() int {
	switch runtime.GOOS {
	case "windows":
		return WindowsMaxLength
	default:
		// POSIX systems
		return getPosixMaxLength()
	}
}

// getPosixMaxLength gets the max command length on POSIX systems
func getPosixMaxLength() int {
	// Try to get SC_ARG_MAX via stack limit estimation
	var rlim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_STACK, &rlim); err == nil {
		// Use a reasonable estimate based on stack limit
		// ARG_MAX is typically 1/4 of stack limit on many systems
		argMax := int(rlim.Max / 4)
		if argMax > 0 {
			// Subtract headroom for environment variables
			envSize := environSize()
			maxLength := argMax - 2048 - envSize
			// Clamp to reasonable bounds
			if maxLength > 1<<17 {
				maxLength = 1 << 17 // 128KB max
			}
			if maxLength < MinMaxLength {
				maxLength = MinMaxLength
			}
			return maxLength
		}
	}

	// Fallback to POSIX minimum
	return MinMaxLength
}

// environSize calculates the approximate size of environment variables
func environSize() int {
	env := os.Environ()
	size := 8 * len(env) // pointer size for each entry
	for _, e := range env {
		size += len(e) + 1 // string + null terminator
	}
	return size
}

// CommandLength calculates the length a command takes on the command line
// This matches Python pre-commit's xargs._command_length()
func CommandLength(cmd ...string) int {
	fullCmd := strings.Join(cmd, " ")

	if runtime.GOOS == "windows" {
		// Windows uses UTF-16 character count
		utf16Chars := utf16.Encode([]rune(fullCmd))
		return len(utf16Chars)
	}

	// Unix uses byte length in filesystem encoding
	return len(fullCmd)
}

// Partition divides file arguments into batches that fit within command line limits
// This matches Python pre-commit's xargs.partition()
func Partition(cmd []string, varargs []string, targetConcurrency int, maxLength int) [][]string {
	if maxLength <= 0 {
		maxLength = GetPlatformMaxLength()
	}

	// Calculate max args per partition to enable parallelism
	maxArgs := MinPartitionArgs
	if targetConcurrency > 0 && len(varargs) > 0 {
		maxArgs = int(math.Ceil(float64(len(varargs)) / float64(targetConcurrency)))
		if maxArgs < MinPartitionArgs {
			maxArgs = MinPartitionArgs
		}
	}

	var partitions [][]string
	var currentPartition []string
	baseLength := CommandLength(cmd...) + 1

	totalLength := baseLength
	for _, arg := range varargs {
		argLength := CommandLength(arg) + 1

		// Check if we need to start a new partition
		if (totalLength+argLength > maxLength || len(currentPartition) >= maxArgs) && len(currentPartition) > 0 {
			// Save current partition
			partition := make([]string, len(cmd)+len(currentPartition))
			copy(partition, cmd)
			copy(partition[len(cmd):], currentPartition)
			partitions = append(partitions, partition)

			// Start new partition
			currentPartition = nil
			totalLength = baseLength
		}

		// Add argument to current partition
		currentPartition = append(currentPartition, arg)
		totalLength += argLength
	}

	// Add final partition if not empty
	if len(currentPartition) > 0 {
		partition := make([]string, len(cmd)+len(currentPartition))
		copy(partition, cmd)
		copy(partition[len(cmd):], currentPartition)
		partitions = append(partitions, partition)
	}

	// If no partitions created, add one with just the command
	if len(partitions) == 0 {
		partitions = append(partitions, cmd)
	}

	return partitions
}

// Shuffled returns a deterministically shuffled copy of the slice
// This matches Python pre-commit's lang_base._shuffled()
func Shuffled(seq []string) []string {
	if len(seq) == 0 {
		return seq
	}

	// Create a copy
	result := make([]string, len(seq))
	copy(result, seq)

	// Use deterministic shuffling based on hash
	// This isn't exactly the same as Python's Random.shuffle with seed,
	// but it's deterministic and provides good distribution
	sort.Slice(result, func(i, j int) bool {
		hi := hashString(result[i], FixedRandomSeed)
		hj := hashString(result[j], FixedRandomSeed)
		if hi != hj {
			return hi < hj
		}
		// Fallback to lexicographic for stability
		return result[i] < result[j]
	})

	return result
}

// hashString creates a deterministic hash for shuffling
func hashString(s string, seed int) uint64 {
	h := fnv.New64a()
	// Mix in the seed
	seedBytes := []byte{
		byte(seed >> 24), byte(seed >> 16),
		byte(seed >> 8), byte(seed),
	}
	h.Write(seedBytes)
	h.Write([]byte(s))
	return h.Sum64()
}

// Run executes a command with file arguments using xargs-style batching
// This is the main entry point matching Python pre-commit's xargs.xargs()
func Run(cmd []string, fileArgs []string, cfg *Config) Result {
	if cfg == nil {
		cfg = &Config{
			TargetConcurrency: TargetConcurrency(),
		}
	}

	// Determine concurrency and shuffling
	targetConcurrency := cfg.TargetConcurrency
	files := fileArgs

	if cfg.RequireSerial {
		targetConcurrency = 1
	} else if targetConcurrency <= 0 {
		targetConcurrency = TargetConcurrency()
	}

	// Shuffle files for better load balancing (unless serial)
	if !cfg.RequireSerial && targetConcurrency > 1 {
		files = Shuffled(fileArgs)
	}

	// Get max length
	maxLength := cfg.MaxLength
	if maxLength <= 0 {
		maxLength = GetPlatformMaxLength()

		// Handle Windows batch files
		if runtime.GOOS == "windows" && len(cmd) > 0 {
			lower := strings.ToLower(cmd[0])
			if strings.HasSuffix(lower, ".bat") || strings.HasSuffix(lower, ".cmd") {
				maxLength = WindowsBatchMaxLength
			}
		}
	}

	// Partition the file arguments
	partitions := Partition(cmd, files, targetConcurrency, maxLength)

	// Run partitions
	threads := len(partitions)
	if threads > targetConcurrency {
		threads = targetConcurrency
	}

	return runPartitions(partitions, threads, cfg)
}

// runPartitions executes command partitions in parallel or serial
func runPartitions(partitions [][]string, threads int, cfg *Config) Result {
	if threads <= 1 || len(partitions) <= 1 {
		return runPartitionsSerial(partitions, cfg)
	}
	return runPartitionsParallel(partitions, threads, cfg)
}

// runPartitionsSerial runs partitions one at a time
func runPartitionsSerial(partitions [][]string, cfg *Config) Result {
	var result Result
	var output bytes.Buffer

	for _, partition := range partitions {
		partResult := runSinglePartition(partition, cfg)
		output.Write(partResult.Output)

		// Keep track of the maximum (by absolute value) exit code
		if abs(partResult.ExitCode) > abs(result.ExitCode) {
			result.ExitCode = partResult.ExitCode
		}
	}

	result.Output = output.Bytes()
	return result
}

// runPartitionsParallel runs partitions concurrently
func runPartitionsParallel(partitions [][]string, threads int, cfg *Config) Result {
	type partitionResult struct {
		index    int
		exitCode int
		output   []byte
	}

	results := make(chan partitionResult, len(partitions))
	sem := make(chan struct{}, threads)
	var wg sync.WaitGroup

	for i, partition := range partitions {
		wg.Add(1)
		go func(idx int, part []string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			r := runSinglePartition(part, cfg)
			results <- partitionResult{
				index:    idx,
				exitCode: r.ExitCode,
				output:   r.Output,
			}
		}(i, partition)
	}

	// Wait for all goroutines then close channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results in order
	resultsByIndex := make(map[int]partitionResult)
	for r := range results {
		resultsByIndex[r.index] = r
	}

	// Combine results in original order
	var result Result
	var output bytes.Buffer

	for i := 0; i < len(partitions); i++ {
		r := resultsByIndex[i]
		output.Write(r.output)
		if abs(r.exitCode) > abs(result.ExitCode) {
			result.ExitCode = r.exitCode
		}
	}

	result.Output = output.Bytes()
	return result
}

// runSinglePartition executes a single command partition
func runSinglePartition(partition []string, cfg *Config) Result {
	if len(partition) == 0 {
		return Result{}
	}

	cmd := exec.Command(partition[0], partition[1:]...)

	// Set up environment
	if len(cfg.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range cfg.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	// Run and capture output
	output, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return Result{
		ExitCode: exitCode,
		Output:   output,
	}
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
