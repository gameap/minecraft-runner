package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	promotionsURL    = "https://files.minecraftforge.net/net/minecraftforge/forge/promotions_slim.json"
	mavenMetadataURL = "https://files.minecraftforge.net/net/minecraftforge/forge/maven-metadata.json"
	mavenBaseURL     = "https://maven.minecraftforge.net/net/minecraftforge/forge"
)

// Client is the Forge API client
type Client struct {
	http             *utils.HTTPClient
	promotionsURL    string
	mavenMetadataURL string
}

// NewClient creates a new Forge API client
func NewClient() *Client {
	return NewClientWithURLs(promotionsURL, mavenMetadataURL)
}

// NewClientWithURLs creates a Forge API client with custom metadata locations
func NewClientWithURLs(promotions, mavenMetadata string) *Client {
	return &Client{
		http:             utils.NewHTTPClient(false),
		promotionsURL:    promotions,
		mavenMetadataURL: mavenMetadata,
	}
}

// Promotions represents the Forge promotions data
type Promotions struct {
	Homepage string            `json:"homepage"`
	Promos   map[string]string `json:"promos"`
}

// ForgeVersion contains parsed version information
type ForgeVersion struct {
	MinecraftVersion string
	ForgeVersion     string
	IsRecommended    bool
	IsLatest         bool
}

// GetPromotions fetches the Forge promotions data
func (c *Client) GetPromotions(ctx context.Context) (*Promotions, error) {
	data, err := c.http.Get(ctx, c.promotionsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch promotions: %w", err)
	}

	var promotions Promotions
	if err := json.Unmarshal(data, &promotions); err != nil {
		return nil, fmt.Errorf("failed to parse promotions: %w", err)
	}

	return &promotions, nil
}

// GetVersions returns all available Forge versions
func (c *Client) GetVersions(ctx context.Context) ([]ForgeVersion, error) {
	promotions, err := c.GetPromotions(ctx)
	if err != nil {
		return nil, err
	}

	versionMap := make(map[string]*ForgeVersion)

	for key, forgeVer := range promotions.Promos {
		// Keys are like "1.20.4-recommended", "1.20.4-latest"
		parts := strings.Split(key, "-")
		if len(parts) != 2 {
			continue
		}

		mcVersion := parts[0]
		vType := parts[1]

		if _, exists := versionMap[mcVersion]; !exists {
			versionMap[mcVersion] = &ForgeVersion{
				MinecraftVersion: mcVersion,
			}
		}

		v := versionMap[mcVersion]
		switch vType {
		case "recommended":
			v.ForgeVersion = forgeVer
			v.IsRecommended = true
		case "latest":
			if v.ForgeVersion == "" {
				v.ForgeVersion = forgeVer
			}
			v.IsLatest = true
		}
	}

	versions := make([]ForgeVersion, 0, len(versionMap))
	for _, v := range versionMap {
		versions = append(versions, *v)
	}

	return versions, nil
}

// GetVersionForMC returns the Forge version for a specific Minecraft version
func (c *Client) GetVersionForMC(ctx context.Context, mcVersion string, preferRecommended bool) (*ForgeVersion, error) {
	promotions, err := c.GetPromotions(ctx)
	if err != nil {
		return nil, err
	}

	v := &ForgeVersion{
		MinecraftVersion: mcVersion,
	}

	if preferRecommended {
		if forgeVer, ok := promotions.Promos[mcVersion+"-recommended"]; ok {
			v.ForgeVersion = forgeVer
			v.IsRecommended = true
			return v, nil
		}
	}

	if forgeVer, ok := promotions.Promos[mcVersion+"-latest"]; ok {
		v.ForgeVersion = forgeVer
		v.IsLatest = true
		return v, nil
	}

	return nil, fmt.Errorf("no Forge version found for Minecraft %s", mcVersion)
}

// GetMavenMetadata returns the artifact versions published for each Minecraft
// version, oldest first
func (c *Client) GetMavenMetadata(ctx context.Context) (map[string][]string, error) {
	data, err := c.http.Get(ctx, c.mavenMetadataURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch maven metadata: %w", err)
	}

	var metadata map[string][]string
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse maven metadata: %w", err)
	}

	return metadata, nil
}

// ResolveArtifactVersion returns the Maven artifact version of a Forge release.
// It is "<mc>-<forge>" for modern releases, but old ones carry one more suffix
// ("1.7.10-10.13.4.1614-1.7.10", "1.10-12.18.0.2000-1.10.0"), so it has to be
// looked up rather than assembled.
func (c *Client) ResolveArtifactVersion(ctx context.Context, mcVersion, forgeVersion string) (string, error) {
	metadata, err := c.GetMavenMetadata(ctx)
	if err != nil {
		return "", err
	}

	if artifact, ok := FindArtifactVersion(metadata[mcVersion], mcVersion, forgeVersion); ok {
		return artifact, nil
	}

	return "", fmt.Errorf("no Forge %s published for Minecraft %s", forgeVersion, mcVersion)
}

// FindArtifactVersion picks the artifact version of a Forge release out of the
// versions published for its Minecraft version
func FindArtifactVersion(artifacts []string, mcVersion, forgeVersion string) (string, bool) {
	want := ArtifactPrefix(mcVersion, forgeVersion)

	for _, artifact := range artifacts {
		if artifact == want || strings.HasPrefix(artifact, want+"-") {
			return artifact, true
		}
	}

	return "", false
}

// ForgeVersionOf extracts the Forge version from an artifact version
func ForgeVersionOf(artifact, mcVersion string) string {
	version := strings.TrimPrefix(artifact, mcVersion+"-")
	if i := strings.Index(version, "-"); i >= 0 {
		version = version[:i]
	}
	return version
}

// ArtifactPrefix is the part every artifact version of a release starts with
func ArtifactPrefix(mcVersion, forgeVersion string) string {
	return mcVersion + "-" + forgeVersion
}

// GetInstallerURL returns the installer JAR download URL of an artifact version
func (c *Client) GetInstallerURL(artifactVersion string) string {
	return fmt.Sprintf("%s/%s/%s", mavenBaseURL, artifactVersion, InstallerName(artifactVersion))
}

// InstallerName returns the installer file name of an artifact version
func InstallerName(artifactVersion string) string {
	return fmt.Sprintf("forge-%s-installer.jar", artifactVersion)
}
