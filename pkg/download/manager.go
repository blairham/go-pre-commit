// Package download provides unified download and archive extraction utilities
package download

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/blairham/go-pre-commit/pkg/constants"
)

// Manager provides download and extraction capabilities
type Manager struct {
	client  *http.Client
	timeout time.Duration
	verbose bool
}

// NewManager creates a new download manager
func NewManager() *Manager {
	// Check for debug/verbose environment variable
	verbose := os.Getenv("DEBUG") != "" || os.Getenv("VERBOSE") != ""

	return &Manager{
		timeout: 30 * time.Second,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		verbose: verbose,
	}
}

// WithTimeout sets the download timeout
func (m *Manager) WithTimeout(timeout time.Duration) *Manager {
	m.timeout = timeout
	m.client.Timeout = timeout
	return m
}

// WithVerbose sets the verbose mode for download messages
func (m *Manager) WithVerbose(verbose bool) *Manager {
	m.verbose = verbose
	return m
}

// GetNormalizedOS returns normalized OS name for downloads
func (m *Manager) GetNormalizedOS() string {
	switch runtime.GOOS {
	case constants.DarwinOS:
		return "osx"
	case constants.WindowsOS:
		return "win"
	case constants.LinuxOS:
		return "linux"
	default:
		return runtime.GOOS
	}
}

// GetNormalizedArch returns normalized architecture name for downloads
func (m *Manager) GetNormalizedArch() string {
	switch runtime.GOARCH {
	case constants.ArchAMD64:
		return "x64"
	case constants.ArchARM64:
		return "arm64"
	case constants.Arch386:
		return "x86"
	default:
		return runtime.GOARCH
	}
}

// DownloadFile downloads a file from URL to destination
func (m *Manager) DownloadFile(url, dest string) error {
	return m.DownloadFileWithContext(context.Background(), url, dest)
}

// DownloadFileWithContext downloads a file from URL to destination with context
func (m *Manager) DownloadFileWithContext(ctx context.Context, url, dest string) error {
	if m.verbose {
		fmt.Printf("[INFO] Downloading from %s...\n", url)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, "GET", url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", url, err)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download from %s: %w", url, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d for %s", resp.StatusCode, url)
	}

	// Create destination directory if it doesn't exist
	if dirErr := os.MkdirAll(filepath.Dir(dest), 0o750); dirErr != nil {
		return fmt.Errorf("failed to create directory for %s: %w", dest, dirErr)
	}

	// Create destination file
	file, err := os.Create(dest) // #nosec G304 -- downloading to user-specified destination
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", dest, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close file: %v\n", closeErr)
		}
	}()

	// Copy with basic progress indication
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", dest, err)
	}

	if m.verbose {
		fmt.Printf("[INFO] Downloaded successfully to %s\n", dest)
	}
	return nil
}

// DownloadAndExtract downloads and extracts an archive in one operation
func (m *Manager) DownloadAndExtract(url, destDir string) error {
	// Determine file extension from URL
	urlPath := strings.TrimSuffix(url, "/")
	var ext string
	switch {
	case strings.Contains(urlPath, ".tar.gz"):
		ext = ".tar.gz"
	case strings.Contains(urlPath, ".tgz"):
		ext = ".tgz"
	default:
		ext = filepath.Ext(urlPath)
	}

	// Create temporary file for download with proper extension
	tempFile, err := os.CreateTemp("", "download-*"+ext)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	defer func() { _ = os.Remove(tempPath) }() //nolint:errcheck // intentionally ignore cleanup error

	// Download to temp file
	if err := m.DownloadFile(url, tempPath); err != nil {
		return err
	}

	// Extract based on file extension
	archiver := NewArchiver()
	return archiver.Extract(tempPath, destDir)
}

// Archiver handles various archive formats
type Archiver struct{}

// NewArchiver creates a new archiver
func NewArchiver() *Archiver {
	return &Archiver{}
}

