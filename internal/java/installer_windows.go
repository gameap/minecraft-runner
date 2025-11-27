//go:build windows

package java

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// setDefault adds Java to the system PATH on Windows
func (i *Installer) setDefault(javaBin string) error {
	binDir := filepath.Dir(javaBin)

	// Check if we can modify system PATH (requires admin)
	// Try user PATH first as it doesn't require elevation
	fmt.Println("Adding Java to user PATH...")

	// Get current user PATH
	cmd := exec.Command("powershell", "-Command",
		"[Environment]::GetEnvironmentVariable('PATH', 'User')")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get user PATH: %w", err)
	}

	currentPath := strings.TrimSpace(string(output))

	// Check if already in PATH
	if strings.Contains(strings.ToLower(currentPath), strings.ToLower(binDir)) {
		fmt.Println("Java already in PATH")
		return nil
	}

	// Add to PATH
	var newPath string
	if currentPath == "" {
		newPath = binDir
	} else {
		newPath = binDir + ";" + currentPath
	}

	cmd = exec.Command("powershell", "-Command",
		fmt.Sprintf("[Environment]::SetEnvironmentVariable('PATH', '%s', 'User')", newPath))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update user PATH: %w", err)
	}

	// Also set JAVA_HOME
	javaHome := filepath.Dir(binDir)
	cmd = exec.Command("powershell", "-Command",
		fmt.Sprintf("[Environment]::SetEnvironmentVariable('JAVA_HOME', '%s', 'User')", javaHome))
	cmd.Run() // Ignore errors for JAVA_HOME

	fmt.Println("Java added to PATH. Please restart your terminal for changes to take effect.")
	return nil
}
