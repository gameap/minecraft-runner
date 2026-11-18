package providers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gameap/minecraft-runner/internal/preseed"

	"github.com/gameap/minecraft-runner/pkg/api/paper"
)

// FillProvider serves the projects published through the PaperMC Fill API:
// Paper, Folia and the Velocity and Waterfall proxies
type FillProvider struct {
	client      *paper.Client
	name        string
	project     string
	proxy       bool
	versionType string
	javaVersion func(ctx context.Context, version string) int
}

// NewPaperProvider creates a new Paper provider
func NewPaperProvider() *FillProvider {
	return newFillProvider(paper.NewClient(), "paper", false, GetRecommendedJavaVersion)
}

// NewFoliaProvider creates a new Folia provider
func NewFoliaProvider() *FillProvider {
	return newFillProvider(paper.NewClient(), "folia", false, GetRecommendedJavaVersion)
}

// NewVelocityProvider creates a new Velocity proxy provider
func NewVelocityProvider() *FillProvider {
	return newFillProvider(paper.NewClient(), "velocity", true, fixedJavaVersion(25))
}

// NewWaterfallProvider creates a new Waterfall proxy provider
func NewWaterfallProvider() *FillProvider {
	return newFillProvider(paper.NewClient(), "waterfall", true, fixedJavaVersion(17))
}

func newFillProvider(
	client *paper.Client, project string, proxy bool, javaVersion func(context.Context, string) int,
) *FillProvider {
	versionType := "release"
	if proxy {
		versionType = "proxy"
	}

	return &FillProvider{
		client:      client,
		name:        project,
		project:     project,
		proxy:       proxy,
		versionType: versionType,
		javaVersion: javaVersion,
	}
}

func fixedJavaVersion(version int) func(context.Context, string) int {
	return func(context.Context, string) int {
		return version
	}
}

// Name returns the provider name
func (p *FillProvider) Name() string {
	return p.name
}

// IsProxy reports whether the project is a proxy rather than a game server
func (p *FillProvider) IsProxy() bool {
	return p.proxy
}

// ListenArgs returns the command line arguments that set the listen port.
// Only Velocity takes its port this way; Waterfall reads it from config.yml.
func (p *FillProvider) ListenArgs(port int) []string {
	if p.project != "velocity" || port <= 0 {
		return nil
	}
	return []string{"--port", strconv.Itoa(port)}
}

// ListVersions returns available versions, newest first
func (p *FillProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	project, err := p.client.GetProject(ctx, p.project)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(project.Versions))
	for _, version := range project.Versions {
		versions = append(versions, VersionInfo{
			MinecraftVersion: version,
			IsStable:         !isPreRelease(version),
			Type:             p.versionType,
		})
	}

	return versions, nil
}

// ListModVersions returns builds for a specific version
func (p *FillProvider) ListModVersions(ctx context.Context, version string) ([]VersionInfo, error) {
	builds, err := p.client.GetVersionBuilds(ctx, p.project, version)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(builds))
	for _, build := range builds {
		versions = append(versions, VersionInfo{
			MinecraftVersion: version,
			ModVersion:       strconv.Itoa(build.ID),
			IsStable:         build.IsStable(),
			Type:             "build",
		})
	}

	return versions, nil
}

// GetLatestVersion returns the newest version that has a stable build
func (p *FillProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	version, err := p.client.GetLatestStableVersion(ctx, p.project)
	if err != nil {
		return nil, err
	}

	return &VersionInfo{
		MinecraftVersion: version,
		IsStable:         true,
		Type:             p.versionType,
	}, nil
}

// GetServerJar returns download info for a specific version
func (p *FillProvider) GetServerJar(ctx context.Context, version, modVersion string) (*ServerJar, error) {
	var build *paper.BuildInfo
	var err error

	if modVersion != "" {
		number, parseErr := strconv.Atoi(modVersion)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid build number: %s", modVersion)
		}
		build, err = p.client.GetBuildInfo(ctx, p.project, version, number)
	} else {
		build, err = p.client.GetLatestBuild(ctx, p.project, version)
	}

	if err != nil {
		return nil, err
	}

	download, ok := build.ServerDownload()
	if !ok {
		return nil, fmt.Errorf("build %d of %s %s has no server download", build.ID, p.name, version)
	}

	if !build.IsStable() {
		fmt.Printf("[WARNING] %s %s has no stable build yet, using %s build %d\n",
			p.name, version, build.Channel, build.ID)
	}

	return &ServerJar{
		Version:    version,
		ModVersion: strconv.Itoa(build.ID),
		URL:        download.URL,
		SHA256:     download.Checksums.SHA256,
		Filename:   download.Name,
		Extras:     fillModules(build),
	}, nil
}

// fillModules lists the modules Waterfall would download on start
func fillModules(build *paper.BuildInfo) []preseed.File {
	var modules []preseed.File
	for key, download := range build.Downloads {
		name, ok := strings.CutPrefix(key, "module:")
		if !ok {
			continue
		}
		modules = append(modules, preseed.File{
			Path:   "modules/" + name + ".jar",
			URLs:   []string{download.URL},
			SHA256: download.Checksums.SHA256,
		})
	}

	sort.Slice(modules, func(i, j int) bool { return modules[i].Path < modules[j].Path })

	return modules
}

// PostDownload handles post-download steps (none for Fill projects)
func (p *FillProvider) PostDownload(_ context.Context, _ string, _ *ServerJar, _ string) error {
	return nil
}

// Prepare puts in place what the server downloads on start: the vanilla
// server Paper and Folia patch, the modules of Waterfall
func (p *FillProvider) Prepare(ctx context.Context, dir string, jar *ServerJar) error {
	switch p.project {
	case "paper", "folia":
		return preparePaperclip(ctx, dir, jar)
	case "waterfall":
		return prepareModules(ctx, dir, jar)
	}
	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *FillProvider) GetRecommendedJavaVersion(ctx context.Context, version string) int {
	return p.javaVersion(ctx, version)
}
