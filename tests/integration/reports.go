package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ReportGenerator handles saving results and generating reports
type ReportGenerator struct {
	suite *Suite
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(suite *Suite) *ReportGenerator {
	return &ReportGenerator{suite: suite}
}

// SaveResults saves test results to JSON file
func (rg *ReportGenerator) SaveResults() error {
	rg.suite.resultsMutex.RLock()
	defer rg.suite.resultsMutex.RUnlock()

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(rg.suite.outputDir, 0o750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Save individual results
	for language, result := range rg.suite.results {
		filename := filepath.Join(rg.suite.outputDir, fmt.Sprintf("%s_test_results.json", language))
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal results for %s: %w", language, err)
		}

		if err := os.WriteFile(filename, data, 0o600); err != nil {
			return fmt.Errorf("failed to write results file for %s: %w", language, err)
		}
	}

	// Save summary results
	summaryFilename := filepath.Join(rg.suite.outputDir, "test_results_summary.json")
	summaryData, err := json.MarshalIndent(rg.suite.results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal summary results: %w", err)
	}

	if err := os.WriteFile(summaryFilename, summaryData, 0o600); err != nil {
		return fmt.Errorf("failed to write summary results file: %w", err)
	}

	return nil
}

// GenerateReport generates a human-readable test report
//
//nolint:gocognit,cyclop,gocyclo,funlen // Complex report generation logic - acceptable for formatting
func (rg *ReportGenerator) GenerateReport() error {
	rg.suite.resultsMutex.RLock()
	defer rg.suite.resultsMutex.RUnlock()

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(rg.suite.outputDir, 0o750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	reportFilename := filepath.Join(rg.suite.outputDir, "compatibility_test_report.md")

	var report strings.Builder
	report.WriteString("# Language Compatibility Test Report\n\n")
	report.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339)))

	// Summary statistics
	totalTests := len(rg.suite.results)
	successfulTests := 0
	totalGoTime := time.Duration(0)
	totalPythonTime := time.Duration(0)

	for _, result := range rg.suite.results {
		if result.Success {
			successfulTests++
		}
		totalGoTime += result.GoInstallTime
		totalPythonTime += result.PythonInstallTime
	}

	report.WriteString("## Summary\n\n")
	report.WriteString(fmt.Sprintf("- Total Languages Tested: %d\n", totalTests))
	report.WriteString(fmt.Sprintf("- Successful Tests: %d\n", successfulTests))
	report.WriteString(
		fmt.Sprintf("- Success Rate: %.1f%%\n", float64(successfulTests)/float64(totalTests)*100),
	)
	report.WriteString(fmt.Sprintf("- Total Go Install Time: %v\n", totalGoTime))
	if totalPythonTime > 0 {
		report.WriteString(fmt.Sprintf("- Total Python Install Time: %v\n", totalPythonTime))
		report.WriteString(
			fmt.Sprintf(
				"- Performance Improvement: %.2fx\n",
				float64(totalPythonTime)/float64(totalGoTime),
			),
		)
	}
	report.WriteString("\n")

	// Detailed results
	report.WriteString("## Detailed Results\n\n")
	report.WriteString(
		"| Language | Status | Install Time | Cache Efficiency | Functional | Bidirectional | Environment |\n",
	)
	report.WriteString(
		"|----------|--------|--------------|-----------|------------|---------------|-------------|\n",
	)

	for _, result := range rg.suite.results {
		status := CrossMark
		if result.Success {
			status = CheckMark
		}

		funcEquiv := CrossMark
		if result.FunctionalEquivalence {
			funcEquiv = CheckMark
		}

		biCache := CrossMark
		if result.CacheBidirectional {
			biCache = CheckMark
		}

		envIso := CrossMark
		if result.EnvironmentIsolation {
			envIso = CheckMark
		}

		// Format install time to be more readable
		installTime := result.GoInstallTime
		var timeStr string
		switch {
		case installTime >= time.Second:
			timeStr = fmt.Sprintf("%.2fs", installTime.Seconds())
		case installTime >= time.Millisecond:
			timeStr = fmt.Sprintf("%.1fms", float64(installTime.Nanoseconds())/1e6)
		default:
			timeStr = fmt.Sprintf("%.0fμs", float64(installTime.Nanoseconds())/1e3)
		}

		report.WriteString(fmt.Sprintf("| %s | %s | %s | %.1f%% | %s | %s | %s |\n",
			result.Language, status, timeStr, result.GoCacheEfficiency,
			funcEquiv, biCache, envIso))
	}

	report.WriteString("\n## Error Details\n\n")
	for _, result := range rg.suite.results {
		if len(result.Errors) > 0 {
			report.WriteString(fmt.Sprintf("### %s\n", result.Language))
			for _, err := range result.Errors {
				report.WriteString(fmt.Sprintf("- %s\n", err))
			}
			report.WriteString("\n")
		}
	}

	if err := os.WriteFile(reportFilename, []byte(report.String()), 0o600); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	return nil
}
