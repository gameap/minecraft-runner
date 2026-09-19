package providers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gameap/minecraft-runner/pkg/api/paper"
)

// WaterfallProvider provides Waterfall proxy server downloads
type WaterfallProvider struct {
	client *paper.Client
}

// NewWaterfallProvider creates a new Waterfall provider
func NewWaterfallProvider() *WaterfallProvider {
	return &WaterfallProvider{
		client: paper.NewClient(),
	}
}

// Name returns the provider name
func (p *WaterfallProvider) Name() string {
	return "waterfall"
}

// ListVersions returns available Waterfall versions (proxy versions, not MC versions)
func (p *WaterfallProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	project, err := p.client.GetProject(ctx, "waterfall")
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(project.Versions))
	for i := len(project.Versions) - 1; i >= 0; i-- {
		versions = append(versions, VersionInfo{
			MinecraftVersion: project.Versions[i],
			IsStable:         true,
			Type:             "proxy",
		})
	}

	return versions, nil
}

// ListModVersions returns builds for a specific Waterfall version
func (p *WaterfallProvider) ListModVersions(ctx context.Context, version string) ([]VersionInfo, error) {
	vb, err := p.client.GetVersionBuilds(ctx, "waterfall", version)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(*vb))
	for _, build := range *vb {
		versions = append(versions, VersionInfo{
			MinecraftVersion: version,
			ModVersion:       strconv.Itoa(build.ID),
			IsStable:         build.Channel == "STABLE",
			Type:             "build",
		})
	}

	return versions, nil
}

// GetLatestVersion returns the latest Waterfall version
func (p *WaterfallProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	project, err := p.client.GetProject(ctx, "waterfall")
	if err != nil {
		return nil, err
	}

	if len(project.Versions) == 0 {
		return nil, fmt.Errorf("no versions available")
	}

	latest := project.Versions[len(project.Versions)-1]

	return &VersionInfo{
		MinecraftVersion: latest,
		IsStable:         true,
		Type:             "proxy",
	}, nil
}

// GetServerJar returns download info for Waterfall
func (p *WaterfallProvider) GetServerJar(ctx context.Context, version, modVersion string) (*ServerJar, error) {
	var buildInfo *paper.BuildInfo
	var err error

	if modVersion != "" {
		build, parseErr := strconv.Atoi(modVersion)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid build number: %s", modVersion)
		}
		buildInfo, err = p.client.GetBuildInfo(ctx, "waterfall", version, build)
	} else {
		buildInfo, err = p.client.GetLatestBuild(ctx, "waterfall", version)
	}

	if err != nil {
		return nil, err
	}

	downloadURL := p.client.GetDownloadURL("waterfall", version, buildInfo.Build, buildInfo.Downloads.Application.Name)

	return &ServerJar{
		Version:         version,
		ModVersion:      strconv.Itoa(buildInfo.Build),
		URL:             downloadURL,
		SHA256:          buildInfo.Downloads.Application.SHA256,
		Filename:        buildInfo.Downloads.Application.Name,
		RequiresInstall: false,
	}, nil
}

// PostDownload handles post-download steps (none for Waterfall)
func (p *WaterfallProvider) PostDownload(ctx context.Context, jarPath string, javaPath string) error {
	return nil
}

// GetRecommendedJavaVersion returns Java 17 for modern Waterfall
func (p *WaterfallProvider) GetRecommendedJavaVersion(_ context.Context, _ string) int {
	return 17
}
