package jenkins

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	bungeecordBaseURL = "https://hub.spigotmc.org/jenkins/job/BungeeCord"
)

// Client is a Jenkins API client
type Client struct {
	http    *utils.HTTPClient
	baseURL string
}

// NewBungeecordClient creates a client for Bungeecord Jenkins
func NewBungeecordClient() *Client {
	return &Client{
		http:    utils.NewHTTPClient(false),
		baseURL: bungeecordBaseURL,
	}
}

// JobInfo represents Jenkins job information
type JobInfo struct {
	DisplayName         string     `json:"displayName"`
	LastSuccessfulBuild BuildRef   `json:"lastSuccessfulBuild"`
	Builds              []BuildRef `json:"builds"`
}

// BuildRef is a reference to a build
type BuildRef struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

// BuildInfo represents detailed build information
type BuildInfo struct {
	Number    int        `json:"number"`
	Result    string     `json:"result"`
	Artifacts []Artifact `json:"artifacts"`
}

// Artifact represents a build artifact
type Artifact struct {
	FileName     string `json:"fileName"`
	RelativePath string `json:"relativePath"`
}

// GetJobInfo fetches job information
func (c *Client) GetJobInfo(ctx context.Context) (*JobInfo, error) {
	url := fmt.Sprintf("%s/api/json", c.baseURL)
	data, err := c.http.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job info: %w", err)
	}

	var info JobInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse job info: %w", err)
	}

	return &info, nil
}

// GetBuilds returns available build numbers
func (c *Client) GetBuilds(ctx context.Context) ([]BuildRef, error) {
	info, err := c.GetJobInfo(ctx)
	if err != nil {
		return nil, err
	}
	return info.Builds, nil
}

// GetBuild fetches information about a specific build
func (c *Client) GetBuild(ctx context.Context, number int) (*BuildInfo, error) {
	url := fmt.Sprintf("%s/%d/api/json", c.baseURL, number)
	data, err := c.http.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch build info: %w", err)
	}

	var build BuildInfo
	if err := json.Unmarshal(data, &build); err != nil {
		return nil, fmt.Errorf("failed to parse build info: %w", err)
	}

	return &build, nil
}

// GetLastSuccessfulBuild fetches the last successful build
func (c *Client) GetLastSuccessfulBuild(ctx context.Context) (*BuildInfo, error) {
	url := fmt.Sprintf("%s/lastSuccessfulBuild/api/json", c.baseURL)
	data, err := c.http.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch last successful build: %w", err)
	}

	var build BuildInfo
	if err := json.Unmarshal(data, &build); err != nil {
		return nil, fmt.Errorf("failed to parse build info: %w", err)
	}

	return &build, nil
}

// GetArtifactURL constructs the download URL for an artifact
func (c *Client) GetArtifactURL(buildNumber int, relativePath string) string {
	return fmt.Sprintf("%s/%d/artifact/%s", c.baseURL, buildNumber, relativePath)
}
