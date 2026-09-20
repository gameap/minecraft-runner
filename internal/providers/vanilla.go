package providers

import (
	"context"
	"fmt"

	"github.com/gameap/minecraft-runner/pkg/api/mojang"
)

// VanillaProvider provides vanilla Minecraft server downloads
type VanillaProvider struct {
	client *mojang.Client
}

// NewVanillaProvider creates a new vanilla provider
func NewVanillaProvider() *VanillaProvider {
	return &VanillaProvider{
		client: mojang.NewClient(),
	}
}

// Name returns the provider name
func (p *VanillaProvider) Name() string {
	return "vanilla"
}

// ListVersions returns available Minecraft versions
func (p *VanillaProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	manifest, err := p.client.GetVersionManifest(ctx)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(manifest.Versions))
	for _, v := range manifest.Versions {
		versions = append(versions, VersionInfo{
			MinecraftVersion: v.ID,
			IsStable:         v.Type == "release",
			ReleaseDate:      v.ReleaseTime,
			Type:             v.Type,
		})
	}

	return versions, nil
}

// ListModVersions returns mod versions (always empty for vanilla)
func (p *VanillaProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	// Vanilla doesn't have mod versions
	return []VersionInfo{}, nil
}

// GetLatestVersion returns the latest stable version
func (p *VanillaProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	manifest, err := p.client.GetVersionManifest(ctx)
	if err != nil {
		return nil, err
	}

	v := manifest.FindVersion(manifest.Latest.Release)
	if v == nil {
		return nil, fmt.Errorf("latest version not found in manifest")
	}

	return &VersionInfo{
		MinecraftVersion: v.ID,
		IsStable:         true,
		ReleaseDate:      v.ReleaseTime,
		Type:             v.Type,
	}, nil
}

// GetServerJar returns download info for a specific version
func (p *VanillaProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	manifest, err := p.client.GetVersionManifest(ctx)
	if err != nil {
		return nil, err
	}

	v := manifest.FindVersion(mcVersion)
	if v == nil {
		return nil, fmt.Errorf("version %s not found", mcVersion)
	}

	detail, err := p.client.GetVersionDetail(ctx, v.URL)
	if err != nil {
		return nil, err
	}

	if detail.Downloads.Server.URL == "" {
		return nil, fmt.Errorf("server download not available for version %s", mcVersion)
	}

	return &ServerJar{
		Version:         mcVersion,
		URL:             detail.Downloads.Server.URL,
		SHA1:            detail.Downloads.Server.SHA1,
		Filename:        fmt.Sprintf("minecraft_server.%s.jar", mcVersion),
		RequiresInstall: false,
	}, nil
}

// PostDownload handles post-download steps (none for vanilla)
func (p *VanillaProvider) PostDownload(_ context.Context, _ string, _ *ServerJar, _ string) error {
	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *VanillaProvider) GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return GetRecommendedJavaVersion(ctx, mcVersion)
}
