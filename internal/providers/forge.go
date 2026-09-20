package providers

import (
	"context"
	"fmt"

	"github.com/gameap/minecraft-runner/pkg/api/forge"
)

// ForgeProvider provides Forge server downloads
type ForgeProvider struct {
	client *forge.Client
}

// NewForgeProvider creates a new Forge provider
func NewForgeProvider() *ForgeProvider {
	return &ForgeProvider{
		client: forge.NewClient(),
	}
}

// Name returns the provider name
func (p *ForgeProvider) Name() string {
	return "forge"
}

// ListVersions returns available Minecraft versions with Forge support
func (p *ForgeProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	versions, err := p.client.GetVersions(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]VersionInfo, 0, len(versions))
	for _, v := range versions {
		result = append(result, VersionInfo{
			MinecraftVersion: v.MinecraftVersion,
			ModVersion:       v.ForgeVersion,
			IsStable:         v.IsRecommended,
			Type:             "forge",
		})
	}

	sortVersionInfosDesc(result)

	return result, nil
}

// ListModVersions returns Forge versions for a specific Minecraft version, newest first
func (p *ForgeProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	promotions, err := p.client.GetPromotions(ctx)
	if err != nil {
		return nil, err
	}

	metadata, err := p.client.GetMavenMetadata(ctx)
	if err != nil {
		return nil, err
	}

	recommended := promotions.Promos[mcVersion+"-recommended"]
	latest := promotions.Promos[mcVersion+"-latest"]

	artifacts := metadata[mcVersion]
	versions := make([]VersionInfo, 0, len(artifacts))

	for i := len(artifacts) - 1; i >= 0; i-- {
		forgeVersion := forge.ForgeVersionOf(artifacts[i], mcVersion)

		vType := "build"
		switch forgeVersion {
		case recommended:
			vType = "recommended"
		case latest:
			vType = "latest"
		}

		versions = append(versions, VersionInfo{
			MinecraftVersion: mcVersion,
			ModVersion:       forgeVersion,
			IsStable:         forgeVersion == recommended,
			Type:             vType,
		})
	}

	return versions, nil
}

// GetLatestVersion returns the latest Minecraft version with Forge support
func (p *ForgeProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	versions, err := p.client.GetVersions(ctx)
	if err != nil {
		return nil, err
	}

	// Prefer the highest MC version with a recommended build
	var latest *forge.ForgeVersion
	for _, recommendedOnly := range []bool{true, false} {
		for i := range versions {
			v := &versions[i]
			if recommendedOnly && !v.IsRecommended {
				continue
			}
			if latest == nil || compareMCVersions(v.MinecraftVersion, latest.MinecraftVersion) > 0 {
				latest = v
			}
		}
		if latest != nil {
			break
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no Forge versions available")
	}

	return &VersionInfo{
		MinecraftVersion: latest.MinecraftVersion,
		ModVersion:       latest.ForgeVersion,
		IsStable:         latest.IsRecommended,
		Type:             "forge",
	}, nil
}

// GetServerJar returns download info for a specific version
func (p *ForgeProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	forgeVersion := modVersion
	if forgeVersion == "" {
		v, err := p.client.GetVersionForMC(ctx, mcVersion, true)
		if err != nil {
			return nil, err
		}
		forgeVersion = v.ForgeVersion
	}

	artifact, err := p.client.ResolveArtifactVersion(ctx, mcVersion, forgeVersion)
	if err != nil {
		return nil, err
	}

	return &ServerJar{
		Version:         mcVersion,
		ModVersion:      forgeVersion,
		URL:             p.client.GetInstallerURL(artifact),
		Filename:        forge.InstallerName(artifact),
		RequiresInstall: true,
		ServerArgs:      []string{"nogui"},
	}, nil
}

// ResolveLaunch finds the launch target of an installed Forge version.
// Forge 1.17+ is modular and starts from a JVM argument file; since 1.20.3 that
// file merely points at the shim JAR, so it covers every modern release. Older
// installers leave a runnable JAR in the server directory instead.
func (p *ForgeProvider) ResolveLaunch(dir string, jar *ServerJar) *LaunchTarget {
	prefix := forge.ArtifactPrefix(jar.Version, jar.ModVersion)

	if target := resolveArgFileLaunch(dir, "net", "minecraftforge", "forge", prefix); target != nil {
		return target
	}

	return resolveLegacyForgeJar(dir, "forge-"+prefix)
}

// PostDownload runs the Forge installer
func (p *ForgeProvider) PostDownload(ctx context.Context, dir string, jar *ServerJar, javaPath string) error {
	fmt.Println("Running Forge installer...")

	if err := runInstaller(ctx, javaPath, dir, jar.Filename, "--installServer"); err != nil {
		return installerError("Forge", err)
	}

	removeInstaller(dir, jar.Filename)

	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *ForgeProvider) GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return GetRecommendedJavaVersion(ctx, mcVersion)
}
