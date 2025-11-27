package paper

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	baseURL = "https://api.papermc.io/v2"
)

// Client is the Paper API client
type Client struct {
	http *utils.HTTPClient
}

// NewClient creates a new Paper API client
func NewClient() *Client {
	return &Client{
		http: utils.NewHTTPClient(false),
	}
}

// Project represents a Paper project
type Project struct {
	ProjectID   string   `json:"project_id"`
	ProjectName string   `json:"project_name"`
	Versions    []string `json:"versions"`
}

// VersionBuilds contains builds for a specific version
type VersionBuilds struct {
	ProjectID string `json:"project_id"`
	Version   string `json:"version"`
	Builds    []int  `json:"builds"`
}

// BuildInfo contains detailed information about a build
type BuildInfo struct {
	ProjectID string `json:"project_id"`
	Version   string `json:"version"`
	Build     int    `json:"build"`
	Time      string `json:"time"`
	Channel   string `json:"channel"`
	Downloads struct {
		Application struct {
			Name   string `json:"name"`
			SHA256 string `json:"sha256"`
		} `json:"application"`
	} `json:"downloads"`
}

// GetProject fetches project information
func (c *Client) GetProject(ctx context.Context, project string) (*Project, error) {
	url := fmt.Sprintf("%s/projects/%s", baseURL, project)
	data, err := c.http.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch project: %w", err)
	}

	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse project: %w", err)
	}

	return &p, nil
}

// GetVersionBuilds fetches available builds for a version
func (c *Client) GetVersionBuilds(ctx context.Context, project, version string) (*VersionBuilds, error) {
	url := fmt.Sprintf("%s/projects/%s/versions/%s", baseURL, project, version)
	data, err := c.http.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version builds: %w", err)
	}

	var vb VersionBuilds
	if err := json.Unmarshal(data, &vb); err != nil {
		return nil, fmt.Errorf("failed to parse version builds: %w", err)
	}

	return &vb, nil
}

// GetBuildInfo fetches information about a specific build
func (c *Client) GetBuildInfo(ctx context.Context, project, version string, build int) (*BuildInfo, error) {
	url := fmt.Sprintf("%s/projects/%s/versions/%s/builds/%d", baseURL, project, version, build)
	data, err := c.http.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch build info: %w", err)
	}

	var bi BuildInfo
	if err := json.Unmarshal(data, &bi); err != nil {
		return nil, fmt.Errorf("failed to parse build info: %w", err)
	}

	return &bi, nil
}

// GetDownloadURL constructs the download URL for a build
func (c *Client) GetDownloadURL(project, version string, build int, filename string) string {
	return fmt.Sprintf("%s/projects/%s/versions/%s/builds/%d/downloads/%s",
		baseURL, project, version, build, filename)
}

// GetLatestBuild gets the latest build for a version
func (c *Client) GetLatestBuild(ctx context.Context, project, version string) (*BuildInfo, error) {
	vb, err := c.GetVersionBuilds(ctx, project, version)
	if err != nil {
		return nil, err
	}

	if len(vb.Builds) == 0 {
		return nil, fmt.Errorf("no builds available for version %s", version)
	}

	// Get the latest build (last in the list)
	latestBuild := vb.Builds[len(vb.Builds)-1]

	return c.GetBuildInfo(ctx, project, version, latestBuild)
}
