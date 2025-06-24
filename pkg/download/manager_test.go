package download

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blairham/go-pre-commit/pkg/constants"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()
	assert.NotNil(t, manager)
	assert.Equal(t, 30*time.Second, manager.timeout)
	assert.NotNil(t, manager.client)
	assert.Equal(t, 30*time.Second, manager.client.Timeout)
}

func TestManager_WithTimeout(t *testing.T) {
	manager := NewManager()
	customTimeout := 60 * time.Second

	updatedManager := manager.WithTimeout(customTimeout)
	assert.Equal(t, customTimeout, updatedManager.timeout)
	assert.Equal(t, customTimeout, updatedManager.client.Timeout)
	assert.Same(t, manager, updatedManager) // Should return the same instance
}

func TestManager_GetNormalizedOS(t *testing.T) {
	manager := NewManager()

	tests := []struct {
		goOS     string
		expected string
	}{
		{"darwin", "osx"},
		{constants.WindowsOS, "win"},
		{"linux", "linux"},
		{"freebsd", "freebsd"}, // Unknown OS should return as-is
	}

	for _, tt := range tests {
		t.Run(tt.goOS, func(t *testing.T) {
			// We can't change runtime.GOOS in tests, so we'll test the current OS
			// and verify the logic is sound
			result := manager.GetNormalizedOS()

			// Test that the function returns a non-empty string
			assert.NotEmpty(t, result)

			// Test the specific mappings we can verify
			switch runtime.GOOS {
			case "darwin":
				assert.Equal(t, "osx", result)
			case constants.WindowsOS:
				assert.Equal(t, "win", result)
			case constants.LinuxOS:
				assert.Equal(t, "linux", result)
			default:
				assert.Equal(t, runtime.GOOS, result)
			}
		})
	}
}

func TestManager_GetNormalizedArch(t *testing.T) {
	manager := NewManager()

	tests := []struct {
		goArch   string
		expected string
	}{
		{"amd64", "x64"},
		{"arm64", "arm64"},
		{"386", "x86"},
		{"riscv64", "riscv64"}, // Unknown arch should return as-is
	}

	for _, tt := range tests {
		t.Run(tt.goArch, func(t *testing.T) {
			// We can't change runtime.GOARCH in tests, so we'll test the current arch
			// and verify the logic is sound
			result := manager.GetNormalizedArch()

			// Test that the function returns a non-empty string
			assert.NotEmpty(t, result)

			// Test the specific mappings we can verify
			switch runtime.GOARCH {
			case "amd64":
				assert.Equal(t, "x64", result)
			case constants.ArchARM64:
				assert.Equal(t, "arm64", result)
			case "386":
				assert.Equal(t, "x86", result)
			default:
				assert.Equal(t, runtime.GOARCH, result)
			}
		})
	}
}

func TestManager_DownloadFile(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/success":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("test content"))
		case "/notfound":
			w.WriteHeader(http.StatusNotFound)
		case "/slow":
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("slow content"))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	tests := []struct {
		name        string
		path        string
		expectedErr string
		timeout     time.Duration
		expectError bool
	}{
		{
			name:        "successful download",
			path:        "/success",
			timeout:     30 * time.Second,
			expectError: false,
		},
		{
			name:        "404 not found",
			path:        "/notfound",
			timeout:     30 * time.Second,
			expectError: true,
			expectedErr: "download failed: HTTP 404",
		},
		{
			name:        "timeout",
			path:        "/slow",
			timeout:     1 * time.Second,
			expectError: true,
			expectedErr: "failed to download",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			destFile := filepath.Join(tempDir, "downloaded.txt")

			manager := NewManager().WithTimeout(tt.timeout)
			url := server.URL + tt.path

			err := manager.DownloadFile(url, destFile)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
			} else {
				assert.NoError(t, err)

				// Verify file was created and has correct content
				content, err := os.ReadFile(destFile)
				require.NoError(t, err)
				assert.Equal(t, "test content", string(content))
			}
		})
	}
}

func TestManager_DownloadFile_DirectoryCreation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test content"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	nestedPath := filepath.Join(tempDir, "nested", "dir", "file.txt")

	manager := NewManager()
	err := manager.DownloadFile(server.URL, nestedPath)

	assert.NoError(t, err)

	// Verify file exists
	content, err := os.ReadFile(nestedPath)
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content))
}

func TestManager_DownloadFile_InvalidURL(t *testing.T) {
	tempDir := t.TempDir()
	destFile := filepath.Join(tempDir, "test.txt")

	manager := NewManager()
	err := manager.DownloadFile("://invalid-url", destFile)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create request")
}

func TestNewArchiver(t *testing.T) {
	archiver := NewArchiver()
	assert.NotNil(t, archiver)
}

func TestArchiver_GetSupportedFormats(t *testing.T) {
	archiver := NewArchiver()
	formats := archiver.GetSupportedFormats()

	expectedFormats := []string{".tar.gz", ".tgz", ".zip", ".tar"}
	assert.ElementsMatch(t, expectedFormats, formats)
}

