package quilt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const baseURL = "https://meta.quiltmc.org/v3"

// Client is the Quilt meta API client
type Client struct {
	http    *utils.HTTPClient
	baseURL string
}

// NewClient creates a new Quilt API client
func NewClient() *Client {
	return NewClientWithBaseURL(baseURL)
}

// NewClientWithBaseURL creates a Quilt API client for a custom API root
func NewClientWithBaseURL(url string) *Client {
	return &Client{
		http:    utils.NewHTTPClient(false),
		baseURL: url,
	}
}

// GameVersion represents a Minecraft version supported by Quilt
type GameVersion struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

// LoaderVersion represents a Quilt loader version
type LoaderVersion struct {
	Version string `json:"version"`
}

// IsStable reports whether the loader version is a release rather than a beta
func (v LoaderVersion) IsStable() bool {
	return !strings.Contains(v.Version, "-")
}

// InstallerVersion represents a Quilt installer version. The hashes the meta
// API lists are not used: they can lag behind the file Maven actually serves.
type InstallerVersion struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

// GetGameVersions fetches the supported Minecraft versions, newest first
func (c *Client) GetGameVersions(ctx context.Context) ([]GameVersion, error) {
	var versions []GameVersion
	if err := c.get(ctx, "/versions/game", &versions); err != nil {
		return nil, fmt.Errorf("failed to fetch game versions: %w", err)
	}
	return versions, nil
}

// GetLoaderVersions fetches the loader versions, newest first
func (c *Client) GetLoaderVersions(ctx context.Context) ([]LoaderVersion, error) {
	var versions []LoaderVersion
	if err := c.get(ctx, "/versions/loader", &versions); err != nil {
		return nil, fmt.Errorf("failed to fetch loader versions: %w", err)
	}
	return versions, nil
}

// GetLatestInstaller fetches the newest installer
func (c *Client) GetLatestInstaller(ctx context.Context) (*InstallerVersion, error) {
	var versions []InstallerVersion
	if err := c.get(ctx, "/versions/installer", &versions); err != nil {
		return nil, fmt.Errorf("failed to fetch installer versions: %w", err)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no installer versions available")
	}

	return &versions[0], nil
}

// GetInstallerSHA256 fetches the checksum Maven publishes next to the installer
func (c *Client) GetInstallerSHA256(ctx context.Context, installer *InstallerVersion) (string, error) {
	data, err := c.http.Get(ctx, installer.URL+".sha256")
	if err != nil {
		return "", fmt.Errorf("failed to fetch installer checksum: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// GetLatestStableLoader returns the newest loader release, or the newest beta if there is none
func (c *Client) GetLatestStableLoader(ctx context.Context) (*LoaderVersion, error) {
	versions, err := c.GetLoaderVersions(ctx)
	if err != nil {
		return nil, err
	}

	for i := range versions {
		if versions[i].IsStable() {
			return &versions[i], nil
		}
	}

	if len(versions) > 0 {
		return &versions[0], nil
	}

	return nil, fmt.Errorf("no loader versions available")
}

func (c *Client) get(ctx context.Context, path string, target interface{}) error {
	data, err := c.http.Get(ctx, c.baseURL+path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
