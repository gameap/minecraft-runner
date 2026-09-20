package providers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gameap/minecraft-runner/pkg/api/leaf"
)

// LeafProvider provides Leaf server downloads
type LeafProvider struct {
	client *leaf.Client
}

// NewLeafProvider creates a new Leaf provider
func NewLeafProvider() *LeafProvider {
	return &LeafProvider{
		client: leaf.NewClient(),
	}
}

// Name returns the provider name
func (p *LeafProvider) Name() string {
	return "leaf"
}

// ListVersions returns available Minecraft versions, newest first
func (p *LeafProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	mcVersions, err := p.sortedVersions(ctx)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(mcVersions))
	for _, version := range mcVersions {
		versions = append(versions, VersionInfo{
			MinecraftVersion: version,
			IsStable:         !isPreRelease(version),
			Type:             "release",
		})
	}

	return versions, nil
}

// ListModVersions returns builds for a specific Minecraft version, newest first
func (p *LeafProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	builds, err := p.client.GetBuilds(ctx, mcVersion)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(builds))
	for i := len(builds) - 1; i >= 0; i-- {
		versions = append(versions, VersionInfo{
			MinecraftVersion: mcVersion,
			ModVersion:       strconv.Itoa(builds[i].Build),
			IsStable:         builds[i].IsStable(),
			Type:             "build",
		})
	}

	return versions, nil
}

// GetLatestVersion returns the newest Minecraft release Leaf was built for
func (p *LeafProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	mcVersions, err := p.sortedVersions(ctx)
	if err != nil {
		return nil, err
	}

	for _, version := range mcVersions {
		if !isPreRelease(version) {
			return &VersionInfo{MinecraftVersion: version, IsStable: true, Type: "release"}, nil
		}
	}

	if len(mcVersions) == 0 {
		return nil, fmt.Errorf("no versions available")
	}

	return &VersionInfo{MinecraftVersion: mcVersions[0], Type: "release"}, nil
}

// GetServerJar returns download info for a specific version
func (p *LeafProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	builds, err := p.client.GetBuilds(ctx, mcVersion)
	if err != nil {
		return nil, err
	}

	build, err := pickLeafBuild(builds, mcVersion, modVersion)
	if err != nil {
		return nil, err
	}

	if !build.IsStable() {
		fmt.Printf("[WARNING] Leaf %s has no build in the default channel, using %s build %d\n",
			mcVersion, build.Channel, build.Build)
	}

	return &ServerJar{
		Version:    mcVersion,
		ModVersion: strconv.Itoa(build.Build),
		URL:        p.client.GetDownloadURL(mcVersion, build.Build, build.Downloads.Primary.Name),
		SHA256:     build.Downloads.Primary.SHA256,
		Filename:   build.Downloads.Primary.Name,
	}, nil
}

// pickLeafBuild returns the requested build, or the newest one of the default
// channel, falling back to the newest build of any channel
func pickLeafBuild(builds []leaf.Build, mcVersion, modVersion string) (*leaf.Build, error) {
	if modVersion != "" {
		number, err := strconv.Atoi(modVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid build number: %s", modVersion)
		}

		for i := range builds {
			if builds[i].Build == number {
				return &builds[i], nil
			}
		}

		return nil, fmt.Errorf("build %d not found for version %s", number, mcVersion)
	}

	for _, stableOnly := range []bool{true, false} {
		for i := len(builds) - 1; i >= 0; i-- {
			if stableOnly && !builds[i].IsStable() {
				continue
			}
			if builds[i].Downloads.Primary.Name != "" {
				return &builds[i], nil
			}
		}
	}

	return nil, fmt.Errorf("no builds available for version %s", mcVersion)
}

func (p *LeafProvider) sortedVersions(ctx context.Context) ([]string, error) {
	project, err := p.client.GetProject(ctx)
	if err != nil {
		return nil, err
	}

	versions := append([]string{}, project.Versions...)
	sortMCVersionsDesc(versions)

	return versions, nil
}

// PostDownload handles post-download steps (none for Leaf)
func (p *LeafProvider) PostDownload(_ context.Context, _ string, _ *ServerJar, _ string) error {
	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *LeafProvider) GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return GetRecommendedJavaVersion(ctx, mcVersion)
}
