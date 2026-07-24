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
	GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int
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
