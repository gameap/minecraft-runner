package providers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gameap/minecraft-runner/pkg/api/mohist"
)

// mohistVersionProbes limits how many Minecraft versions are inspected when
// looking for the newest one that has a build
const mohistVersionProbes = 6

// MohistProvider provides the MohistMC hybrid servers: Mohist (Forge + Bukkit)
// and Banner (Fabric + Bukkit)
type MohistProvider struct {
	client  *mohist.Client
	project string
}

// NewMohistProvider creates a new Mohist provider
func NewMohistProvider() *MohistProvider {
	return &MohistProvider{client: mohist.NewClient(), project: mohist.ProjectMohist}
}

// NewBannerProvider creates a new Banner provider
func NewBannerProvider() *MohistProvider {
	return &MohistProvider{client: mohist.NewClient(), project: mohist.ProjectBanner}
}

// Name returns the provider name
func (p *MohistProvider) Name() string {
	return p.project
}

// ListVersions returns available Minecraft versions, newest first
func (p *MohistProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
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
func (p *MohistProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	builds, err := p.client.GetBuilds(ctx, p.project, mcVersion)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(builds))
	for _, build := range builds {
		versions = append(versions, VersionInfo{
			MinecraftVersion: mcVersion,
			ModVersion:       strconv.Itoa(build.ID),
			IsStable:         true,
			Type:             "build",
		})
	}

	return versions, nil
}

// GetLatestVersion returns the newest Minecraft release that has a build. The
// API also lists versions that are announced but not built yet.
func (p *MohistProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	mcVersions, err := p.sortedVersions(ctx)
	if err != nil {
		return nil, err
	}

	probes := 0
	for _, version := range mcVersions {
		if isPreRelease(version) {
			continue
		}

		if probes++; probes > mohistVersionProbes {
			break
		}

		builds, err := p.client.GetBuilds(ctx, p.project, version)
		if err != nil {
			return nil, err
		}

		if _, err := pickMohistBuild(builds, version, ""); err == nil {
			return &VersionInfo{MinecraftVersion: version, IsStable: true, Type: "release"}, nil
		}
	}

	return nil, fmt.Errorf("no %s builds available", p.project)
}

// GetServerJar returns download info for a specific version
func (p *MohistProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	builds, err := p.client.GetBuilds(ctx, p.project, mcVersion)
	if err != nil {
		return nil, err
	}

	build, err := pickMohistBuild(builds, mcVersion, modVersion)
	if err != nil {
		return nil, err
	}

	return &ServerJar{
		Version:    mcVersion,
		ModVersion: strconv.Itoa(build.ID),
		URL:        p.client.GetDownloadURL(p.project, mcVersion, build.ID),
		SHA256:     build.FileSHA256,
		Filename:   fmt.Sprintf("%s-%s-%d.jar", p.project, mcVersion, build.ID),
		ServerArgs: []string{"nogui"},
	}, nil
}

// pickMohistBuild returns the requested build, or the newest one
func pickMohistBuild(builds []mohist.Build, mcVersion, modVersion string) (*mohist.Build, error) {
	if len(builds) == 0 {
		return nil, fmt.Errorf("no builds available for version %s", mcVersion)
	}

	if modVersion == "" {
		return &builds[0], nil
	}

	number, err := strconv.Atoi(modVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid build number: %s", modVersion)
	}

	for i := range builds {
		if builds[i].ID == number {
			return &builds[i], nil
		}
	}

	return nil, fmt.Errorf("build %d not found for version %s", number, mcVersion)
}

func (p *MohistProvider) sortedVersions(ctx context.Context) ([]string, error) {
	mcVersions, err := p.client.GetVersions(ctx, p.project)
	if err != nil {
		return nil, err
	}

	versions := append([]string{}, mcVersions...)
	sortMCVersionsDesc(versions)

	return versions, nil
}

// PostDownload handles post-download steps: none, the hybrids install their
// libraries themselves on the first start
func (p *MohistProvider) PostDownload(_ context.Context, _ string, _ *ServerJar, _ string) error {
	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *MohistProvider) GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return GetRecommendedJavaVersion(ctx, mcVersion)
}
