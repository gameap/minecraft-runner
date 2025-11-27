package java

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gameap/minecraft-runner/internal/utils"
	"github.com/gameap/minecraft-runner/pkg/api/adoptium"
)

// InstallOptions contains options for Java installation
type InstallOptions struct {
	SystemWide bool   // Install to system directory vs local
	Directory  string // Custom installation directory
	SetDefault bool   // Set as system default after install
}

// Installer handles Java installation
type Installer struct {
	client *adoptium.Client
	http   *utils.HTTPClient
}

// NewInstaller creates a new Java installer
func NewInstaller() *Installer {
	return &Installer{
		client: adoptium.NewClient(),
		http:   utils.NewHTTPClient(true), // Show progress
	}
}

// Install downloads and installs a specific Java version
func (i *Installer) Install(ctx context.Context, version int, opts InstallOptions) (*Installation, error) {
	// Determine installation directory
	installDir, err := i.getInstallDir(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to determine install directory: %w", err)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create install directory: %w", err)
	}

	// Get asset info from Adoptium
	asset, err := i.client.GetAssetForCurrentPlatform(ctx, version, "jre")
	if err != nil {
		return nil, fmt.Errorf("failed to get Java asset info: %w", err)
	}

	fmt.Printf("Downloading Java %d (%s)...\n", version, asset.Version.Semver)

	// Download the archive
	archivePath := filepath.Join(os.TempDir(), asset.Binary.Package.Name)
	if err := i.http.DownloadFile(ctx, asset.Binary.Package.Link, archivePath); err != nil {
		return nil, fmt.Errorf("failed to download Java: %w", err)
	}
	defer os.Remove(archivePath)

	// Verify checksum
	fmt.Println("Verifying checksum...")
	if err := utils.VerifySHA256(archivePath, asset.Binary.Package.Checksum); err != nil {
		return nil, fmt.Errorf("checksum verification failed: %w", err)
	}

	// Extract archive
	fmt.Println("Extracting...")
	var javaHome string
	if runtime.GOOS == "windows" {
		javaHome, err = utils.ExtractZip(archivePath, installDir)
	} else {
		javaHome, err = utils.ExtractTarGz(archivePath, installDir)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to extract Java: %w", err)
	}

	// Find java binary
	javaBin := i.findJavaBinary(javaHome)
	if javaBin == "" {
		return nil, fmt.Errorf("could not find java binary in extracted archive")
	}

	// Platform-specific post-install
	if err := i.platformPostInstall(ctx, javaBin, opts); err != nil {
		// Log warning but don't fail
		fmt.Printf("Warning: post-install step failed: %v\n", err)
	}

	fmt.Printf("Java %d installed successfully to %s\n", version, javaHome)

	return &Installation{
		Path:        javaBin,
		Version:     version,
		FullVersion: asset.Version.Semver,
		Vendor:      "Eclipse Adoptium",
		IsSystem:    opts.SystemWide,
		Arch:        runtime.GOARCH,
	}, nil
}

// getInstallDir determines the installation directory
func (i *Installer) getInstallDir(opts InstallOptions) (string, error) {
	if opts.Directory != "" {
		return opts.Directory, nil
	}

	if opts.SystemWide {
		return i.getSystemInstallDir()
	}

	return i.getUserInstallDir()
}

// getSystemInstallDir returns the system-wide Java installation directory
func (i *Installer) getSystemInstallDir() (string, error) {
	if runtime.GOOS == "windows" {
		return `C:\Program Files\Java`, nil
	}
	return "/usr/lib/jvm", nil
}

// getUserInstallDir returns the user-local Java installation directory
func (i *Installer) getUserInstallDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".mcrun", "java"), nil
}

// findJavaBinary locates the java binary in an extracted JRE/JDK directory
func (i *Installer) findJavaBinary(javaHome string) string {
	javaBin := "java"
	if runtime.GOOS == "windows" {
		javaBin = "java.exe"
	}

	// Standard location
	binPath := filepath.Join(javaHome, "bin", javaBin)
	if _, err := os.Stat(binPath); err == nil {
		return binPath
	}

	// macOS bundle structure
	if runtime.GOOS == "darwin" {
		macPath := filepath.Join(javaHome, "Contents", "Home", "bin", javaBin)
		if _, err := os.Stat(macPath); err == nil {
			return macPath
		}
	}

	// Search recursively as fallback
	var found string
	filepath.Walk(javaHome, func(path string, info os.FileInfo, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if info.Name() == javaBin && filepath.Base(filepath.Dir(path)) == "bin" {
			found = path
		}
		return nil
	})

	return found
}

// platformPostInstall performs platform-specific post-installation steps
func (i *Installer) platformPostInstall(ctx context.Context, javaBin string, opts InstallOptions) error {
	if !opts.SetDefault {
		return nil
	}

	return i.setDefault(javaBin)
}

// GetBundledPath returns the path for a bundled JRE in a server directory
func (i *Installer) GetBundledPath(serverDir string, version int) string {
	return filepath.Join(serverDir, ".java", fmt.Sprintf("jre-%d", version))
}

// InstallBundled installs Java to a server's local directory
func (i *Installer) InstallBundled(ctx context.Context, serverDir string, version int) (*Installation, error) {
	bundledDir := i.GetBundledPath(serverDir, version)
	return i.Install(ctx, version, InstallOptions{
		Directory: bundledDir,
	})
}
