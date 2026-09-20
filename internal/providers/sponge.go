package providers

import (
	"context"
	"fmt"
	"path"

	"github.com/gameap/minecraft-runner/pkg/api/sponge"
)

// spongeVersionProbes limits how many Minecraft versions are inspected when
// looking for the newest one that already has a recommended build
const spongeVersionProbes = 6

// SpongeVanillaProvider provides SpongeVanilla server downloads
type SpongeVanillaProvider struct {
	client *sponge.Client
}

// NewSpongeVanillaProvider creates a new SpongeVanilla provider
func NewSpongeVanillaProvider() *SpongeVanillaProvider {
	return &SpongeVanillaProvider{
		client: sponge.NewClient(),
	}
}

// Name returns the provider name
func (p *SpongeVanillaProvider) Name() string {
	return sponge.ArtifactVanilla
}

// ListVersions returns available Minecraft versions, newest first
func (p *SpongeVanillaProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	mcVersions, err := p.client.GetMinecraftVersions(ctx, sponge.ArtifactVanilla)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(mcVersions))
	for _, version := range mcVersions {
		vType := "release"
		if isPreRelease(version) {
			vType = "snapshot"
		}

		versions = append(versions, VersionInfo{
			MinecraftVersion: version,
			IsStable:         vType == "release",
			Type:             vType,
		})
	}

	return versions, nil
}

// ListModVersions returns SpongeVanilla builds for a specific Minecraft version, newest first
func (p *SpongeVanillaProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	builds, err := p.client.GetVersions(ctx, sponge.ArtifactVanilla, mcVersion, false)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(builds))
	for _, build := range builds {
		versions = append(versions, VersionInfo{
			MinecraftVersion: mcVersion,
			ModVersion:       build.Version,
			IsStable:         build.Recommended,
			Type:             "build",
		})
	}

	return versions, nil
}

// GetLatestVersion returns the newest Minecraft release with a recommended build
func (p *SpongeVanillaProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	mcVersions, err := p.client.GetMinecraftVersions(ctx, sponge.ArtifactVanilla)
	if err != nil {
		return nil, err
	}

	var releases []string
	for _, version := range mcVersions {
		if !isPreRelease(version) {
			releases = append(releases, version)
		}
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no versions available")
	}

	probes := releases
	if len(probes) > spongeVersionProbes {
		probes = probes[:spongeVersionProbes]
	}

	for _, version := range probes {
		builds, err := p.client.GetVersions(ctx, sponge.ArtifactVanilla, version, true)
		if err != nil {
			return nil, err
		}
		if len(builds) > 0 {
			return &VersionInfo{MinecraftVersion: version, IsStable: true, Type: "release"}, nil
		}
	}

	return &VersionInfo{MinecraftVersion: releases[0], Type: "release"}, nil
}

// GetServerJar returns download info for a specific version
func (p *SpongeVanillaProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	version := modVersion
	if version == "" {
		build, err := p.latestBuild(ctx, mcVersion)
		if err != nil {
			return nil, err
		}
		version = build
	}

	asset, err := p.client.GetServerAsset(ctx, sponge.ArtifactVanilla, version)
	if err != nil {
		return nil, err
	}

	return &ServerJar{
		Version:    mcVersion,
		ModVersion: version,
		URL:        asset.DownloadURL,
		SHA1:       asset.SHA1,
		MD5:        asset.MD5,
		Filename:   path.Base(asset.DownloadURL),
	}, nil
}

// latestBuild returns the newest recommended build, falling back to the newest build of any kind
func (p *SpongeVanillaProvider) latestBuild(ctx context.Context, mcVersion string) (string, error) {
	recommended, err := p.client.GetVersions(ctx, sponge.ArtifactVanilla, mcVersion, true)
	if err != nil {
		return "", err
	}
	if len(recommended) > 0 {
		return recommended[0].Version, nil
	}

	builds, err := p.client.GetVersions(ctx, sponge.ArtifactVanilla, mcVersion, false)
	if err != nil {
		return "", err
	}
	if len(builds) == 0 {
		return "", fmt.Errorf("no SpongeVanilla builds found for Minecraft %s", mcVersion)
	}

	fmt.Printf("[WARNING] SpongeVanilla has no recommended build for Minecraft %s yet, using %s\n",
		mcVersion, builds[0].Version)

	return builds[0].Version, nil
}

// PostDownload handles post-download steps (none for SpongeVanilla)
func (p *SpongeVanillaProvider) PostDownload(_ context.Context, _ string, _ *ServerJar, _ string) error {
	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *SpongeVanillaProvider) GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return GetRecommendedJavaVersion(ctx, mcVersion)
}
