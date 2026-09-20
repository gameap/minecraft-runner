package providers

import (
	"context"
	"fmt"
)

// cauldronMinecraftVersion is the last Minecraft version Cauldron supported
const cauldronMinecraftVersion = "1.7.10"

// CauldronProvider keeps the discontinued Cauldron server type working. Its
// downloads are gone, so it serves Mohist 1.7.10 instead: the maintained
// Forge + Bukkit hybrid for the same Minecraft version.
type CauldronProvider struct {
	mohist *MohistProvider
}

// NewCauldronProvider creates a new Cauldron provider
func NewCauldronProvider() *CauldronProvider {
	return &CauldronProvider{
		mohist: NewMohistProvider(),
	}
}

// Name returns the provider name
func (p *CauldronProvider) Name() string {
	return "cauldron"
}

func (p *CauldronProvider) printDeprecationWarning() {
	fmt.Println("\n[WARNING] Cauldron is discontinued and its downloads are no longer available.")
	fmt.Printf("  Using Mohist %s instead - a maintained Forge + Bukkit hybrid.\n", cauldronMinecraftVersion)
	fmt.Printf("  Run: mcrun run --mod=mohist --version=%s\n\n", cauldronMinecraftVersion)
}

// ListVersions returns the only Minecraft version Cauldron is served for
func (p *CauldronProvider) ListVersions(_ context.Context) ([]VersionInfo, error) {
	p.printDeprecationWarning()

	return []VersionInfo{
		{MinecraftVersion: cauldronMinecraftVersion, IsStable: true, Type: "legacy"},
	}, nil
}

// ListModVersions returns the Mohist builds that stand in for Cauldron
func (p *CauldronProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	p.printDeprecationWarning()

	if mcVersion != cauldronMinecraftVersion {
		return []VersionInfo{}, nil
	}

	return p.mohist.ListModVersions(ctx, mcVersion)
}

// GetLatestVersion returns the latest stable version
func (p *CauldronProvider) GetLatestVersion(_ context.Context) (*VersionInfo, error) {
	return &VersionInfo{
		MinecraftVersion: cauldronMinecraftVersion,
		IsStable:         true,
		Type:             "legacy",
	}, nil
}

// GetServerJar returns download info for a specific version
func (p *CauldronProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	p.printDeprecationWarning()

	if mcVersion != cauldronMinecraftVersion {
		return nil, fmt.Errorf("cauldron is only available for Minecraft %s, not %s",
			cauldronMinecraftVersion, mcVersion)
	}

	return p.mohist.GetServerJar(ctx, mcVersion, modVersion)
}

// PostDownload handles post-download steps
func (p *CauldronProvider) PostDownload(ctx context.Context, dir string, jar *ServerJar, javaPath string) error {
	return p.mohist.PostDownload(ctx, dir, jar, javaPath)
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *CauldronProvider) GetRecommendedJavaVersion(_ context.Context, _ string) int {
	return 8
}
