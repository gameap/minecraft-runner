package providers

import (
	"context"
	"fmt"
)

// CraftBukkitProvider suggests Paper as a replacement for CraftBukkit
type CraftBukkitProvider struct {
	paper *PaperProvider
}

// NewCraftBukkitProvider creates a new CraftBukkit provider
func NewCraftBukkitProvider() *CraftBukkitProvider {
	return &CraftBukkitProvider{
		paper: NewPaperProvider(),
	}
}

// Name returns the provider name
func (p *CraftBukkitProvider) Name() string {
	return "craftbukkit"
}

func (p *CraftBukkitProvider) printPaperSuggestion() {
	fmt.Println("\n[WARNING] CraftBukkit requires BuildTools compilation which is slow and complex.")
	fmt.Println("  Suggestion: Use Paper instead - it's a modern fork of CraftBukkit/Spigot")
	fmt.Println("  that's directly downloadable and offers better performance.")
	fmt.Println("  Run: mcrun run --mod=paper --version=<version>")
	fmt.Println()
}

// ListVersions returns available Minecraft versions
func (p *CraftBukkitProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	p.printPaperSuggestion()
	return p.paper.ListVersions(ctx)
}

// ListModVersions returns mod versions for a specific Minecraft version
func (p *CraftBukkitProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	p.printPaperSuggestion()
	return p.paper.ListModVersions(ctx, mcVersion)
}

// GetLatestVersion returns the latest stable version
func (p *CraftBukkitProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	p.printPaperSuggestion()
	return p.paper.GetLatestVersion(ctx)
}

// GetServerJar returns download info for a specific version
func (p *CraftBukkitProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	p.printPaperSuggestion()
	fmt.Println("Downloading Paper instead of CraftBukkit...")
	return p.paper.GetServerJar(ctx, mcVersion, modVersion)
}

// PostDownload handles post-download steps
func (p *CraftBukkitProvider) PostDownload(ctx context.Context, jarPath string, javaPath string) error {
	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *CraftBukkitProvider) GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return GetRecommendedJavaVersion(ctx, mcVersion)
}
