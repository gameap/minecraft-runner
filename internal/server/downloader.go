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

// Download resolves and downloads a server JAR
func (d *Downloader) Download(ctx context.Context, opts DownloadOptions) (*providers.ServerJar, error) {
	jar, err := d.Resolve(ctx, opts)
	if err != nil {
		return nil, err
	}

	if err := d.Fetch(ctx, opts.Directory, jar, opts.Force); err != nil {
		return nil, err
	}

	return jar, nil
}

// Resolve asks the provider which file serves the requested version. It is the
// only step that needs the provider's API.
func (d *Downloader) Resolve(ctx context.Context, opts DownloadOptions) (*providers.ServerJar, error) {
	provider, err := d.registry.Get(opts.Mod)
	if err != nil {
		return nil, fmt.Errorf("unknown server type '%s': %w", opts.Mod, err)
	}

	version := opts.Version
	if version == "" {
		latest, err := provider.GetLatestVersion(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest version: %w", err)
		}
		version = latest.MinecraftVersion
		fmt.Printf("Using latest version: %s\n", version)
	}

	jar, err := provider.GetServerJar(ctx, version, opts.ModVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get server info: %w", err)
	}

	return jar, nil
}

// Fetch downloads a resolved JAR into the directory unless a verified copy is already there
func (d *Downloader) Fetch(ctx context.Context, directory string, jar *providers.ServerJar, force bool) error {
	targetPath := filepath.Join(directory, jar.Filename)

	if !force && d.IsDownloaded(directory, jar) {
		fmt.Printf("Server JAR already exists: %s\n", jar.Filename)
		return nil
	}

	if err := os.MkdirAll(directory, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fmt.Printf("Downloading %s...\n", jar.Filename)
	if err := d.http.DownloadFile(ctx, jar.URL, targetPath); err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	if err := verifyChecksum(targetPath, jar, true); err != nil {
		os.Remove(targetPath)
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	os.Chmod(targetPath, 0755)

	fmt.Printf("Downloaded successfully: %s\n", jar.Filename)
	return nil
}

// IsDownloaded checks that the JAR is present and, when the provider published
// a checksum, that it still matches
func (d *Downloader) IsDownloaded(directory string, jar *providers.ServerJar) bool {
	path := filepath.Join(directory, jar.Filename)

	if _, err := os.Stat(path); err != nil {
		return false
	}

	return verifyChecksum(path, jar, false) == nil
}

// verifyChecksum checks the strongest checksum the provider published
func verifyChecksum(path string, jar *providers.ServerJar, announce bool) error {
	var name string
	var verify func() error

	switch {
	case jar.SHA256 != "":
		name, verify = "SHA256", func() error { return utils.VerifySHA256(path, jar.SHA256) }
	case jar.SHA1 != "":
		name, verify = "SHA1", func() error { return utils.VerifySHA1(path, jar.SHA1) }
	case jar.MD5 != "":
		name, verify = "MD5", func() error { return utils.VerifyMD5(path, jar.MD5) }
	default:
		return nil
	}

	if announce {
		fmt.Printf("Verifying %s checksum...\n", name)
	}

	return verify()
}
