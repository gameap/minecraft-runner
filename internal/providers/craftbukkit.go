package providers

import (
	"context"
	"fmt"

	"github.com/fatih/color"
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
	color.Yellow("\n⚠ CraftBukkit requires BuildTools compilation which is slow and complex.")
	color.Cyan("  Suggestion: Use Paper instead - it's a modern fork of CraftBukkit/Spigot")
	color.Cyan("  that's directly downloadable and offers better performance.")
	color.Cyan("  Run: mcrun run --mod=paper --version=<version>\n")
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
func (p *CraftBukkitProvider) GetRecommendedJavaVersion(mcVersion string) int {
	return GetRecommendedJavaVersion(mcVersion)
}
