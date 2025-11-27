package utils

import (
	"runtime"
)

// GetOS returns the current operating system name normalized for APIs
func GetOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "mac"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

// GetArch returns the current architecture normalized for APIs
func GetArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "aarch64"
	case "386":
		return "x86"
	default:
		return runtime.GOARCH
	}
}

// isWindows returns true if running on Windows
func isWindows() bool {
	return runtime.GOOS == "windows"
}

// IsLinux returns true if running on Linux
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// IsMac returns true if running on macOS
func IsMac() bool {
	return runtime.GOOS == "darwin"
}

// GetExecutableExtension returns the executable extension for the current OS
func GetExecutableExtension() string {
	if isWindows() {
		return ".exe"
	}
	return ""
}

// GetJavaExecutable returns the Java executable name for the current OS
func GetJavaExecutable() string {
	return "java" + GetExecutableExtension()
}
