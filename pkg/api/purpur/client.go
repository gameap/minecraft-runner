package purpur

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	baseURL = "https://api.purpurmc.org/v2/purpur"

	resultSuccess = "SUCCESS"
)

// Client is the Purpur API client
type Client struct {
	http    *utils.HTTPClient
	baseURL string
}

// NewClient creates a new Purpur API client
func NewClient() *Client {
	return NewClientWithBaseURL(baseURL)
}

// NewClientWithBaseURL creates a Purpur API client for a custom API root
func NewClientWithBaseURL(url string) *Client {
	return &Client{
		http:    utils.NewHTTPClient(false),
		baseURL: url,
	}
}

// Project lists the Minecraft versions Purpur was built for, oldest first
type Project struct {
	Versions []string `json:"versions"`
	Metadata struct {
		Current string `json:"current"`
	} `json:"metadata"`
}

// Version lists the builds of a Minecraft version, oldest first
type Version struct {
	Version string `json:"version"`
	Builds  struct {
		Latest string   `json:"latest"`
		All    []string `json:"all"`
	} `json:"builds"`
}

// Build describes a single build
type Build struct {
	Build  string `json:"build"`
	Result string `json:"result"`
	MD5    string `json:"md5"`
}

// IsSuccessful reports whether the build produced a JAR
func (b *Build) IsSuccessful() bool {
	return b.Result == resultSuccess
}

// GetProject fetches the supported Minecraft versions
func (c *Client) GetProject(ctx context.Context) (*Project, error) {
	var project Project
	if err := c.get(ctx, c.baseURL, &project); err != nil {
		return nil, fmt.Errorf("failed to fetch project: %w", err)
	}
	return &project, nil
}

// GetVersion fetches the builds of a Minecraft version
func (c *Client) GetVersion(ctx context.Context, version string) (*Version, error) {
	var v Version
	if err := c.get(ctx, fmt.Sprintf("%s/%s", c.baseURL, version), &v); err != nil {
		return nil, fmt.Errorf("failed to fetch version: %w", err)
	}
	return &v, nil
}

// GetBuild fetches a build; "latest" is accepted as a build number
func (c *Client) GetBuild(ctx context.Context, version, build string) (*Build, error) {
	var b Build
	if err := c.get(ctx, fmt.Sprintf("%s/%s/%s", c.baseURL, version, build), &b); err != nil {
		return nil, fmt.Errorf("failed to fetch build: %w", err)
	}
	return &b, nil
}

// GetDownloadURL returns the JAR download URL of a build
func (c *Client) GetDownloadURL(version, build string) string {
	return fmt.Sprintf("%s/%s/%s/download", c.baseURL, version, build)
}

func (c *Client) get(ctx context.Context, url string, target interface{}) error {
	data, err := c.http.Get(ctx, url)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