// Extract extracts an archive to destination directory
func (a *Archiver) Extract(archivePath, destDir string) error {
	ext := strings.ToLower(filepath.Ext(archivePath))

	switch {
	case strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz"):
		return a.ExtractTarGz(archivePath, destDir)
	case ext == ".zip":
		return a.ExtractZip(archivePath, destDir)
	case ext == ".tar":
		return a.ExtractTar(archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive format: %s", filepath.Ext(archivePath))
	}
}

// GetSupportedFormats returns supported archive formats
func (a *Archiver) GetSupportedFormats() []string {
	return []string{".tar.gz", ".tgz", ".zip", ".tar"}
}

// ExtractTarGz extracts a .tar.gz archive
func (a *Archiver) ExtractTarGz(archivePath, destDir string) error {
	fmt.Printf("[INFO] Extracting %s...\n", archivePath)

	tr, cleanup, err := a.openTarGzReader(archivePath)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := a.extractTarEntries(tr, destDir); err != nil {
		return err
	}

	fmt.Printf("[INFO] Extraction completed\n")
	return nil
}

// openTarGzReader opens a tar.gz file and returns a tar reader and cleanup function
func (a *Archiver) openTarGzReader(archivePath string) (*tar.Reader, func(), error) {
	file, err := os.Open(archivePath) // #nosec G304 -- opening downloaded archive for extraction
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open archive %s: %w", archivePath, err)
	}

	gzr, err := gzip.NewReader(file)
	if err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return nil, nil, fmt.Errorf(
				"failed to create gzip reader: %w (also failed to close file: %w)",
				err,
				closeErr,
			)
		}
		return nil, nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}

	tr := tar.NewReader(gzr)

	cleanup := func() {
		if closeErr := gzr.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close gzip reader: %v\n", closeErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close file: %v\n", closeErr)
		}
	}

	return tr, cleanup, nil
}

// ExtractTar extracts a .tar archive
func (a *Archiver) ExtractTar(archivePath, destDir string) error {
	fmt.Printf("[INFO] Extracting %s...\n", archivePath)

	file, err := os.Open(archivePath) // #nosec G304 -- path is validated by caller
	if err != nil {
		return fmt.Errorf("failed to open archive %s: %w", archivePath, err)
	}
	defer func() { _ = file.Close() }() //nolint:errcheck // intentionally ignore cleanup error

	tr := tar.NewReader(file)

	if err := a.extractTarEntries(tr, destDir); err != nil {
		return err
	}

	fmt.Printf("[INFO] Extraction completed\n")
	return nil
}

// ExtractZip extracts a .zip archive
func (a *Archiver) ExtractZip(archivePath, destDir string) error {
	fmt.Printf("[INFO] Extracting %s...\n", archivePath)

	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open zip archive %s: %w", archivePath, err)
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close zip reader: %v\n", closeErr)
		}
	}()

	if err := a.extractZipFiles(r.File, destDir); err != nil {
		return err
	}

	fmt.Printf("[INFO] Extraction completed\n")
	return nil
}

// extractTarEntries extracts all entries from a tar reader
func (a *Archiver) extractTarEntries(tr *tar.Reader, destDir string) error {
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		if err := a.extractTarEntry(tr, header, destDir); err != nil {
			return err
		}
	}
	return nil
}

// extractTarEntry extracts a single tar entry
func (a *Archiver) extractTarEntry(tr *tar.Reader, header *tar.Header, destDir string) error {
	path := filepath.Join(destDir, header.Name) // #nosec G305 -- extracting trusted archives

	// Security check: ensure path is within destDir
	if !a.isPathSafe(path, destDir) {
		return fmt.Errorf("invalid path in archive: %s", header.Name)
	}

	switch header.Typeflag {
	case tar.TypeDir:
		return a.createDirectory(path)
	case tar.TypeReg:
		return a.extractRegularFile(tr, header, path)
	}
	return nil
}

// extractZipFiles extracts all files from a zip archive
func (a *Archiver) extractZipFiles(files []*zip.File, destDir string) error {
	for _, f := range files {
		if err := a.extractZipFile(f, destDir); err != nil {
			return err
		}
	}
	return nil
}

