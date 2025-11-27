package fabric

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	baseURL = "https://meta.fabricmc.net/v2"
)

// Client is the Fabric API client
type Client struct {
	http *utils.HTTPClient
}

// NewClient creates a new Fabric API client
func NewClient() *Client {
	return &Client{
		http: utils.NewHTTPClient(false),
	}
}

// GameVersion represents a Minecraft version supported by Fabric
type GameVersion struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

// LoaderVersion represents a Fabric loader version
type LoaderVersion struct {
	Separator string `json:"separator"`
	Build     int    `json:"build"`
	Maven     string `json:"maven"`
	Version   string `json:"version"`
	Stable    bool   `json:"stable"`
}

// InstallerVersion represents a Fabric installer version
type InstallerVersion struct {
	URL     string `json:"url"`
	Maven   string `json:"maven"`
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

// GetGameVersions fetches available game versions
func (c *Client) GetGameVersions(ctx context.Context) ([]GameVersion, error) {
	url := fmt.Sprintf("%s/versions/game", baseURL)
	data, err := c.http.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch game versions: %w", err)
	}

	var versions []GameVersion
	if err := json.Unmarshal(data, &versions); err != nil {
		return nil, fmt.Errorf("failed to parse game versions: %w", err)
	}

	return versions, nil
}

// GetLoaderVersions fetches available loader versions
func (c *Client) GetLoaderVersions(ctx context.Context) ([]LoaderVersion, error) {
	url := fmt.Sprintf("%s/versions/loader", baseURL)
	data, err := c.http.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch loader versions: %w", err)
	}

	var versions []LoaderVersion
	if err := json.Unmarshal(data, &versions); err != nil {
		return nil, fmt.Errorf("failed to parse loader versions: %w", err)
	}

	return versions, nil
}

// GetInstallerVersions fetches available installer versions
func (c *Client) GetInstallerVersions(ctx context.Context) ([]InstallerVersion, error) {
	url := fmt.Sprintf("%s/versions/installer", baseURL)
	data, err := c.http.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch installer versions: %w", err)
	}

	var versions []InstallerVersion
	if err := json.Unmarshal(data, &versions); err != nil {
		return nil, fmt.Errorf("failed to parse installer versions: %w", err)
	}

	return versions, nil
}

// GetServerJarURL constructs the server JAR download URL
func (c *Client) GetServerJarURL(gameVersion, loaderVersion, installerVersion string) string {
	return fmt.Sprintf("%s/versions/loader/%s/%s/%s/server/jar",
		baseURL, gameVersion, loaderVersion, installerVersion)
}

// GetLatestStableLoader returns the latest stable loader version
func (c *Client) GetLatestStableLoader(ctx context.Context) (*LoaderVersion, error) {
	versions, err := c.GetLoaderVersions(ctx)
	if err != nil {
		return nil, err
	}

	for _, v := range versions {
		if v.Stable {
			return &v, nil
		}
	}

	// Fallback to first version if no stable found
	if len(versions) > 0 {
		return &versions[0], nil
	}

	return nil, fmt.Errorf("no loader versions available")
}

// GetLatestStableInstaller returns the latest stable installer version
func (c *Client) GetLatestStableInstaller(ctx context.Context) (*InstallerVersion, error) {
	versions, err := c.GetInstallerVersions(ctx)
	if err != nil {
		return nil, err
	}

	for _, v := range versions {
		if v.Stable {
			return &v, nil
		}
	}

	// Fallback to first version if no stable found
	if len(versions) > 0 {
		return &versions[0], nil
	}

	return nil, fmt.Errorf("no installer versions available")
}