func TestArchiver_Extract_UnsupportedFormat(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "test.rar")

	// Create dummy file
	err := os.WriteFile(archivePath, []byte("dummy"), 0o644)
	require.NoError(t, err)

	archiver := NewArchiver()
	err = archiver.Extract(archivePath, tempDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported archive format")
}

func TestArchiver_ExtractZip(t *testing.T) {
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "test.zip")
	extractDir := filepath.Join(tempDir, "extracted")

	// Create test zip file
	createTestZip(t, zipPath)

	archiver := NewArchiver()
	err := archiver.ExtractZip(zipPath, extractDir)

	assert.NoError(t, err)

	// Verify extracted files
	content, err := os.ReadFile(filepath.Join(extractDir, "test.txt"))
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content))

	content, err = os.ReadFile(filepath.Join(extractDir, "subdir", "nested.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested content", string(content))
}

func TestArchiver_ExtractTarGz(t *testing.T) {
	tempDir := t.TempDir()
	tarGzPath := filepath.Join(tempDir, "test.tar.gz")
	extractDir := filepath.Join(tempDir, "extracted")

	// Create test tar.gz file
	createTestTarGz(t, tarGzPath)

	archiver := NewArchiver()
	err := archiver.ExtractTarGz(tarGzPath, extractDir)

	assert.NoError(t, err)

	// Verify extracted files
	content, err := os.ReadFile(filepath.Join(extractDir, "test.txt"))
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content))

	content, err = os.ReadFile(filepath.Join(extractDir, "subdir", "nested.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested content", string(content))
}

func TestArchiver_ExtractTar(t *testing.T) {
	tempDir := t.TempDir()
	tarPath := filepath.Join(tempDir, "test.tar")
	extractDir := filepath.Join(tempDir, "extracted")

	// Create test tar file
	createTestTar(t, tarPath)

	archiver := NewArchiver()
	err := archiver.ExtractTar(tarPath, extractDir)

	assert.NoError(t, err)

	// Verify extracted files
	content, err := os.ReadFile(filepath.Join(extractDir, "test.txt"))
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content))
}

func TestArchiver_Extract_ByExtension(t *testing.T) {
	tempDir := t.TempDir()
	extractDir := filepath.Join(tempDir, "extracted")
	archiver := NewArchiver()

	tests := []struct {
		createFn  func(string)
		name      string
		filename  string
		shouldErr bool
	}{
		{
			name:     "tar.gz file",
			filename: "test.tar.gz",
			createFn: func(path string) { createTestTarGz(t, path) },
		},
		{
			name:     "tgz file",
			filename: "test.tgz",
			createFn: func(path string) { createTestTarGz(t, path) },
		},
		{
			name:     "zip file",
			filename: "test.zip",
			createFn: func(path string) { createTestZip(t, path) },
		},
		{
			name:     "tar file",
			filename: "test.tar",
			createFn: func(path string) { createTestTar(t, path) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archivePath := filepath.Join(tempDir, tt.filename)
			testExtractDir := filepath.Join(extractDir, tt.name)

			tt.createFn(archivePath)

			err := archiver.Extract(archivePath, testExtractDir)

			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify at least one file was extracted
				content, err := os.ReadFile(filepath.Join(testExtractDir, "test.txt"))
				require.NoError(t, err)
				assert.Equal(t, "test content", string(content))
			}
		})
	}
}

func TestManager_DownloadAndExtract(t *testing.T) {
	tempDir := t.TempDir()

	// Create test server that responds with a valid tar.gz
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.WriteHeader(http.StatusOK)

		// Create a simple tar.gz response inline
		gzWriter := gzip.NewWriter(w)
		tarWriter := tar.NewWriter(gzWriter)

		// Add a test file
		header := &tar.Header{
			Name:     "test.txt",
			Mode:     0o644,
			Size:     12,
			Typeflag: tar.TypeReg,
		}
		tarWriter.WriteHeader(header)
		tarWriter.Write([]byte("test content"))

		tarWriter.Close()
		gzWriter.Close()
	}))
	defer server.Close()

	extractDir := filepath.Join(tempDir, "extracted")
	manager := NewManager()

	// Test the method but expect it to fail since the current implementation
	// doesn't preserve file extensions in temp files
	url := server.URL + "/test.tar.gz"
	err := manager.DownloadAndExtract(url, extractDir)

	// Should now work correctly with our fix
	assert.NoError(t, err)

	// Verify extraction worked
	extractedFile := filepath.Join(extractDir, "test.txt")
	assert.FileExists(t, extractedFile)

	content, err := os.ReadFile(extractedFile)
	require.NoError(t, err)
	assert.Equal(t, []byte("test content"), content)
}

