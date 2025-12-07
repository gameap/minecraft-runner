package providers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gameap/minecraft-runner/pkg/api/jenkins"
)

// BungeecordProvider provides Bungeecord proxy server downloads
type BungeecordProvider struct {
	client *jenkins.Client
}

// NewBungeecordProvider creates a new Bungeecord provider
func NewBungeecordProvider() *BungeecordProvider {
	return &BungeecordProvider{
		client: jenkins.NewBungeecordClient(),
	}
}

// Name returns the provider name
func (p *BungeecordProvider) Name() string {
	return "bungeecord"
}

// ListVersions returns available builds (Bungeecord doesn't have versions, only builds)
func (p *BungeecordProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	builds, err := p.client.GetBuilds(ctx)
	if err != nil {
		return nil, err
	}

	// Return latest 20 builds
	limit := 20
	if len(builds) < limit {
		limit = len(builds)
	}

	versions := make([]VersionInfo, 0, limit)
	for i := 0; i < limit; i++ {
		versions = append(versions, VersionInfo{
			MinecraftVersion: "latest",
			ModVersion:       strconv.Itoa(builds[i].Number),
			IsStable:         true,
			Type:             "build",
		})
	}

	return versions, nil
}

// ListModVersions returns build numbers
func (p *BungeecordProvider) ListModVersions(ctx context.Context, _ string) ([]VersionInfo, error) {
	return p.ListVersions(ctx)
}

// GetLatestVersion returns the latest successful build
func (p *BungeecordProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	build, err := p.client.GetLastSuccessfulBuild(ctx)
	if err != nil {
		return nil, err
	}

	return &VersionInfo{
		MinecraftVersion: "latest",
		ModVersion:       strconv.Itoa(build.Number),
		IsStable:         true,
		Type:             "build",
	}, nil
}

// GetServerJar returns download info for Bungeecord
func (p *BungeecordProvider) GetServerJar(ctx context.Context, _, modVersion string) (*ServerJar, error) {
	var build *jenkins.BuildInfo
	var err error

	if modVersion != "" {
		buildNum, parseErr := strconv.Atoi(modVersion)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid build number: %s", modVersion)
		}
		build, err = p.client.GetBuild(ctx, buildNum)
	} else {
		build, err = p.client.GetLastSuccessfulBuild(ctx)
	}

	if err != nil {
		return nil, err
	}

	downloadURL := p.client.GetArtifactURL(build.Number, "bootstrap/target/BungeeCord.jar")

	return &ServerJar{
		Version:         "latest",
		ModVersion:      strconv.Itoa(build.Number),
		URL:             downloadURL,
		Filename:        fmt.Sprintf("BungeeCord-%d.jar", build.Number),
		RequiresInstall: false,
	}, nil
}

// PostDownload handles post-download steps (none for Bungeecord)
func (p *BungeecordProvider) PostDownload(ctx context.Context, jarPath string, javaPath string) error {
	return nil
}

// GetRecommendedJavaVersion returns Java 17 for modern Bungeecord
func (p *BungeecordProvider) GetRecommendedJavaVersion(_ string) int {
	return 17
}
