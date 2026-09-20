package jenkins

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	bungeecordJobURL = "https://hub.spigotmc.org/jenkins/job/BungeeCord"

	resultSuccess = "SUCCESS"

	// buildsTree asks Jenkins for the artifacts of every build in one request
	buildsTree = "builds[number,result,artifacts[fileName,relativePath]]"
)

// Client is a Jenkins API client
type Client struct {
	http    *utils.HTTPClient
	baseURL string
}

// NewClient creates a client for a Jenkins URL: a server root or a single job
func NewClient(baseURL string) *Client {
	return &Client{
		http:    utils.NewHTTPClient(false),
		baseURL: baseURL,
	}
}

// NewBungeecordClient creates a client for Bungeecord Jenkins
func NewBungeecordClient() *Client {
	return NewClient(bungeecordJobURL)
}

// Job returns a client for a job of this Jenkins server
func (c *Client) Job(name string) *Client {
	return &Client{
		http:    c.http,
		baseURL: fmt.Sprintf("%s/job/%s", c.baseURL, name),
	}
}

// JobRef is a reference to a job of a Jenkins server
type JobRef struct {
	Name string `json:"name"`
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

// IsSuccessful reports whether the build finished and produced its artifacts
func (b *BuildInfo) IsSuccessful() bool {
	return b.Result == resultSuccess
}

// Artifact represents a build artifact
type Artifact struct {
	FileName     string `json:"fileName"`
	RelativePath string `json:"relativePath"`
}

// GetJobs fetches the jobs of a Jenkins server
func (c *Client) GetJobs(ctx context.Context) ([]JobRef, error) {
	var response struct {
		Jobs []JobRef `json:"jobs"`
	}
	if err := c.get(ctx, c.baseURL+"/api/json?tree=jobs[name]", &response); err != nil {
		return nil, fmt.Errorf("failed to fetch jobs: %w", err)
	}
	return response.Jobs, nil
}

// GetJobInfo fetches job information
func (c *Client) GetJobInfo(ctx context.Context) (*JobInfo, error) {
	var info JobInfo
	if err := c.get(ctx, c.baseURL+"/api/json", &info); err != nil {
		return nil, fmt.Errorf("failed to fetch job info: %w", err)
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

// GetBuildsWithArtifacts fetches the builds of a job with their artifacts, newest first
func (c *Client) GetBuildsWithArtifacts(ctx context.Context) ([]BuildInfo, error) {
	var response struct {
		Builds []BuildInfo `json:"builds"`
	}
	if err := c.get(ctx, c.baseURL+"/api/json?tree="+buildsTree, &response); err != nil {
		return nil, fmt.Errorf("failed to fetch builds: %w", err)
	}
	return response.Builds, nil
}

// GetBuild fetches information about a specific build
func (c *Client) GetBuild(ctx context.Context, number int) (*BuildInfo, error) {
	var build BuildInfo
	if err := c.get(ctx, fmt.Sprintf("%s/%d/api/json", c.baseURL, number), &build); err != nil {
		return nil, fmt.Errorf("failed to fetch build info: %w", err)
	}
	return &build, nil
}

// GetLastSuccessfulBuild fetches the last successful build
func (c *Client) GetLastSuccessfulBuild(ctx context.Context) (*BuildInfo, error) {
	var build BuildInfo
	if err := c.get(ctx, c.baseURL+"/lastSuccessfulBuild/api/json", &build); err != nil {
		return nil, fmt.Errorf("failed to fetch last successful build: %w", err)
	}
	return &build, nil
}

// GetArtifactURL constructs the download URL for an artifact
func (c *Client) GetArtifactURL(buildNumber int, relativePath string) string {
	return fmt.Sprintf("%s/%d/artifact/%s", c.baseURL, buildNumber, relativePath)
}

func (c *Client) get(ctx context.Context, url string, target interface{}) error {
	data, err := c.http.Get(ctx, url)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
