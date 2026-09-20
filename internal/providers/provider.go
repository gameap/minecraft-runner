package providers

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// ServerJar contains information about a downloadable server JAR
type ServerJar struct {
	Version         string   // Minecraft version
	ModVersion      string   // Mod-specific version (e.g., Paper build number)
	URL             string   // Download URL
	SHA256          string   // SHA256 hash for verification (optional)
	SHA1            string   // SHA1 hash for verification (optional)
	MD5             string   // MD5 hash for verification (optional)
	Filename        string   // Suggested filename
	RequiresInstall bool     // True if post-download installation is needed (e.g., Forge)
	ServerArgs      []string // Program arguments; empty means the runner default
}

// LaunchTarget describes how an installed server is started.
// Paths are relative to the server directory.
type LaunchTarget struct {
	Jar      string   `json:"jar,omitempty"`       // Started with -jar
	ArgFiles []string `json:"arg_files,omitempty"` // JVM @argfiles (Forge 1.17+, NeoForge)
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
	PostDownload(ctx context.Context, dir string, jar *ServerJar, javaPath string) error

	// GetRecommendedJavaVersion returns the recommended Java version for this MC version
	GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int
}

// LaunchResolver is implemented by providers whose installed server is not
// started with `-jar <downloaded file>`. ResolveLaunch looks only at
// version-qualified paths, so it needs no record of earlier runs, and returns
// nil when that exact version is not installed in dir.
type LaunchResolver interface {
	ResolveLaunch(dir string, jar *ServerJar) *LaunchTarget
}

// ProxyProvider is implemented by proxy servers, which have neither an EULA
// nor a server.properties
type ProxyProvider interface {
	IsProxy() bool
}

// ListenArgsProvider is implemented by servers that take their listen port on
// the command line instead of from server.properties
type ListenArgsProvider interface {
	ListenArgs(port int) []string
}

// IsProxy reports whether the provider serves a proxy rather than a game server
func IsProxy(p Provider) bool {
	proxy, ok := p.(ProxyProvider)
	return ok && proxy.IsProxy()
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

// List returns all registered provider names in alphabetical order
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
