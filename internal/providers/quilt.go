package providers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gameap/minecraft-runner/pkg/api/quilt"
)

const (
	// quiltLauncher is the launcher JAR the Quilt installer writes. It carries
	// no version, and the vanilla server.jar next to it belongs to whichever
	// Minecraft version was installed last.
	quiltLauncher = "quilt-server-launch.jar"

	quiltServerJar = "server.jar"
)

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

// GetServerJar returns download info for a specific version. The download is
// the Quilt installer; ModVersion is the loader it will install.
func (p *QuiltProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	loaderVersion := modVersion
	if loaderVersion == "" {
		loader, err := p.client.GetLatestStableLoader(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get loader version: %w", err)
		}
		loaderVersion = loader.Version
	}

	installer, err := p.client.GetLatestInstaller(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get installer version: %w", err)
	}

	checksum, err := p.client.GetInstallerSHA256(ctx, installer)
	if err != nil {
		return nil, err
	}

	return &ServerJar{
		Version:         mcVersion,
		ModVersion:      loaderVersion,
		URL:             installer.URL,
		SHA256:          checksum,
		Filename:        fmt.Sprintf("quilt-installer-%s.jar", installer.Version),
		RequiresInstall: true,
	}, nil
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

// PostDownload runs the Quilt installer and gives the launcher a versioned
// name, which is what lets ResolveLaunch tell the installed version. Launchers
// of other versions are removed: they would start against the server.jar that
// was just replaced.
func (p *QuiltProvider) PostDownload(ctx context.Context, dir string, jar *ServerJar, javaPath string) error {
	fmt.Println("Running Quilt installer...")

	err := runInstaller(ctx, javaPath, dir, jar.Filename,
		"install", "server", jar.Version, jar.ModVersion, "--install-dir=.", "--download-server")
	if err != nil {
		return installerError("Quilt", err)
	}

	stale, _ := filepath.Glob(filepath.Join(dir, "quilt-server-launch-*.jar"))
	for _, launcher := range stale {
		os.Remove(launcher)
	}

	if err := os.Rename(filepath.Join(dir, quiltLauncher), filepath.Join(dir, quiltVersionedLauncher(jar))); err != nil {
		return fmt.Errorf("failed to rename the Quilt launcher: %w", err)
	}

	removeInstaller(dir, jar.Filename)

	return nil
}

func quiltVersionedLauncher(jar *ServerJar) string {
	return fmt.Sprintf("quilt-server-launch-%s-%s.jar", jar.Version, jar.ModVersion)
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *QuiltProvider) GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return GetRecommendedJavaVersion(ctx, mcVersion)
}