// extractZipFile extracts a single file from a zip archive
func (a *Archiver) extractZipFile(f *zip.File, destDir string) error {
	path := filepath.Join(destDir, f.Name) // #nosec G305 -- extracting trusted archives

	// Security check: ensure path is within destDir
	if !a.isPathSafe(path, destDir) {
		return fmt.Errorf("invalid path in archive: %s", f.Name)
	}

	if f.FileInfo().IsDir() {
		return a.createZipDirectory(path, f.FileInfo().Mode())
	}

	return a.extractZipRegularFile(f, path)
}

// createZipDirectory creates a directory from zip file info
func (a *Archiver) createZipDirectory(path string, mode os.FileMode) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

// extractZipRegularFile extracts a regular file from zip
func (a *Archiver) extractZipRegularFile(f *zip.File, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("failed to create parent directory for %s: %w", path, err)
	}

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("failed to open file in archive: %s: %w", f.Name, err)
	}
	defer func() {
		if closeErr := rc.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close archive file: %v\n", closeErr)
		}
	}()

	outFile, err := os.Create(path) // #nosec G304 -- creating files during archive extraction
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer func() {
		if closeErr := outFile.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close output file: %v\n", closeErr)
		}
	}()

	// #nosec G110 - Legitimate zip extraction from trusted sources
	if _, err := io.Copy(outFile, rc); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	if err := os.Chmod(path, f.FileInfo().Mode()); err != nil {
		return fmt.Errorf("failed to set permissions for %s: %w", path, err)
	}

	return nil
}

// MakeBinaryExecutable makes a binary file executable (Unix-like systems)
func (m *Manager) MakeBinaryExecutable(path string) error {
	if runtime.GOOS == constants.WindowsOS {
		return nil // Windows doesn't need execute permissions
	}

	return os.Chmod(path, 0o700) // #nosec G302 -- executable permissions required for binary
}

// InstallBinary installs a binary to the environment bin directory
func (m *Manager) InstallBinary(srcPath, envPath, binaryName string) error {
	binDir := filepath.Join(envPath, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	destPath := filepath.Join(binDir, binaryName)
	if runtime.GOOS == constants.WindowsOS && !strings.HasSuffix(binaryName, ".exe") {
		destPath += ".exe"
	}

	// Copy the binary
	if err := m.copyFile(srcPath, destPath); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Make executable
	if err := m.MakeBinaryExecutable(destPath); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst
func (m *Manager) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src) // #nosec G304 -- copying from trusted source
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		if closeErr := sourceFile.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close source file: %v\n", closeErr)
		}
	}()

	destFile, err := os.Create(dst) // #nosec G304 -- creating in environment directory
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close destination file: %v\n", closeErr)
		}
	}()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	return nil
}

// isPathSafe checks if the extraction path is safe (within destination directory)
func (a *Archiver) isPathSafe(path, destDir string) bool {
	return strings.HasPrefix(path, filepath.Clean(destDir)+string(os.PathSeparator))
}

// createDirectory creates a directory with proper permissions
func (a *Archiver) createDirectory(path string) error {
	if err := os.MkdirAll(path, 0o750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

// extractRegularFile extracts a regular file from tar
func (a *Archiver) extractRegularFile(tr *tar.Reader, header *tar.Header, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("failed to create parent directory for %s: %w", path, err)
	}

	outFile, err := os.Create(path) // #nosec G304 -- creating files during archive extraction
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer func() {
		if closeErr := outFile.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close file: %v\n", closeErr)
		}
	}()

	// #nosec G110 - Legitimate tar extraction from trusted sources
	if _, err := io.Copy(outFile, tr); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	// Set file permissions
	if err := os.Chmod(path, os.FileMode(header.Mode)); err != nil {
		return fmt.Errorf("failed to set permissions for %s: %w", path, err)
	}

	return nil
}

// GetStatistics implements interfaces.StatisticsProvider
func (m *Manager) GetStatistics() map[string]any {
	return map[string]any{
		"timeout": m.timeout.String(),
	}
}
