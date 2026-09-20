package providers

import (
	"context"
	"fmt"

	"github.com/gameap/minecraft-runner/pkg/api/purpur"
)

// purpurBuildProbes limits how far back a successful build is searched for
const purpurBuildProbes = 5

// PurpurProvider provides Purpur server downloads
type PurpurProvider struct {
	client *purpur.Client
}

// NewPurpurProvider creates a new Purpur provider
func NewPurpurProvider() *PurpurProvider {
	return &PurpurProvider{
		client: purpur.NewClient(),
	}
}

// Name returns the provider name
func (p *PurpurProvider) Name() string {
	return "purpur"
}

// ListVersions returns available Minecraft versions, newest first
func (p *PurpurProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	project, err := p.client.GetProject(ctx)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(project.Versions))
	for i := len(project.Versions) - 1; i >= 0; i-- {
		versions = append(versions, VersionInfo{
			MinecraftVersion: project.Versions[i],
			IsStable:         !isPreRelease(project.Versions[i]),
			Type:             "release",
		})
	}

	return versions, nil
}

// ListModVersions returns builds for a specific Minecraft version, newest first
func (p *PurpurProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	version, err := p.client.GetVersion(ctx, mcVersion)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(version.Builds.All))
	for i := len(version.Builds.All) - 1; i >= 0; i-- {
		versions = append(versions, VersionInfo{
			MinecraftVersion: mcVersion,
			ModVersion:       version.Builds.All[i],
			IsStable:         true,
			Type:             "build",
		})
	}

	return versions, nil
}

// GetLatestVersion returns the Minecraft version Purpur currently recommends
func (p *PurpurProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	project, err := p.client.GetProject(ctx)
	if err != nil {
		return nil, err
	}

	latest := project.Metadata.Current
	if latest == "" && len(project.Versions) > 0 {
		latest = project.Versions[len(project.Versions)-1]
	}
	if latest == "" {
		return nil, fmt.Errorf("no versions available")
	}

	return &VersionInfo{
		MinecraftVersion: latest,
		IsStable:         true,
		Type:             "release",
	}, nil
}

// GetServerJar returns download info for a specific version
func (p *PurpurProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	build, err := p.resolveBuild(ctx, mcVersion, modVersion)
	if err != nil {
		return nil, err
	}

	return &ServerJar{
		Version:    mcVersion,
		ModVersion: build.Build,
		URL:        p.client.GetDownloadURL(mcVersion, build.Build),
		MD5:        build.MD5,
		Filename:   fmt.Sprintf("purpur-%s-%s.jar", mcVersion, build.Build),
	}, nil
}

// resolveBuild returns the requested build, or the newest one that produced a JAR
func (p *PurpurProvider) resolveBuild(ctx context.Context, mcVersion, modVersion string) (*purpur.Build, error) {
	if modVersion != "" {
		build, err := p.client.GetBuild(ctx, mcVersion, modVersion)
		if err != nil {
			return nil, err
		}
		if !build.IsSuccessful() {
			return nil, fmt.Errorf("build %s of Purpur %s has no JAR (result: %s)", modVersion, mcVersion, build.Result)
		}
		return build, nil
	}

	version, err := p.client.GetVersion(ctx, mcVersion)
	if err != nil {
		return nil, err
	}

	all := version.Builds.All
	for i := len(all) - 1; i >= 0 && i >= len(all)-purpurBuildProbes; i-- {
		build, err := p.client.GetBuild(ctx, mcVersion, all[i])
		if err != nil {
			return nil, err
		}
		if build.IsSuccessful() {
			return build, nil
		}
	}

	return nil, fmt.Errorf("no successful Purpur build found for %s", mcVersion)
}

// PostDownload handles post-download steps (none for Purpur)
func (p *PurpurProvider) PostDownload(_ context.Context, _ string, _ *ServerJar, _ string) error {
	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *PurpurProvider) GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return GetRecommendedJavaVersion(ctx, mcVersion)
}
