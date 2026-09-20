package mohist

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	// baseURL is the current MohistMC API. The older mohistmc.com/api/v2 still
	// answers with metadata, but serves empty files for every download.
	baseURL = "https://api.mohistmc.com/project"

	// ProjectMohist is the Forge + Bukkit hybrid
	ProjectMohist = "mohist"

	// ProjectBanner is the Fabric + Bukkit hybrid
	ProjectBanner = "banner"
)

// Client is the MohistMC API client
type Client struct {
	http    *utils.HTTPClient
	baseURL string
}

// NewClient creates a new MohistMC API client
func NewClient() *Client {
	return NewClientWithBaseURL(baseURL)
}

// NewClientWithBaseURL creates a MohistMC API client for a custom API root
func NewClientWithBaseURL(url string) *Client {
	return &Client{
		http:    utils.NewHTTPClient(false),
		baseURL: url,
	}
}

// Build describes a single build
type Build struct {
	ID         int    `json:"id"`
	FileSHA256 string `json:"file_sha256"`
	BuildDate  string `json:"build_date"`
	Loader     struct {
		ForgeVersion string `json:"forge_version"`
	} `json:"loader"`
}

// GetVersions fetches the Minecraft versions of a project, in no particular order
func (c *Client) GetVersions(ctx context.Context, project string) ([]string, error) {
	data, err := c.http.Get(ctx, fmt.Sprintf("%s/%s/versions", c.baseURL, project))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch versions: %w", err)
	}

	var entries []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse versions: %w", err)
	}

	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		versions = append(versions, entry.Name)
	}

	return versions, nil
}

// GetBuilds fetches the builds of a Minecraft version, newest first
func (c *Client) GetBuilds(ctx context.Context, project, version string) ([]Build, error) {
	data, err := c.http.Get(ctx, fmt.Sprintf("%s/%s/%s/builds", c.baseURL, project, version))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch builds: %w", err)
	}

	var builds []Build
	if err := json.Unmarshal(data, &builds); err != nil {
		return nil, fmt.Errorf("failed to parse builds: %w", err)
	}

	return builds, nil
}

// GetDownloadURL returns the JAR download URL of a build
func (c *Client) GetDownloadURL(project, version string, build int) string {
	return fmt.Sprintf("%s/%s/%s/builds/%d/download", c.baseURL, project, version, build)
}
