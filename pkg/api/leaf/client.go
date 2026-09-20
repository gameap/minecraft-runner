package leaf

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	baseURL = "https://api.leafmc.one/v2/projects/leaf"

	// ChannelDefault marks the builds Leaf considers production ready
	ChannelDefault = "default"
)

// Client is the Leaf API client
type Client struct {
	http    *utils.HTTPClient
	baseURL string
}

// NewClient creates a new Leaf API client
func NewClient() *Client {
	return NewClientWithBaseURL(baseURL)
}

// NewClientWithBaseURL creates a Leaf API client for a custom API root
func NewClientWithBaseURL(url string) *Client {
	return &Client{
		http:    utils.NewHTTPClient(false),
		baseURL: url,
	}
}

// Project lists the Minecraft versions Leaf was built for, in no particular order
type Project struct {
	Versions []string `json:"versions"`
}

// Build describes a single build
type Build struct {
	Build     int    `json:"build"`
	Channel   string `json:"channel"`
	Downloads struct {
		Primary struct {
			Name   string `json:"name"`
			SHA256 string `json:"sha256"`
		} `json:"primary"`
	} `json:"downloads"`
}

// IsStable reports whether the build was published to the default channel
func (b *Build) IsStable() bool {
	return b.Channel == ChannelDefault
}

// GetProject fetches the supported Minecraft versions
func (c *Client) GetProject(ctx context.Context) (*Project, error) {
	data, err := c.http.Get(ctx, c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch project: %w", err)
	}

	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("failed to parse project: %w", err)
	}

	return &project, nil
}

// GetBuilds fetches the builds of a Minecraft version, oldest first
func (c *Client) GetBuilds(ctx context.Context, version string) ([]Build, error) {
	data, err := c.http.Get(ctx, fmt.Sprintf("%s/versions/%s/builds", c.baseURL, version))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch builds: %w", err)
	}

	var response struct {
		Builds []Build `json:"builds"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse builds: %w", err)
	}

	return response.Builds, nil
}

// GetDownloadURL returns the JAR download URL of a build
func (c *Client) GetDownloadURL(version string, build int, filename string) string {
	return fmt.Sprintf("%s/versions/%s/builds/%d/downloads/%s", c.baseURL, version, build, filename)
}
