package providers

import (
	"context"
	"fmt"
	"time"
)

// ServerJar contains information about a downloadable server JAR
type ServerJar struct {
	Version         string // Minecraft version
	ModVersion      string // Mod-specific version (e.g., Paper build number)
	URL             string // Download URL
	SHA256          string // SHA256 hash for verification (optional)
	SHA1            string // SHA1 hash for verification (optional)
	Filename        string // Suggested filename
	RequiresInstall bool   // True if post-download installation is needed (e.g., Forge)
}

// VersionInfo contains information about an available version
type VersionInfo struct {
	MinecraftVersion string
	ModVersion       string
	IsStable         bool
	ReleaseDate      time.Time
	Type             string // "release", "snapshot", etc.
}

// Provider is the interface for server providers
type Provider interface {
	// Name returns the provider name (vanilla, paper, forge, etc.)
	Name() string

	// ListVersions returns available Minecraft versions for this provider
	ListVersions(ctx context.Context) ([]VersionInfo, error)

	// ListModVersions returns mod versions for a specific Minecraft version
	ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error)

	// GetLatestVersion returns the latest stable version
	GetLatestVersion(ctx context.Context) (*VersionInfo, error)

	// GetServerJar returns download info for a specific version
	GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error)

	// PostDownload handles any post-download steps (e.g., Forge installer)
	PostDownload(ctx context.Context, jarPath string, javaPath string) error

	// GetRecommendedJavaVersion returns the recommended Java version for this MC version
	GetRecommendedJavaVersion(mcVersion string) int
}

// Registry holds all available providers
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(p Provider) {
	r.providers[p.Name()] = p
}

// Get returns a provider by name
func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return p, nil
}

// List returns all registered provider names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// GetRecommendedJavaVersion returns the recommended Java version for a Minecraft version
func GetRecommendedJavaVersion(mcVersion string) int {
	// Parse version to determine Java requirement
	// MC 1.21+ requires Java 21
	// MC 1.20.5+ requires Java 21
	// MC 1.18 - 1.20.4 requires Java 17
	// MC 1.17 requires Java 16
	// Older versions can use Java 8

	major, minor, patch := parseMinecraftVersion(mcVersion)

	if major >= 1 {
		if minor >= 21 {
			return 21
		}
		if minor == 20 && patch >= 5 {
			return 21
		}
		if minor >= 18 {
			return 17
		}
		if minor >= 17 {
			return 16
		}
	}

	return 8
}

// parseMinecraftVersion parses a Minecraft version string into components
func parseMinecraftVersion(version string) (major, minor, patch int) {
	// Handle formats like "1.20.4", "1.20", "1.20.4-pre1", etc.
	var m, n, p int
	fmt.Sscanf(version, "%d.%d.%d", &m, &n, &p)
	if m == 0 {
		fmt.Sscanf(version, "%d.%d", &m, &n)
	}
	return m, n, p
}