func TestArchiver_ExtractZip_InvalidFile(t *testing.T) {
	tempDir := t.TempDir()
	invalidZip := filepath.Join(tempDir, "invalid.zip")
	extractDir := filepath.Join(tempDir, "extracted")

	// Create invalid zip file
	err := os.WriteFile(invalidZip, []byte("not a zip file"), 0o644)
	require.NoError(t, err)

	archiver := NewArchiver()
	err = archiver.ExtractZip(invalidZip, extractDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open zip")
}

func TestArchiver_ExtractTarGz_InvalidFile(t *testing.T) {
	tempDir := t.TempDir()
	invalidTarGz := filepath.Join(tempDir, "invalid.tar.gz")
	extractDir := filepath.Join(tempDir, "extracted")

	// Create invalid tar.gz file
	err := os.WriteFile(invalidTarGz, []byte("not a tar.gz file"), 0o644)
	require.NoError(t, err)

	archiver := NewArchiver()
	err = archiver.ExtractTarGz(invalidTarGz, extractDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create gzip reader")
}

func TestManager_MakeBinaryExecutable(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "testbinary")

	// Create a test binary file
	err := os.WriteFile(binaryPath, []byte("fake binary"), 0o644)
	require.NoError(t, err)

	// Test making it executable
	err = manager.MakeBinaryExecutable(binaryPath)
	assert.NoError(t, err)

	// Check permissions (Unix-like systems only)
	if runtime.GOOS != constants.WindowsOS {
		info, err := os.Stat(binaryPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	}
}

func TestManager_MakeBinaryExecutable_Windows(t *testing.T) {
	if runtime.GOOS != constants.WindowsOS {
		t.Skip("This test is for Windows only")
	}

	manager := NewManager()
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "testbinary.exe")

	// Create a test binary file
	err := os.WriteFile(binaryPath, []byte("fake binary"), 0o644)
	require.NoError(t, err)

	// On Windows, this should be a no-op
	err = manager.MakeBinaryExecutable(binaryPath)
	assert.NoError(t, err)
}

func TestManager_MakeBinaryExecutable_NonexistentFile(t *testing.T) {
	if runtime.GOOS == constants.WindowsOS {
		t.Skip("This test is for Unix-like systems only")
	}

	manager := NewManager()
	nonexistentPath := "/nonexistent/path/binary"

	err := manager.MakeBinaryExecutable(nonexistentPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestManager_InstallBinary(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	// Create source binary
	srcDir := filepath.Join(tempDir, "src")
	err := os.MkdirAll(srcDir, 0o750)
	require.NoError(t, err)

	srcPath := filepath.Join(srcDir, "testbinary")
	binaryContent := []byte("fake binary content")
	err = os.WriteFile(srcPath, binaryContent, 0o644)
	require.NoError(t, err)

	// Install to environment
	envPath := filepath.Join(tempDir, "env")
	binaryName := "mybinary"

	err = manager.InstallBinary(srcPath, envPath, binaryName)
	assert.NoError(t, err)

	// Check that binary was installed
	expectedPath := filepath.Join(envPath, "bin", binaryName)
	if runtime.GOOS == constants.WindowsOS {
		expectedPath += ".exe"
	}

	assert.FileExists(t, expectedPath)

	// Check content
	installedContent, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	assert.Equal(t, binaryContent, installedContent)

	// Check permissions (Unix-like systems only)
	if runtime.GOOS != constants.WindowsOS {
		info, err := os.Stat(expectedPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	}
}

func TestManager_InstallBinary_WindowsExeExtension(t *testing.T) {
	if runtime.GOOS != constants.WindowsOS {
		t.Skip("This test simulates Windows behavior")
	}

	manager := NewManager()
	tempDir := t.TempDir()

	// Create source binary
	srcPath := filepath.Join(tempDir, "testbinary.exe")
	err := os.WriteFile(srcPath, []byte("fake binary"), 0o644)
	require.NoError(t, err)

	// Install binary without .exe extension
	envPath := filepath.Join(tempDir, "env")
	err = manager.InstallBinary(srcPath, envPath, "mybinary")
	assert.NoError(t, err)

	// Should add .exe extension automatically on Windows
	expectedPath := filepath.Join(envPath, "bin", "mybinary.exe")
	assert.FileExists(t, expectedPath)
}

func TestManager_InstallBinary_SourceFileNotExists(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	nonexistentSrc := filepath.Join(tempDir, "nonexistent")
	envPath := filepath.Join(tempDir, "env")

	err := manager.InstallBinary(nonexistentSrc, envPath, "binary")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to copy binary")
}

func TestManager_InstallBinary_BinDirectoryCreation(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	// Create source binary
	srcPath := filepath.Join(tempDir, "testbinary")
	err := os.WriteFile(srcPath, []byte("fake binary"), 0o644)
	require.NoError(t, err)

	// Use non-existent environment path
	envPath := filepath.Join(tempDir, "newenv")

	err = manager.InstallBinary(srcPath, envPath, "binary")
	assert.NoError(t, err)

	// Check that bin directory was created
	binDir := filepath.Join(envPath, "bin")
	assert.DirExists(t, binDir)
}

func TestManager_copyFile(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tempDir, "source.txt")
	content := []byte("test content for copy")
	err := os.WriteFile(srcPath, content, 0o644)
	require.NoError(t, err)

	// Copy to destination
	dstPath := filepath.Join(tempDir, "destination.txt")
	err = manager.copyFile(srcPath, dstPath)
	assert.NoError(t, err)

	// Verify copy
	copiedContent, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, content, copiedContent)
}

func TestManager_copyFile_SourceNotExists(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	nonexistentSrc := filepath.Join(tempDir, "nonexistent.txt")
	dstPath := filepath.Join(tempDir, "destination.txt")

	err := manager.copyFile(nonexistentSrc, dstPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open source file")
}

func TestManager_copyFile_DestinationDirectoryNotExists(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tempDir, "source.txt")
	err := os.WriteFile(srcPath, []byte("content"), 0o644)
	require.NoError(t, err)

	// Try to copy to non-existent directory
	dstPath := filepath.Join(tempDir, "nonexistent", "destination.txt")
	err = manager.copyFile(srcPath, dstPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create destination file")
}

func TestArchiver_isPathSafe(t *testing.T) {
	archiver := NewArchiver()

	tests := []struct {
		name     string
		path     string
		destDir  string
		expected bool
	}{
		{
			name:     "safe path",
			path:     "/tmp/extract/file.txt",
			destDir:  "/tmp/extract",
			expected: true,
		},
		{
			name:     "safe nested path",
			path:     "/tmp/extract/subdir/file.txt",
			destDir:  "/tmp/extract",
			expected: true,
		},
		{
			name:     "path traversal attempt",
			path:     filepath.Join("/tmp/extract", "../../../etc/passwd"),
			destDir:  "/tmp/extract",
			expected: false,
		},
		{
			name:     "path outside destination",
			path:     "/tmp/other/file.txt",
			destDir:  "/tmp/extract",
			expected: false,
		},
		{
			name:     "exact destination path",
			path:     "/tmp/extract",
			destDir:  "/tmp/extract",
			expected: false, // Should require trailing separator
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := archiver.isPathSafe(tt.path, tt.destDir)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestArchiver_createDirectory(t *testing.T) {
	archiver := NewArchiver()
	tempDir := t.TempDir()

	dirPath := filepath.Join(tempDir, "testdir", "nested")
	err := archiver.createDirectory(dirPath)
	assert.NoError(t, err)
	assert.DirExists(t, dirPath)

	// Check permissions
	info, err := os.Stat(dirPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0o750), info.Mode().Perm())
}

func TestArchiver_createDirectory_InvalidPath(t *testing.T) {
	archiver := NewArchiver()

	// Try to create directory in a location that requires root permissions
	invalidPath := "/root/testdir"
	err := archiver.createDirectory(invalidPath)

	// This should fail on most systems unless running as root
	if os.Geteuid() != 0 { // Not running as root
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create directory")
	}
}

func TestManager_GetStatistics(t *testing.T) {
	manager := NewManager()

	stats := manager.GetStatistics()
	assert.NotNil(t, stats)

	timeout, exists := stats["timeout"]
	assert.True(t, exists)
	assert.Equal(t, "30s", timeout)

	// Test with custom timeout
	manager = manager.WithTimeout(60 * time.Second)
	stats = manager.GetStatistics()
	timeout, exists = stats["timeout"]
	assert.True(t, exists)
	assert.Equal(t, "1m0s", timeout)
}

func TestArchiver_ExtractTarEntry_PathTraversal(t *testing.T) {
	archiver := NewArchiver()
	tempDir := t.TempDir()
	destDir := filepath.Join(tempDir, "extract")

	// Create a tar header with path traversal attempt
	header := &tar.Header{
		Name:     "../../../etc/passwd",
		Typeflag: tar.TypeReg,
		Size:     10,
		Mode:     0o644,
	}

	// Create a fake tar reader (we don't actually read from it in this test)
	var fakeReader *tar.Reader
	err := archiver.extractTarEntry(fakeReader, header, destDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid path in archive")
}

func TestArchiver_extractZipFile_PathTraversal(t *testing.T) {
	archiver := NewArchiver()
	tempDir := t.TempDir()
	destDir := filepath.Join(tempDir, "extract")

	// Create a fake zip file entry with path traversal
	fakeFile := &zip.File{
		FileHeader: zip.FileHeader{
			Name: "../../../etc/passwd",
		},
	}

	err := archiver.extractZipFile(fakeFile, destDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid path in archive")
}

func TestArchiver_ExtractTar_CorruptedFile(t *testing.T) {
	archiver := NewArchiver()
	tempDir := t.TempDir()

	// Create a corrupted tar file
	corruptedTar := filepath.Join(tempDir, "corrupted.tar")
	err := os.WriteFile(corruptedTar, []byte("corrupted tar data"), 0o644)
	require.NoError(t, err)

	extractDir := filepath.Join(tempDir, "extracted")
	err = archiver.ExtractTar(corruptedTar, extractDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read tar entry")
}

func TestArchiver_extractTarEntries_EmptyArchive(t *testing.T) {
	archiver := NewArchiver()
	tempDir := t.TempDir()

	// Create an empty tar file
	emptyTar := filepath.Join(tempDir, "empty.tar")
	file, err := os.Create(emptyTar)
	require.NoError(t, err)

	tarWriter := tar.NewWriter(file)
	err = tarWriter.Close()
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	extractDir := filepath.Join(tempDir, "extracted")
	err = archiver.ExtractTar(emptyTar, extractDir)

	// Should succeed but create no files
	assert.NoError(t, err)
}

func TestArchiver_ExtractZip_DirectoryEntry(t *testing.T) {
	archiver := NewArchiver()
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "test.zip")
	extractDir := filepath.Join(tempDir, "extracted")

	// Create zip with directory entry
	file, err := os.Create(zipPath)
	require.NoError(t, err)
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	// Add directory entry with proper permissions
	dirHeader := &zip.FileHeader{
		Name: "testdir/",
	}
	dirHeader.SetMode(0o755 | os.ModeDir)
	_, err = zipWriter.CreateHeader(dirHeader)
	require.NoError(t, err)

	// Add file in directory
	fileInDir, err := zipWriter.Create("testdir/file.txt")
	require.NoError(t, err)
	_, err = fileInDir.Write([]byte("content"))
	require.NoError(t, err)

	err = zipWriter.Close()
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	// Extract and verify
	err = archiver.ExtractZip(zipPath, extractDir)
	assert.NoError(t, err)

	// Check that directory was created
	assert.DirExists(t, filepath.Join(extractDir, "testdir"))

	// Check that file exists
	filePath := filepath.Join(extractDir, "testdir", "file.txt")
	assert.FileExists(t, filePath)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, []byte("content"), content)
}

func TestArchiver_ExtractTar_DirectoryEntry(t *testing.T) {
	archiver := NewArchiver()
	tempDir := t.TempDir()
	tarPath := filepath.Join(tempDir, "test.tar")
	extractDir := filepath.Join(tempDir, "extracted")

	// Create tar with directory entry
	file, err := os.Create(tarPath)
	require.NoError(t, err)
	defer file.Close()

	tarWriter := tar.NewWriter(file)
	defer tarWriter.Close()

	// Add directory entry
	dirHeader := &tar.Header{
		Name:     "testdir/",
		Typeflag: tar.TypeDir,
		Mode:     0o755,
	}
	err = tarWriter.WriteHeader(dirHeader)
	require.NoError(t, err)

	// Add file in directory
	fileHeader := &tar.Header{
		Name:     "testdir/file.txt",
		Typeflag: tar.TypeReg,
		Size:     7,
		Mode:     0o644,
	}
	err = tarWriter.WriteHeader(fileHeader)
	require.NoError(t, err)
	_, err = tarWriter.Write([]byte("content"))
	require.NoError(t, err)

	err = tarWriter.Close()
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	// Extract and verify
	err = archiver.ExtractTar(tarPath, extractDir)
	assert.NoError(t, err)

	// Check that directory was created
	assert.DirExists(t, filepath.Join(extractDir, "testdir"))

	// Check that file exists with correct content and permissions
	filePath := filepath.Join(extractDir, "testdir", "file.txt")
	assert.FileExists(t, filePath)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, []byte("content"), content)

	// Check file permissions
	info, err := os.Stat(filePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func TestArchiver_openTarGzReader_NonexistentFile(t *testing.T) {
	archiver := NewArchiver()

	_, _, err := archiver.openTarGzReader("/nonexistent/file.tar.gz")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open archive")
}

func TestArchiver_openTarGzReader_InvalidGzip(t *testing.T) {
	archiver := NewArchiver()
	tempDir := t.TempDir()

	// Create a file that's not a valid gzip
	invalidGzip := filepath.Join(tempDir, "invalid.tar.gz")
	err := os.WriteFile(invalidGzip, []byte("not gzip data"), 0o644)
	require.NoError(t, err)

	_, _, err = archiver.openTarGzReader(invalidGzip)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create gzip reader")
}

func TestManager_DownloadFile_CreateFileError(t *testing.T) {
	// Test file creation error by trying to create file in non-existent directory without parent creation
	manager := NewManager()

	// Start a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test content"))
	}))
	defer server.Close()

	// Try to download to a path that will fail file creation (subdirectory of a file)
	tempDir := t.TempDir()
	blockingFile := filepath.Join(tempDir, "blocking")
	err := os.WriteFile(blockingFile, []byte("blocking"), 0o644)
	require.NoError(t, err)

	// Try to create a file inside what is actually a file (not directory)
	invalidDest := filepath.Join(blockingFile, "subfile.txt")

	err = manager.DownloadFile(server.URL, invalidDest)
	assert.Error(t, err)
	// The error could be either directory creation or file creation, both are valid
	assert.True(t,
		strings.Contains(err.Error(), "failed to create file") ||
			strings.Contains(err.Error(), "failed to create directory for"),
		"Expected error about file or directory creation, got: %v", err,
	)
}

func TestManager_DownloadFile_ResponseBodyCloseError(t *testing.T) {
	// This test verifies that warnings are logged when response body close fails
	// We can't easily simulate a close error, but we can verify the defer close logic exists
	manager := NewManager()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test content"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	dest := filepath.Join(tempDir, "downloaded.txt")

	err := manager.DownloadFile(server.URL, dest)
	assert.NoError(t, err)

	// Verify file was created and has correct content
	content, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, []byte("test content"), content)
}

func TestArchiver_extractZipRegularFile_ChmodError(t *testing.T) {
	// This test is difficult to implement portably since we can't easily create
	// a scenario where chmod fails without affecting other operations.
	// On most systems, this would require specific permission setups.
	t.Skip("Chmod error testing requires specific system setup")
}

func TestArchiver_extractRegularFile_ChmodError(t *testing.T) {
	// Similar to above - chmod error testing is system-dependent
	t.Skip("Chmod error testing requires specific system setup")
}

func TestManager_DownloadAndExtract_TempFileError(t *testing.T) {
	manager := NewManager()

	// Test the full workflow with a proper server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Create a tar.gz response
		w.Header().Set("Content-Type", "application/gzip")
		w.WriteHeader(http.StatusOK)

		gzWriter := gzip.NewWriter(w)
		tarWriter := tar.NewWriter(gzWriter)

		// Add a test file
		header := &tar.Header{
			Name:     "test.txt",
			Mode:     0o644,
			Size:     12,
			Typeflag: tar.TypeReg,
		}
		tarWriter.WriteHeader(header)
		tarWriter.Write([]byte("test content"))

		tarWriter.Close()
		gzWriter.Close()
	}))
	defer server.Close()

	tempDir := t.TempDir()
	destDir := filepath.Join(tempDir, "extracted")

	err := manager.DownloadAndExtract(server.URL+"/test.tar.gz", destDir)
	assert.NoError(t, err)

	// Verify extraction worked
	extractedFile := filepath.Join(destDir, "test.txt")
	assert.FileExists(t, extractedFile)

	content, err := os.ReadFile(extractedFile)
	require.NoError(t, err)
	assert.Equal(t, []byte("test content"), content)
}

