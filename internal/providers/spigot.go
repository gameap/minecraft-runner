package providers

import (
	"context"
	"fmt"

	"github.com/fatih/color"
)

// SpigotProvider suggests Paper as a replacement for Spigot
type SpigotProvider struct {
	paper *PaperProvider
}

// NewSpigotProvider creates a new Spigot provider
func NewSpigotProvider() *SpigotProvider {
	return &SpigotProvider{
		paper: NewPaperProvider(),
	}
}

// Name returns the provider name
func (p *SpigotProvider) Name() string {
	return "spigot"
}

func (p *SpigotProvider) printPaperSuggestion() {
	color.Yellow("\n⚠ Spigot requires BuildTools compilation which is slow and complex.")
	color.Cyan("  Suggestion: Use Paper instead - it's a drop-in replacement for Spigot")
	color.Cyan("  that's directly downloadable and offers better performance.")
	color.Cyan("  Run: mcrun run --mod=paper --version=<version>\n")
}

// ListVersions returns available Minecraft versions
func (p *SpigotProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	p.printPaperSuggestion()
	return p.paper.ListVersions(ctx)
}

// ListModVersions returns mod versions for a specific Minecraft version
func (p *SpigotProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	p.printPaperSuggestion()
	return p.paper.ListModVersions(ctx, mcVersion)
}

// GetLatestVersion returns the latest stable version
func (p *SpigotProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	p.printPaperSuggestion()
	return p.paper.GetLatestVersion(ctx)
}

// GetServerJar returns download info for a specific version
func (p *SpigotProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	p.printPaperSuggestion()
	fmt.Println("Downloading Paper instead of Spigot...")
	return p.paper.GetServerJar(ctx, mcVersion, modVersion)
}

// PostDownload handles post-download steps
func (p *SpigotProvider) PostDownload(ctx context.Context, jarPath string, javaPath string) error {
	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *SpigotProvider) GetRecommendedJavaVersion(mcVersion string) int {
	return GetRecommendedJavaVersion(mcVersion)
}
