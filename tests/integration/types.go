package integration

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"sync"
	"time"

	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

const (
	// ScriptLanguage represents the script language type for testing
	ScriptLanguage = "script"
)

const (
	// CheckMark represents a successful test result in output
	CheckMark = "✅"
	// CrossMark represents a failed test result in output
	CrossMark = "❌"

	// LangPython represents the Python language identifier
	LangPython = "python"
	// LangNode represents the Node.js language identifier
	LangNode = "node"
	// LangGolang represents the Go language identifier
	LangGolang = "golang"
	// LangRuby represents the Ruby language identifier
	LangRuby = "ruby"
	// LangRust represents the Rust language identifier
	LangRust = "rust"
)

// LanguageCompatibilityTest represents a comprehensive test for language compatibility
type LanguageCompatibilityTest struct {
	PythonPrecommitBinary    string
	Language                 string
	TestRepository           string
	TestCommit               string
	HookID                   string
	GoPrecommitBinary        string
	Name                     string
	ExpectedFiles            []string
	TestVersions             []string
	AdditionalDependencies   []string
	TestTimeout              time.Duration
	NeedsRuntimeInstalled    bool
	CacheTestEnabled         bool
	BiDirectionalTestEnabled bool
}

// TestResults holds the results of compatibility testing
type TestResults struct {
	Timestamp             time.Time     `json:"timestamp"`
	Language              string        `json:"language"`
	TestRepository        string        `json:"test_repository"`
	HookID                string        `json:"hook_id"`
	Errors                []string      `json:"errors,omitempty"`
	Warnings              []string      `json:"warnings,omitempty"`
	PythonCacheEfficiency float64       `json:"python_cache_efficiency"`
	PythonInstallTime     time.Duration `json:"python_install_time"`
	GoCacheEfficiency     float64       `json:"go_cache_efficiency"`
	GoInstallTime         time.Duration `json:"go_install_time"`
	PerformanceRatio      float64       `json:"performance_ratio"`
	TestDuration          time.Duration `json:"test_duration"`
	FunctionalEquivalence bool          `json:"functional_equivalence"`
	CacheBidirectional    bool          `json:"cache_bidirectional"`
	EnvironmentIsolation  bool          `json:"environment_isolation"`
	VersionManagement     bool          `json:"version_management"`
	Success               bool          `json:"success"`
}

// AddError adds an error message to the test results
func (tr *TestResults) AddError(err string) {
	tr.Errors = append(tr.Errors, err)
	tr.Success = false
}

// AddWarning adds a warning message to the test results
func (tr *TestResults) AddWarning(warning string) {
	tr.Warnings = append(tr.Warnings, warning)
}

// AddErrorf adds a formatted error message to the test results
func (tr *TestResults) AddErrorf(format string, args ...interface{}) {
	tr.AddError(fmt.Sprintf(format, args...))
}

// AddWarningf adds a formatted warning message to the test results
func (tr *TestResults) AddWarningf(format string, args ...interface{}) {
	tr.AddWarning(fmt.Sprintf(format, args...))
}

// roundToDecimalPlaces rounds a float64 to the specified number of decimal places
func roundToDecimalPlaces(value float64, decimals int) float64 {
	multiplier := math.Pow(10, float64(decimals))
	return math.Round(value*multiplier) / multiplier
}

// MarshalJSON provides custom JSON marshaling for TestResults
// This converts time.Duration fields from nanoseconds to milliseconds for consistency
func (tr *TestResults) MarshalJSON() ([]byte, error) {
	type Alias TestResults
	return json.Marshal(&struct {
		*Alias
		PythonInstallTimeMs   float64 `json:"python_install_time"`
		GoInstallTimeMs       float64 `json:"go_install_time"`
		TestDurationMs        float64 `json:"test_duration"`
		PerformanceRatio      float64 `json:"performance_ratio"`
		PythonCacheEfficiency float64 `json:"python_cache_efficiency"`
		GoCacheEfficiency     float64 `json:"go_cache_efficiency"`
	}{
		Alias:                 (*Alias)(tr),
		PythonInstallTimeMs:   roundToDecimalPlaces(float64(tr.PythonInstallTime.Nanoseconds())/1e6, 2),
		GoInstallTimeMs:       roundToDecimalPlaces(float64(tr.GoInstallTime.Nanoseconds())/1e6, 2),
		TestDurationMs:        roundToDecimalPlaces(float64(tr.TestDuration.Nanoseconds())/1e6, 2),
		PerformanceRatio:      roundToDecimalPlaces(tr.PerformanceRatio, 1),
		PythonCacheEfficiency: roundToDecimalPlaces(tr.PythonCacheEfficiency, 1),
		GoCacheEfficiency:     roundToDecimalPlaces(tr.GoCacheEfficiency, 1),
	})
}

// Suite manages the comprehensive language testing
type Suite struct {
	registry     *languages.LanguageRegistry
	results      map[string]*TestResults
	cache        map[string]bool
	pythonBinary string
	goBinary     string
	testDataDir  string
	outputDir    string
	resultsMutex sync.RWMutex
}

// NewSuite creates a new language test suite
func NewSuite(pythonBinary, goBinary, testDataDir, outputDir string) *Suite {
	return &Suite{
		registry:     languages.NewLanguageRegistry(),
		pythonBinary: pythonBinary,
		goBinary:     goBinary,
		testDataDir:  testDataDir,
		outputDir:    outputDir,
		results:      make(map[string]*TestResults),
		cache:        make(map[string]bool),
	}
}

// CommandDiagnostics holds detailed information about command execution
type CommandDiagnostics struct {
	Start    time.Time     `json:"start"`
	Command  string        `json:"command"`
	Dir      string        `json:"dir"`
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
	ExitCode int           `json:"exit_code"`
}

// InstallHooksResult holds the results of install-hooks command execution
type InstallHooksResult struct {
	CacheStructure      map[string][]string
	Stdout              string
	Stderr              string
	RepositoriesFound   []string
	EnvironmentsCreated []string
	DatabaseEntries     []string
	ExecutionTime       time.Duration
	ExitCode            int
}

// GetResults returns a copy of the current results
func (s *Suite) GetResults() map[string]*TestResults {
	s.resultsMutex.RLock()
	defer s.resultsMutex.RUnlock()

	results := make(map[string]*TestResults)
	maps.Copy(results, s.results)
	return results
}

// SetResults sets the results map
func (s *Suite) SetResults(results map[string]*TestResults) {
	s.resultsMutex.Lock()
	defer s.resultsMutex.Unlock()
	s.results = results
}

// AddResult adds a test result to the suite
func (s *Suite) AddResult(result *TestResults) {
	s.resultsMutex.Lock()
	defer s.resultsMutex.Unlock()
	s.results[result.Language] = result
}