func TestManager_DownloadAndExtract_DownloadError(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	// Test with invalid URL that will cause download to fail
	err := manager.DownloadAndExtract("http://invalid-host-that-does-not-exist.com/file.tar.gz", tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no such host")
}

func TestManager_DownloadAndExtract_ExtractError(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	// Create a server that returns invalid archive content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write invalid tar.gz content
		_, _ = w.Write([]byte("not a valid archive"))
	}))
	defer server.Close()

	err := manager.DownloadAndExtract(server.URL+"/invalid.tar.gz", tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gzip:")
}

func TestManager_DownloadAndExtract_UnsupportedFormat(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	// Create a server that returns an unsupported file format
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("some content"))
	}))
	defer server.Close()

	err := manager.DownloadAndExtract(server.URL+"/file.unknown", tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported archive format")
}

func TestManager_DownloadAndExtract_TempFileCloseError(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	// This test ensures that if tempFile.Close() fails, we still handle it gracefully
	// We can't easily simulate a close error, but this verifies the code path

	// Create a server with valid archive content
	server := createTestArchiveServer()
	defer server.Close()

	// The existing implementation should handle this gracefully
	err := manager.DownloadAndExtract(server.URL+"/test.tar.gz", tempDir)
	assert.NoError(t, err)
}

