package providers

import (
	"context"
	"fmt"

	"github.com/gameap/minecraft-runner/pkg/api/neoforge"
)

// NeoForgeProvider provides NeoForge server downloads
type NeoForgeProvider struct {
	client *neoforge.Client
}

// NewNeoForgeProvider creates a new NeoForge provider
func NewNeoForgeProvider() *NeoForgeProvider {
	return &NeoForgeProvider{
		client: neoforge.NewClient(),
	}
}

// Name returns the provider name
func (p *NeoForgeProvider) Name() string {
	return "neoforge"
}

// ListVersions returns the Minecraft versions NeoForge supports, newest first,
// each with its newest stable NeoForge version
func (p *NeoForgeProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	releases, err := p.client.GetReleases(ctx)
	if err != nil {
		return nil, err
	}

	newest := make(map[string]neoforge.Release)
	for _, release := range releases {
		current, seen := newest[release.MinecraftVersion]
		if !seen || release.Stable || !current.Stable {
			newest[release.MinecraftVersion] = release
		}
	}

	versions := make([]VersionInfo, 0, len(newest))
	for _, release := range newest {
		versions = append(versions, releaseVersionInfo(release))
	}

	sortVersionInfosDesc(versions)

	return versions, nil
}

// ListModVersions returns NeoForge versions for a specific Minecraft version, newest first
func (p *NeoForgeProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	releases, err := p.client.GetReleases(ctx)
	if err != nil {
		return nil, err
	}

	var versions []VersionInfo
	for i := len(releases) - 1; i >= 0; i-- {
		if releases[i].MinecraftVersion == mcVersion {
			versions = append(versions, releaseVersionInfo(releases[i]))
		}
	}

	return versions, nil
}

// GetLatestVersion returns the newest Minecraft version with a stable NeoForge release
func (p *NeoForgeProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	versions, err := p.ListVersions(ctx)
	if err != nil {
		return nil, err
	}

	for i := range versions {
		if versions[i].IsStable {
			return &versions[i], nil
		}
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no NeoForge versions available")
	}

	return &versions[0], nil
}

// GetServerJar returns download info for a specific version
func (p *NeoForgeProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	releases, err := p.client.GetReleases(ctx)
	if err != nil {
		return nil, err
	}

	release, err := pickNeoForgeRelease(releases, mcVersion, modVersion)
	if err != nil {
		return nil, err
	}

	if !release.Stable {
		fmt.Printf("[WARNING] NeoForge has no stable release for Minecraft %s yet, using %s\n",
			mcVersion, release.Version)
	}

	checksum, err := p.client.GetInstallerSHA256(ctx, *release)
	if err != nil {
		return nil, err
	}

	return &ServerJar{
		Version:         mcVersion,
		ModVersion:      release.Version,
		URL:             release.InstallerURL(),
		SHA256:          checksum,
		Filename:        release.InstallerName(),
		RequiresInstall: true,
		ServerArgs:      []string{"nogui"},
	}, nil
}

// pickNeoForgeRelease returns the requested release, or the newest stable one
// for the Minecraft version, falling back to the newest pre-release
func pickNeoForgeRelease(releases []neoforge.Release, mcVersion, modVersion string) (*neoforge.Release, error) {
	var newest *neoforge.Release

	for i := len(releases) - 1; i >= 0; i-- {
		release := &releases[i]
		if release.MinecraftVersion != mcVersion {
			continue
		}

		if modVersion != "" {
			if release.Version == modVersion {
				return release, nil
			}
			continue
		}

		if release.Stable {
			return release, nil
		}
		if newest == nil {
			newest = release
		}
	}

	if newest != nil {
		return newest, nil
	}

	if modVersion != "" {
		return nil, fmt.Errorf("no NeoForge %s published for Minecraft %s", modVersion, mcVersion)
	}

	return nil, fmt.Errorf("no NeoForge version found for Minecraft %s", mcVersion)
}

// ResolveLaunch finds the JVM argument file of an installed NeoForge version
func (p *NeoForgeProvider) ResolveLaunch(dir string, jar *ServerJar) *LaunchTarget {
	if target := resolveArgFileLaunch(dir, "net", "neoforged", neoforge.ArtifactNeoForge, jar.ModVersion); target != nil {
		return target
	}

	legacyVersion := jar.Version + "-" + jar.ModVersion
	return resolveArgFileLaunch(dir, "net", "neoforged", neoforge.ArtifactLegacyForge, legacyVersion)
}

// PostDownload runs the NeoForge installer
func (p *NeoForgeProvider) PostDownload(ctx context.Context, dir string, jar *ServerJar, javaPath string) error {
	fmt.Println("Running NeoForge installer...")

	if err := runInstaller(ctx, javaPath, dir, jar.Filename, "--installServer"); err != nil {
		return installerError("NeoForge", err)
	}

	removeInstaller(dir, jar.Filename)

	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *NeoForgeProvider) GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return GetRecommendedJavaVersion(ctx, mcVersion)
}

func releaseVersionInfo(release neoforge.Release) VersionInfo {
	return VersionInfo{
		MinecraftVersion: release.MinecraftVersion,
		ModVersion:       release.Version,
		IsStable:         release.Stable,
		Type:             "neoforge",
	}
}
