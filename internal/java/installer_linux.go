//go:build linux

package java

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// setDefault sets Java as the system default using update-alternatives
func (i *Installer) setDefault(javaBin string) error {
	// Check if we have root privileges
	if os.Geteuid() != 0 {
		fmt.Println("Note: Setting system default requires root privileges.")
		fmt.Printf("Run: sudo update-alternatives --install /usr/bin/java java %s 1000\n", javaBin)
		return nil
	}

	// Register with update-alternatives
	cmd := exec.Command("update-alternatives", "--install",
		"/usr/bin/java", "java", javaBin, "1000")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("update-alternatives failed: %w", err)
	}

	// Set as default
	cmd = exec.Command("update-alternatives", "--set", "java", javaBin)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set as default: %w", err)
	}

	// Also set javac if available
	javacBin := filepath.Join(filepath.Dir(javaBin), "javac")
	if _, err := os.Stat(javacBin); err == nil {
		cmd = exec.Command("update-alternatives", "--install",
			"/usr/bin/javac", "javac", javacBin, "1000")
		cmd.Run() // Ignore errors for javac
	}

	fmt.Println("Java set as system default")
	return nil
}
