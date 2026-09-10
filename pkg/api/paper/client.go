package paper

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const baseURL = "https://fill.papermc.io/v3"

type Client struct {
	http *utils.HTTPClient
}

func NewClient() *Client {
	return &Client{
		http: utils.NewHTTPClient(false),
	}
}

type Project struct {
	ProjectID   string   `json:"project_id"`
	ProjectName string   `json:"project_name"`
	Versions    []string `json:"versions"`
}

func (p *Project) UnmarshalJSON(data []byte) error {
	var raw struct {
		ProjectID   string          `json:"project_id"`
		ProjectName string          `json:"project_name"`
		Versions    json.RawMessage `json:"versions"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.ProjectID = raw.ProjectID
	p.ProjectName = raw.ProjectName

	// Paper returns versions as a simple array.
	var versions []string
	if err := json.Unmarshal(raw.Versions, &versions); err == nil {
		p.Versions = versions
		return nil
	}

	// Velocity returns versions grouped into objects.
	var groups map[string][]string
	if err := json.Unmarshal(raw.Versions, &groups); err != nil {
		return fmt.Errorf("invalid versions format: %w", err)
	}

	for _, group := range groups {
		p.Versions = append(p.Versions, group...)
	}

	return nil
}

type VersionBuilds []BuildInfo

type BuildInfo struct {
	ProjectID string `json:"project_id"`
	Version   string `json:"version"`
	Build     int    `json:"build"`
	ID        int    `json:"id"`
	Time      string `json:"time"`
	Channel   string `json:"channel"`
	Downloads struct {
		Application struct {
			Name   string `json:"name"`
			SHA256 string `json:"sha256"`
			URL    string `json:"url"`
			Size   int64  `json:"size"`
		} `json:"server:default"`
	} `json:"downloads"`
}

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

func (c *Client) GetVersionBuilds(ctx context.Context, project, version string) (*VersionBuilds, error) {
	url := fmt.Sprintf("%s/projects/%s/versions/%s/builds", baseURL, project, version)

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

func (c *Client) GetBuildInfo(ctx context.Context, project, version string, build int) (*BuildInfo, error) {
	vb, err := c.GetVersionBuilds(ctx, project, version)
	if err != nil {
		return nil, err
	}

	for _, b := range *vb {
		if b.Build == build || b.ID == build {
			return &b, nil
		}
	}

	return nil, fmt.Errorf("build %d not found for version %s", build, version)
}

func (c *Client) GetDownloadURL(project, version string, build int, filename string) string {
	return fmt.Sprintf(
		"https://fill.papermc.io/v3/projects/%s/versions/%s/builds/%d/downloads/%s",
		project,
		version,
		build,
		filename,
	)
}

func (c *Client) GetLatestBuild(ctx context.Context, project, version string) (*BuildInfo, error) {
	vb, err := c.GetVersionBuilds(ctx, project, version)
	if err != nil {
		return nil, err
	}

	for _, build := range *vb {
		if build.Channel == "STABLE" {
			return &build, nil
		}
	}

	return nil, fmt.Errorf("no stable builds available for version %s", version)
}
