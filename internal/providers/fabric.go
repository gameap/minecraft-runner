package providers

import (
	"context"
	"fmt"

	"github.com/gameap/minecraft-runner/pkg/api/fabric"
)

// FabricProvider provides Fabric server downloads
type FabricProvider struct {
	client *fabric.Client
}

// NewFabricProvider creates a new Fabric provider
func NewFabricProvider() *FabricProvider {
	return &FabricProvider{
		client: fabric.NewClient(),
	}
}

// Name returns the provider name
func (p *FabricProvider) Name() string {
	return "fabric"
}

// ListVersions returns available Minecraft versions
func (p *FabricProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	gameVersions, err := p.client.GetGameVersions(ctx)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(gameVersions))
	for _, v := range gameVersions {
		versions = append(versions, VersionInfo{
			MinecraftVersion: v.Version,
			IsStable:         v.Stable,
			Type:             "release",
		})
	}

	return versions, nil
}

// ListModVersions returns loader versions for a specific Minecraft version
func (p *FabricProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	loaderVersions, err := p.client.GetLoaderVersions(ctx)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(loaderVersions))
	for _, v := range loaderVersions {
		versions = append(versions, VersionInfo{
			MinecraftVersion: mcVersion,
			ModVersion:       v.Version,
			IsStable:         v.Stable,
			Type:             "loader",
		})
	}

	return versions, nil
}

// GetLatestVersion returns the latest stable version
func (p *FabricProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	versions, err := p.client.GetGameVersions(ctx)
	if err != nil {
		return nil, err
	}

	for _, v := range versions {
		if v.Stable {
			return &VersionInfo{
				MinecraftVersion: v.Version,
				IsStable:         true,
				Type:             "release",
			}, nil
		}
	}

	if len(versions) > 0 {
		return &VersionInfo{
			MinecraftVersion: versions[0].Version,
			IsStable:         versions[0].Stable,
			Type:             "release",
		}, nil
	}

	return nil, fmt.Errorf("no versions available")
}

// GetServerJar returns download info for a specific version
func (p *FabricProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	// Get loader version
	loaderVersion := modVersion
	if loaderVersion == "" {
		loader, err := p.client.GetLatestStableLoader(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get loader version: %w", err)
		}
		loaderVersion = loader.Version
	}

	// Get installer version
	installer, err := p.client.GetLatestStableInstaller(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get installer version: %w", err)
	}

	downloadURL := p.client.GetServerJarURL(mcVersion, loaderVersion, installer.Version)

	return &ServerJar{
		Version:         mcVersion,
		ModVersion:      loaderVersion,
		URL:             downloadURL,
		Filename:        fmt.Sprintf("fabric-server-mc.%s-loader.%s-launcher.%s.jar", mcVersion, loaderVersion, installer.Version),
		RequiresInstall: false,
	}, nil
}

// PostDownload handles post-download steps (none for Fabric)
func (p *FabricProvider) PostDownload(ctx context.Context, jarPath string, javaPath string) error {
	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *FabricProvider) GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return GetRecommendedJavaVersion(ctx, mcVersion)
}