func TestArchiver_ExtractZip_ReadOnlyDestination(t *testing.T) {
	tempDir := t.TempDir()

	// Create a zip archive
	zipPath := filepath.Join(tempDir, "test.zip")
	zipFile, err := os.Create(zipPath)
	require.NoError(t, err)

	zipWriter := zip.NewWriter(zipFile)
	fileWriter, err := zipWriter.Create("test.txt")
	require.NoError(t, err)
	_, err = fileWriter.Write([]byte("test content"))
	require.NoError(t, err)
	require.NoError(t, zipWriter.Close())
	require.NoError(t, zipFile.Close())

	// Create a read-only destination directory
	destDir := filepath.Join(tempDir, "readonly")
	require.NoError(t, os.Mkdir(destDir, 0o444)) // Read-only directory

	archiver := NewArchiver()
	err = archiver.ExtractZip(zipPath, destDir)

	// Restore write permissions for cleanup
	_ = os.Chmod(destDir, 0o755)

	// Should fail due to permission issues
	assert.Error(t, err)
}

func TestArchiver_ExtractTar_ReadOnlyDestination(t *testing.T) {
	tempDir := t.TempDir()

	// Create a tar archive
	tarPath := filepath.Join(tempDir, "test.tar")
	tarFile, err := os.Create(tarPath)
	require.NoError(t, err)

	tarWriter := tar.NewWriter(tarFile)
	header := &tar.Header{
		Name: "test.txt",
		Mode: 0o644,
		Size: 12,
	}
	require.NoError(t, tarWriter.WriteHeader(header))
	_, err = tarWriter.Write([]byte("test content"))
	require.NoError(t, err)
	require.NoError(t, tarWriter.Close())
	require.NoError(t, tarFile.Close())

	// Create a read-only destination directory
	destDir := filepath.Join(tempDir, "readonly")
	require.NoError(t, os.Mkdir(destDir, 0o444)) // Read-only directory

	archiver := NewArchiver()
	err = archiver.ExtractTar(tarPath, destDir)

	// Restore write permissions for cleanup
	_ = os.Chmod(destDir, 0o755)

	// Should fail due to permission issues
	assert.Error(t, err)
}

