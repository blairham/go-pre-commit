// Package constants provides shared constants used throughout the go-pre-commit project
package constants

// Operating system identifiers
const (
	// WindowsOS represents the Windows operating system string
	WindowsOS = "windows"
	// LinuxOS represents the Linux operating system string
	LinuxOS = "linux"
	// DarwinOS represents the macOS/Darwin operating system string
	DarwinOS = "darwin"
)

// Architecture identifiers
const (
	// ArchAMD64 represents the AMD 64-bit architecture identifier
	ArchAMD64 = "amd64"
	// ArchARM64 represents the ARM 64-bit architecture identifier
	ArchARM64 = "arm64"
	// Arch386 represents the 386 architecture identifier
	Arch386 = "386"
)

// Common path constants
const (
	// TmpEnvPath represents a common temporary environment path for testing
	TmpEnvPath = "/tmp/test-env"
	// TmpPythonEnvPath represents a common temporary Python environment path for testing
	TmpPythonEnvPath = "/tmp/env/python"
)
