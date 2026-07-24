package providers

import (
	"context"
	"fmt"
)

// CauldronProvider handles legacy Cauldron servers
type CauldronProvider struct{}

// NewCauldronProvider creates a new Cauldron provider
func NewCauldronProvider() *CauldronProvider {
	return &CauldronProvider{}
}

// Name returns the provider name
func (p *CauldronProvider) Name() string {
	return "cauldron"
}

func (p *CauldronProvider) printDeprecationWarning() {
	fmt.Println("\n[WARNING] Cauldron is a legacy project (last version: MC 1.7.10)")
	fmt.Println("  It is no longer maintained and may have security vulnerabilities.")
	fmt.Println("\n  Modern alternatives:")
	fmt.Println("  - Mohist (Forge + Bukkit for modern MC versions)")
	fmt.Println("  - SpongeForge (Forge + Sponge API)")
	fmt.Println("  - Magma (Forge + Bukkit/Spigot)")
	fmt.Println()
}

// ListVersions returns available Minecraft versions
func (p *CauldronProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	p.printDeprecationWarning()

	// Cauldron only supports these versions
	return []VersionInfo{
		{MinecraftVersion: "1.7.10", IsStable: true, Type: "legacy"},
		{MinecraftVersion: "1.7.2", IsStable: true, Type: "legacy"},
		{MinecraftVersion: "1.6.4", IsStable: true, Type: "legacy"},
	}, nil
}

// ListModVersions returns mod versions for a specific Minecraft version
func (p *CauldronProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	p.printDeprecationWarning()
	return []VersionInfo{}, nil
}

// GetLatestVersion returns the latest stable version
func (p *CauldronProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	p.printDeprecationWarning()
	return &VersionInfo{
		MinecraftVersion: "1.7.10",
		IsStable:         true,
		Type:             "legacy",
	}, nil
}

// GetServerJar returns download info for a specific version
func (p *CauldronProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	p.printDeprecationWarning()

	// Known Cauldron download URLs (archived)
	// Note: These may not be available as the project is discontinued
	urls := map[string]string{
		"1.7.10": "https://sourceforge.net/projects/cauldron-unofficial/files/1.7.10/cauldron-1.7.10-1.1388.1.0.jar/download",
	}

	url, ok := urls[mcVersion]
	if !ok {
		return nil, fmt.Errorf("Cauldron version %s not available. Cauldron only supports MC 1.6.4-1.7.10", mcVersion)
	}

	return &ServerJar{
		Version:         mcVersion,
		URL:             url,
		Filename:        fmt.Sprintf("cauldron-%s.jar", mcVersion),
		RequiresInstall: false,
	}, nil
}

// PostDownload handles post-download steps
func (p *CauldronProvider) PostDownload(ctx context.Context, jarPath string, javaPath string) error {
	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *CauldronProvider) GetRecommendedJavaVersion(_ context.Context, _ string) int {
	// Cauldron is old and requires Java 8
	return 8
}