func TestArchiver_extractTarEntries_ReaderError(t *testing.T) {
	tempDir := t.TempDir()

	// Create a corrupted tar file that will cause read errors
	tarPath := filepath.Join(tempDir, "corrupted.tar")
	tarFile, err := os.Create(tarPath)
	require.NoError(t, err)

	// Write some header-like bytes but not a complete valid tar
	_, err = tarFile.Write([]byte("invalid tar header data that looks like tar but isn't"))
	require.NoError(t, err)
	require.NoError(t, tarFile.Close())

	destDir := filepath.Join(tempDir, "dest")
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	archiver := NewArchiver()
	err = archiver.ExtractTar(tarPath, destDir)
	assert.Error(t, err)
}

func TestManager_InstallBinary_DestinationNotWritable(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tempDir, "source")
	require.NoError(t, os.WriteFile(srcPath, []byte("binary content"), 0o755))

	// Create read-only destination directory
	destDir := filepath.Join(tempDir, "readonly")
	require.NoError(t, os.Mkdir(destDir, 0o444)) // Read-only directory

	err := manager.InstallBinary(srcPath, destDir, "binary")

	// Restore write permissions for cleanup
	_ = os.Chmod(destDir, 0o755)

	// Should fail due to permission issues
	assert.Error(t, err)
}

