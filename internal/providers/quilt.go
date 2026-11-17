package providers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gameap/minecraft-runner/internal/preseed"
	"github.com/gameap/minecraft-runner/internal/utils"
	"github.com/gameap/minecraft-runner/pkg/api/mojang"
	"github.com/gameap/minecraft-runner/pkg/api/quilt"
)

// quiltServerJar is the vanilla server the Quilt launcher starts. It belongs
// to whichever Minecraft version was installed last.
const quiltServerJar = "server.jar"

// QuiltProvider provides Quilt server downloads
type QuiltProvider struct {
	client *quilt.Client
}

// NewQuiltProvider creates a new Quilt provider
func NewQuiltProvider() *QuiltProvider {
	return &QuiltProvider{
		client: quilt.NewClient(),
	}
}

// Name returns the provider name
func (p *QuiltProvider) Name() string {
	return "quilt"
}

// ListVersions returns available Minecraft versions, newest first
func (p *QuiltProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	gameVersions, err := p.client.GetGameVersions(ctx)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(gameVersions))
	for _, v := range gameVersions {
		vType := "release"
		if !v.Stable {
			vType = "snapshot"
		}

		versions = append(versions, VersionInfo{
			MinecraftVersion: v.Version,
			IsStable:         v.Stable,
			Type:             vType,
		})
	}

	return versions, nil
}

// ListModVersions returns loader versions, newest first
func (p *QuiltProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	loaderVersions, err := p.client.GetLoaderVersions(ctx)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(loaderVersions))
	for _, v := range loaderVersions {
		versions = append(versions, VersionInfo{
			MinecraftVersion: mcVersion,
			ModVersion:       v.Version,
			IsStable:         v.IsStable(),
			Type:             "loader",
		})
	}

	return versions, nil
}

// GetLatestVersion returns the newest stable Minecraft version Quilt supports
func (p *QuiltProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	gameVersions, err := p.client.GetGameVersions(ctx)
	if err != nil {
		return nil, err
	}

	for _, v := range gameVersions {
		if v.Stable {
			return &VersionInfo{MinecraftVersion: v.Version, IsStable: true, Type: "release"}, nil
		}
	}

	return nil, fmt.Errorf("no stable versions available")
}

// GetServerJar returns what to install for a specific version. mcrun installs
// Quilt itself, so there is nothing to download up front; ModVersion is the
// loader.
func (p *QuiltProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	loaderVersion := modVersion
	if loaderVersion == "" {
		loader, err := p.client.GetLatestStableLoader(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get loader version: %w", err)
		}
		loaderVersion = loader.Version
	}

	jar := &ServerJar{
		Version:         mcVersion,
		ModVersion:      loaderVersion,
		RequiresInstall: true,
	}
	jar.Filename = quiltVersionedLauncher(jar)

	return jar, nil
}

// ResolveLaunch finds the launcher of an installed Quilt version
func (p *QuiltProvider) ResolveLaunch(dir string, jar *ServerJar) *LaunchTarget {
	launcher := quiltVersionedLauncher(jar)

	for _, file := range []string{launcher, quiltServerJar} {
		if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
			return nil
		}
	}

	return &LaunchTarget{Jar: launcher}
}

// PostDownload installs Quilt the way its installer would, with the
// launcher under a versioned name, which is what lets ResolveLaunch tell the
// installed version. The Quilt installer is not used: it downloads everything
// again on each run and knows nothing of the mirrors. Launchers of other
// versions are removed: they would start against the server.jar that is
// replaced.
func (p *QuiltProvider) PostDownload(ctx context.Context, dir string, jar *ServerJar, _ string) error {
	fmt.Println("Installing Quilt...")

	stale, _ := filepath.Glob(filepath.Join(dir, "quilt-server-launch*.jar"))
	for _, launcher := range stale {
		os.Remove(launcher)
	}

	// Another Minecraft version needs another vanilla server
	os.Remove(filepath.Join(dir, quiltServerJar))

	return preseed.Quilt(ctx, utils.NewHTTPClient(false), mojang.NewClient(), dir,
		jar.Version, jar.ModVersion, quiltVersionedLauncher(jar))
}

func quiltVersionedLauncher(jar *ServerJar) string {
	return fmt.Sprintf("quilt-server-launch-%s-%s.jar", jar.Version, jar.ModVersion)
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *QuiltProvider) GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return GetRecommendedJavaVersion(ctx, mcVersion)
}
