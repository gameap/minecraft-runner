package providers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gameap/minecraft-runner/pkg/api/paper"
)

// PaperProvider provides Paper server downloads
type PaperProvider struct {
	client *paper.Client
}

// NewPaperProvider creates a new Paper provider
func NewPaperProvider() *PaperProvider {
	return &PaperProvider{
		client: paper.NewClient(),
	}
}

// Name returns the provider name
func (p *PaperProvider) Name() string {
	return "paper"
}

// ListVersions returns available Minecraft versions
func (p *PaperProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	project, err := p.client.GetProject(ctx, "paper")
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(project.Versions))
	for i := len(project.Versions) - 1; i >= 0; i-- {
		versions = append(versions, VersionInfo{
			MinecraftVersion: project.Versions[i],
			IsStable:         true,
			Type:             "release",
		})
	}

	return versions, nil
}

// ListModVersions returns builds for a specific Minecraft version
func (p *PaperProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	vb, err := p.client.GetVersionBuilds(ctx, "paper", mcVersion)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(*vb))
	for _, build := range *vb {
		versions = append(versions, VersionInfo{
			MinecraftVersion: mcVersion,
			ModVersion:       strconv.Itoa(build.ID),
			IsStable:         build.Channel == "STABLE",
			Type:             "build",
		})
	}

	return versions, nil
}

// GetLatestVersion returns the latest stable version
func (p *PaperProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	project, err := p.client.GetProject(ctx, "paper")
	if err != nil {
		return nil, err
	}

	if len(project.Versions) == 0 {
		return nil, fmt.Errorf("no versions available")
	}

	// Latest version is the last in the list
	latest := project.Versions[len(project.Versions)-1]

	return &VersionInfo{
		MinecraftVersion: latest,
		IsStable:         true,
		Type:             "release",
	}, nil
}

// GetServerJar returns download info for a specific version
func (p *PaperProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	var buildInfo *paper.BuildInfo
	var err error

	if modVersion != "" {
		// Get specific build
		build, parseErr := strconv.Atoi(modVersion)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid build number: %s", modVersion)
		}
		buildInfo, err = p.client.GetBuildInfo(ctx, "paper", mcVersion, build)
	} else {
		// Get latest build
		buildInfo, err = p.client.GetLatestBuild(ctx, "paper", mcVersion)
	}

	if err != nil {
		return nil, err
	}

	downloadURL := buildInfo.Downloads.Application.URL

	return &ServerJar{
		Version:         mcVersion,
		ModVersion:      strconv.Itoa(buildInfo.ID),
		URL:             downloadURL,
		SHA256:          buildInfo.Downloads.Application.SHA256,
		Filename:        buildInfo.Downloads.Application.Name,
		RequiresInstall: false,
	}, nil
}

// PostDownload handles post-download steps (none for Paper)
func (p *PaperProvider) PostDownload(ctx context.Context, jarPath string, javaPath string) error {
	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *PaperProvider) GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return GetRecommendedJavaVersion(ctx, mcVersion)
}
