//go:build darwin

package java

import (
	"fmt"
	"os"
	"path/filepath"
)

// setDefault on macOS adds Java to shell profile
func (i *Installer) setDefault(javaBin string) error {
	// On macOS, we add to shell profile instead of update-alternatives
	javaHome := filepath.Dir(filepath.Dir(javaBin))

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Determine which profile to use
	profiles := []string{
		filepath.Join(homeDir, ".zshrc"),
		filepath.Join(homeDir, ".bash_profile"),
		filepath.Join(homeDir, ".profile"),
	}

	var profilePath string
	for _, p := range profiles {
		if _, err := os.Stat(p); err == nil {
			profilePath = p
			break
		}
	}

	if profilePath == "" {
		profilePath = filepath.Join(homeDir, ".zshrc") // Default to zshrc for modern macOS
	}

	exportLine := fmt.Sprintf("\nexport JAVA_HOME=\"%s\"\nexport PATH=\"$JAVA_HOME/bin:$PATH\"\n", javaHome)

	fmt.Printf("To set Java as default, add the following to %s:\n", profilePath)
	fmt.Println(exportLine)
	fmt.Println("Then run: source", profilePath)

	return nil
}
