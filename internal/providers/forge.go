package providers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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

	return result, nil
}

// ListModVersions returns Forge versions for a specific Minecraft version
func (p *ForgeProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	// Forge typically has one recommended and one latest version per MC version
	promotions, err := p.client.GetPromotions(ctx)
	if err != nil {
		return nil, err
	}

	var versions []VersionInfo

	if recommended, ok := promotions.Promos[mcVersion+"-recommended"]; ok {
		versions = append(versions, VersionInfo{
			MinecraftVersion: mcVersion,
			ModVersion:       recommended,
			IsStable:         true,
			Type:             "recommended",
		})
	}

	if latest, ok := promotions.Promos[mcVersion+"-latest"]; ok {
		// Only add if different from recommended
		if len(versions) == 0 || versions[0].ModVersion != latest {
			versions = append(versions, VersionInfo{
				MinecraftVersion: mcVersion,
				ModVersion:       latest,
				IsStable:         false,
				Type:             "latest",
			})
		}
	}

	return versions, nil
}

// GetLatestVersion returns the latest Minecraft version with Forge support
func (p *ForgeProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	versions, err := p.client.GetVersions(ctx)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no Forge versions available")
	}

	// Find the highest MC version with a recommended build
	var latest *forge.ForgeVersion
	for i := range versions {
		v := &versions[i]
		if v.IsRecommended {
			if latest == nil || compareVersions(v.MinecraftVersion, latest.MinecraftVersion) > 0 {
				latest = v
			}
		}
	}

	// Fallback to any version
	if latest == nil {
		for i := range versions {
			v := &versions[i]
			if latest == nil || compareVersions(v.MinecraftVersion, latest.MinecraftVersion) > 0 {
				latest = v
			}
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

	installerURL := p.client.GetInstallerURL(mcVersion, forgeVersion)
	installerName := fmt.Sprintf("forge-%s-%s-installer.jar", mcVersion, forgeVersion)

	return &ServerJar{
		Version:         mcVersion,
		ModVersion:      forgeVersion,
		URL:             installerURL,
		Filename:        installerName,
		RequiresInstall: true,
	}, nil
}

// PostDownload runs the Forge installer
func (p *ForgeProvider) PostDownload(ctx context.Context, jarPath string, javaPath string) error {
	if javaPath == "" {
		javaPath = "java"
	}

	dir := filepath.Dir(jarPath)

	// Run the Forge installer
	cmd := exec.CommandContext(ctx, javaPath, "-jar", jarPath, "--installServer")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("Running Forge installer...")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("forge installer failed: %w", err)
	}

	// Clean up installer
	os.Remove(jarPath)
	// Also remove installer log
	os.Remove(filepath.Join(dir, "installer.log"))

	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *ForgeProvider) GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return GetRecommendedJavaVersion(ctx, mcVersion)
}

// compareVersions compares two version strings
func compareVersions(v1, v2 string) int {
	var major1, minor1, patch1, major2, minor2, patch2 int
	fmt.Sscanf(v1, "%d.%d.%d", &major1, &minor1, &patch1)
	fmt.Sscanf(v2, "%d.%d.%d", &major2, &minor2, &patch2)

	if major1 != major2 {
		return major1 - major2
	}
	if minor1 != minor2 {
		return minor1 - minor2
	}
	return patch1 - patch2
}