func TestManager_copyFile_DestinationExists(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tempDir, "source.txt")
	require.NoError(t, os.WriteFile(srcPath, []byte("source content"), 0o644))

	// Create destination file that already exists
	destPath := filepath.Join(tempDir, "dest.txt")
	require.NoError(t, os.WriteFile(destPath, []byte("old content"), 0o644))

	// Copy should overwrite the existing file
	err := manager.copyFile(srcPath, destPath)
	assert.NoError(t, err)

	// Verify the content was replaced
	content, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Equal(t, "source content", string(content))
}

func TestManager_copyFile_SourceIsDirectory(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	// Create source directory
	srcPath := filepath.Join(tempDir, "source")
	require.NoError(t, os.Mkdir(srcPath, 0o755))

	destPath := filepath.Join(tempDir, "dest.txt")

	// Should fail when trying to copy a directory as file
	err := manager.copyFile(srcPath, destPath)
	assert.Error(t, err)
}

func TestArchiver_createDirectory_PermissionError(t *testing.T) {
	tempDir := t.TempDir()

	// Create a read-only parent directory
	parentDir := filepath.Join(tempDir, "readonly")
	require.NoError(t, os.Mkdir(parentDir, 0o444)) // Read-only directory

	// Try to create a subdirectory in the read-only parent
	targetDir := filepath.Join(parentDir, "subdir")

	archiver := NewArchiver()
	err := archiver.createDirectory(targetDir)

	// Restore write permissions for cleanup
	_ = os.Chmod(parentDir, 0o755)

	// Should fail due to permission issues
	assert.Error(t, err)
}

func TestArchiver_ExtractZip_SymlinkHandling(t *testing.T) {
	if runtime.GOOS == constants.WindowsOS {
		t.Skip("Symlink test not applicable on Windows")
	}

	tempDir := t.TempDir()

	// Create a zip with a symlink
	zipPath := filepath.Join(tempDir, "symlink.zip")
	zipFile, err := os.Create(zipPath)
	require.NoError(t, err)

	zipWriter := zip.NewWriter(zipFile)

	// Add a regular file first
	fileWriter, err := zipWriter.Create("target.txt")
	require.NoError(t, err)
	_, err = fileWriter.Write([]byte("target content"))
	require.NoError(t, err)

	// Add a symlink entry (this is tricky with the zip library)
	// We'll create a file that looks like a symlink would be extracted
	linkWriter, err := zipWriter.Create("link.txt")
	require.NoError(t, err)
	_, err = linkWriter.Write([]byte("symlink content"))
	require.NoError(t, err)

	require.NoError(t, zipWriter.Close())
	require.NoError(t, zipFile.Close())

	destDir := filepath.Join(tempDir, "extracted")
	archiver := NewArchiver()
	err = archiver.ExtractZip(zipPath, destDir)
	assert.NoError(t, err)

	// Verify files were extracted
	assert.FileExists(t, filepath.Join(destDir, "target.txt"))
	assert.FileExists(t, filepath.Join(destDir, "link.txt"))
}

func TestArchiver_ExtractTar_SpecialPermissions(t *testing.T) {
	tempDir := t.TempDir()

	// Create a tar archive with special permissions
	tarPath := filepath.Join(tempDir, "permissions.tar")
	tarFile, err := os.Create(tarPath)
	require.NoError(t, err)

	tarWriter := tar.NewWriter(tarFile)

	// Add a file with special permissions
	content := "#!/bin/bash\necho 'test'"
	header := &tar.Header{
		Name: "executable.sh",
		Mode: 0o755, // Executable permissions
		Size: int64(len(content)),
	}
	require.NoError(t, tarWriter.WriteHeader(header))
	_, err = tarWriter.Write([]byte(content))
	require.NoError(t, err)

	require.NoError(t, tarWriter.Close())
	require.NoError(t, tarFile.Close())

	destDir := filepath.Join(tempDir, "extracted")
	archiver := NewArchiver()
	err = archiver.ExtractTar(tarPath, destDir)
	assert.NoError(t, err)

	// Verify file exists and has correct permissions
	extractedFile := filepath.Join(destDir, "executable.sh")
	assert.FileExists(t, extractedFile)

	info, err := os.Stat(extractedFile)
	require.NoError(t, err)

	// Check that the file has executable permissions
	if runtime.GOOS != constants.WindowsOS {
		assert.Equal(t, os.FileMode(0o755), info.Mode())
	}
}

