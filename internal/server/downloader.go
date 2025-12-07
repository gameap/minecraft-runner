package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gameap/minecraft-runner/internal/providers"
	"github.com/gameap/minecraft-runner/internal/utils"
)

// DownloadOptions contains options for downloading a server
type DownloadOptions struct {
	Version    string // Minecraft version
	Mod        string // Mod type (vanilla, paper, forge, etc.)
	ModVersion string // Mod-specific version
	Directory  string // Server directory
	Force      bool   // Force re-download even if exists
}

// Downloader handles server JAR downloads
type Downloader struct {
	registry *providers.Registry
	http     *utils.HTTPClient
}

// NewDownloader creates a new server downloader
func NewDownloader(registry *providers.Registry) *Downloader {
	return &Downloader{
		registry: registry,
		http:     utils.NewHTTPClient(true), // Show progress
	}
}

// Download downloads a server JAR
func (d *Downloader) Download(ctx context.Context, opts DownloadOptions) (*providers.ServerJar, error) {
	// Get provider
	provider, err := d.registry.Get(opts.Mod)
	if err != nil {
		return nil, fmt.Errorf("unknown server type '%s': %w", opts.Mod, err)
	}

	// Resolve version if not specified
	version := opts.Version
	if version == "" {
		latest, err := provider.GetLatestVersion(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest version: %w", err)
		}
		version = latest.MinecraftVersion
		fmt.Printf("Using latest version: %s\n", version)
	}

	// Get JAR info
	jar, err := provider.GetServerJar(ctx, version, opts.ModVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get server info: %w", err)
	}

	// Determine target path
	targetPath := filepath.Join(opts.Directory, jar.Filename)

	// Check if JAR already exists
	if !opts.Force && d.jarExists(targetPath, jar) {
		fmt.Printf("Server JAR already exists: %s\n", jar.Filename)
		return jar, nil
	}

	// Ensure directory exists
	if err := os.MkdirAll(opts.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Download JAR
	fmt.Printf("Downloading %s...\n", jar.Filename)
	if err := d.http.DownloadFile(ctx, jar.URL, targetPath); err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}

	// Verify checksum
	if jar.SHA256 != "" {
		fmt.Println("Verifying SHA256 checksum...")
		if err := utils.VerifySHA256(targetPath, jar.SHA256); err != nil {
			os.Remove(targetPath)
			return nil, fmt.Errorf("checksum verification failed: %w", err)
		}
	} else if jar.SHA1 != "" {
		fmt.Println("Verifying SHA1 checksum...")
		if err := utils.VerifySHA1(targetPath, jar.SHA1); err != nil {
			os.Remove(targetPath)
			return nil, fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Make executable on Unix
	os.Chmod(targetPath, 0755)

	fmt.Printf("Downloaded successfully: %s\n", jar.Filename)
	return jar, nil
}

// jarExists checks if a JAR file exists and optionally verifies its checksum
func (d *Downloader) jarExists(path string, jar *providers.ServerJar) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}

	// If we have a checksum, verify it
	if jar.SHA256 != "" {
		if err := utils.VerifySHA256(path, jar.SHA256); err != nil {
			return false
		}
	} else if jar.SHA1 != "" {
		if err := utils.VerifySHA1(path, jar.SHA1); err != nil {
			return false
		}
	}

	return true
}

// FindServerJar finds the server JAR file in a directory
func FindServerJar(directory string, mod string, version string) (string, error) {
	// Common JAR patterns
	patterns := []string{
		fmt.Sprintf("minecraft_server.%s.jar", version),
		fmt.Sprintf("server.jar"),
		fmt.Sprintf("paper-*.jar"),
		fmt.Sprintf("forge-*.jar"),
		fmt.Sprintf("fabric-server-*.jar"),
	}

	// Add mod-specific patterns
	switch mod {
	case "paper":
		patterns = append([]string{fmt.Sprintf("paper-%s-*.jar", version)}, patterns...)
	case "forge":
		patterns = append([]string{
			fmt.Sprintf("forge-%s-*.jar", version),
			"forge-*.jar",
		}, patterns...)
	case "fabric":
		patterns = append([]string{fmt.Sprintf("fabric-server-mc.%s-*.jar", version)}, patterns...)
	case "vanilla":
		patterns = append([]string{fmt.Sprintf("minecraft_server.%s.jar", version)}, patterns...)
	case "waterfall":
		patterns = append([]string{
			fmt.Sprintf("waterfall-%s-*.jar", version),
			"waterfall-*.jar",
		}, patterns...)
	case "velocity":
		patterns = append([]string{
			fmt.Sprintf("velocity-%s-*.jar", version),
			"velocity-*.jar",
		}, patterns...)
	case "bungeecord":
		patterns = append([]string{
			"BungeeCord-*.jar",
			"BungeeCord.jar",
		}, patterns...)
	}

	// Search for JAR files
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(directory, pattern))
		if err != nil {
			continue
		}
		if len(matches) > 0 {
			return matches[0], nil
		}
	}

	// Fallback: find any JAR file
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".jar" {
			return filepath.Join(directory, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no server JAR found in %s", directory)
}
