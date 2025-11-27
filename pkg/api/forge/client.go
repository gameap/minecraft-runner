package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	promotionsURL = "https://files.minecraftforge.net/net/minecraftforge/forge/promotions_slim.json"
	mavenBaseURL  = "https://maven.minecraftforge.net/net/minecraftforge/forge"
)

// Client is the Forge API client
type Client struct {
	http *utils.HTTPClient
}

// NewClient creates a new Forge API client
func NewClient() *Client {
	return &Client{
		http: utils.NewHTTPClient(false),
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
	data, err := c.http.Get(ctx, promotionsURL)
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

	// Try recommended first
	if preferRecommended {
		if forgeVer, ok := promotions.Promos[mcVersion+"-recommended"]; ok {
			v.ForgeVersion = forgeVer
			v.IsRecommended = true
			return v, nil
		}
	}

	// Fall back to latest
	if forgeVer, ok := promotions.Promos[mcVersion+"-latest"]; ok {
		v.ForgeVersion = forgeVer
		v.IsLatest = true
		return v, nil
	}

	return nil, fmt.Errorf("no Forge version found for Minecraft %s", mcVersion)
}

// GetInstallerURL returns the installer JAR download URL
func (c *Client) GetInstallerURL(mcVersion, forgeVersion string) string {
	fullVersion := fmt.Sprintf("%s-%s", mcVersion, forgeVersion)
	return fmt.Sprintf("%s/%s/forge-%s-installer.jar", mavenBaseURL, fullVersion, fullVersion)
}

// GetServerJarName returns the expected server JAR name after installation
func (c *Client) GetServerJarName(mcVersion, forgeVersion string) string {
	// For modern Forge (1.17+), the naming changed
	major, minor, _ := parseVersion(mcVersion)
	if major >= 1 && minor >= 17 {
		// Modern Forge uses run.sh/run.bat or the libraries folder
		return fmt.Sprintf("forge-%s-%s-server.jar", mcVersion, forgeVersion)
	}
	// Legacy Forge
	return fmt.Sprintf("forge-%s-%s.jar", mcVersion, forgeVersion)
}

func parseVersion(version string) (major, minor, patch int) {
	fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch)
	return
}