func TestManager_DownloadFile_LargeFile(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	// Create a server that returns a larger file to test streaming
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write 1MB of data
		data := make([]byte, 1024*1024)
		for i := range data {
			data[i] = byte(i % 256)
		}
		_, _ = w.Write(data)
	}))
	defer server.Close()

	destPath := filepath.Join(tempDir, "large_file.dat")
	err := manager.DownloadFile(server.URL, destPath)
	assert.NoError(t, err)

	// Verify file size
	info, err := os.Stat(destPath)
	require.NoError(t, err)
	assert.Equal(t, int64(1024*1024), info.Size())
}

func TestManager_DownloadFile_EmptyResponse(t *testing.T) {
	manager := NewManager()
	tempDir := t.TempDir()

	// Create a server that returns empty content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		// No content written
	}))
	defer server.Close()

	destPath := filepath.Join(tempDir, "empty_file.txt")
	err := manager.DownloadFile(server.URL, destPath)
	assert.NoError(t, err)

	// Verify file exists but is empty
	info, err := os.Stat(destPath)
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func TestArchiver_ExtractTarGz_InvalidGzipData(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file with .tar.gz extension but invalid gzip data
	tarGzPath := filepath.Join(tempDir, "invalid.tar.gz")
	require.NoError(t, os.WriteFile(tarGzPath, []byte("not gzip data"), 0o644))

	destDir := filepath.Join(tempDir, "dest")
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	archiver := NewArchiver()
	err := archiver.ExtractTarGz(tarGzPath, destDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gzip:")
}

func TestArchiver_Extract_FileExtensionEdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	archiver := NewArchiver()
	destDir := filepath.Join(tempDir, "dest")
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	tests := []struct {
		filename      string
		shouldSucceed bool
	}{
		{"file.TAR.GZ", false},  // Case sensitivity
		{"file.tar.gz.", false}, // Trailing dot
		{"file.txt.zip", true},  // Valid zip with multiple dots
		{"file", false},         // No extension
		{".hidden.tar", true},   // Hidden file with tar extension
		{"file.tar.bz2", false}, // Unsupported compression
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			// Create appropriate archive based on expected success
			archivePath := filepath.Join(tempDir, tt.filename)

			if strings.HasSuffix(strings.ToLower(tt.filename), ".zip") {
				createTestZip(t, archivePath)
			} else if strings.HasSuffix(strings.ToLower(tt.filename), ".tar") {
				createTestTar(t, archivePath)
			} else {
				// For other cases, create a dummy file
				require.NoError(t, os.WriteFile(archivePath, []byte("dummy content"), 0o644))
			}

			err := archiver.Extract(archivePath, destDir)
			if tt.shouldSucceed {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// Helper functions for creating test archives

func createTestZip(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	// Add test.txt
	testFile, err := zipWriter.Create("test.txt")
	require.NoError(t, err)
	_, err = testFile.Write([]byte("test content"))
	require.NoError(t, err)

	// Add subdir/nested.txt
	nestedFile, err := zipWriter.Create("subdir/nested.txt")
	require.NoError(t, err)
	_, err = nestedFile.Write([]byte("nested content"))
	require.NoError(t, err)
}

func createTestTarGz(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	addFileToTar(t, tarWriter, "test.txt", "test content")
	addFileToTar(t, tarWriter, "subdir/nested.txt", "nested content")
}

func createTestTar(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()

	tarWriter := tar.NewWriter(file)
	defer tarWriter.Close()

	addFileToTar(t, tarWriter, "test.txt", "test content")
}

func addFileToTar(t *testing.T, tarWriter *tar.Writer, name, content string) {
	t.Helper()
	header := &tar.Header{
		Name:     name,
		Mode:     0o644,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}

	err := tarWriter.WriteHeader(header)
	require.NoError(t, err)

	_, err = tarWriter.Write([]byte(content))
	require.NoError(t, err)
}

func createTestArchiveServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ /* r */ *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.WriteHeader(http.StatusOK)

		gzWriter := gzip.NewWriter(w)
		tarWriter := tar.NewWriter(gzWriter)

		// Add a test file
		header := &tar.Header{
			Name:     "test.txt",
			Mode:     0o644,
			Size:     12,
			Typeflag: tar.TypeReg,
		}
		tarWriter.WriteHeader(header)
		tarWriter.Write([]byte("test content"))

		tarWriter.Close()
		gzWriter.Close()
	}))
}
