package paper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	baseURL = "https://fill.papermc.io/v3"

	// ChannelStable marks builds PaperMC considers production ready
	ChannelStable = "STABLE"

	serverDownloadKey = "server:default"

	// latestVersionProbes limits how many versions are inspected when looking
	// for the newest one that already has a stable build
	latestVersionProbes = 6
)

// Client is the PaperMC Fill API client
type Client struct {
	http    *utils.HTTPClient
	baseURL string
}

// NewClient creates a new PaperMC API client
func NewClient() *Client {
	return NewClientWithBaseURL(baseURL)
}

// NewClientWithBaseURL creates a PaperMC API client for a custom API root
func NewClientWithBaseURL(url string) *Client {
	return &Client{
		http:    utils.NewHTTPClient(false),
		baseURL: url,
	}
}

// Project describes a PaperMC project and the versions it was built for
type Project struct {
	ID       string
	Name     string
	Versions []string // Newest first, in the order the API returns them
}

func (p *Project) UnmarshalJSON(data []byte) error {
	var raw struct {
		Project struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
		Versions json.RawMessage `json:"versions"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	versions, err := flattenVersionGroups(raw.Versions)
	if err != nil {
		return fmt.Errorf("invalid versions format: %w", err)
	}

	p.ID = raw.Project.ID
	p.Name = raw.Project.Name
	p.Versions = versions

	return nil
}

// flattenVersionGroups reads `{"1.21": ["1.21.11", ...], "1.20": [...]}` in
// document order. The API lists the newest group and version first, an order
// that decoding into a Go map would lose.
func flattenVersionGroups(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	dec := json.NewDecoder(bytes.NewReader(raw))

	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}

	if tok == json.Delim('[') {
		var versions []string
		if err := json.Unmarshal(raw, &versions); err != nil {
			return nil, err
		}
		return versions, nil
	}

	if tok != json.Delim('{') {
		return nil, fmt.Errorf("unexpected token %v", tok)
	}

	var versions []string
	for dec.More() {
		if _, err := dec.Token(); err != nil {
			return nil, err
		}

		var group []string
		if err := dec.Decode(&group); err != nil {
			return nil, err
		}
		versions = append(versions, group...)
	}

	return versions, nil
}

// Download is a downloadable file of a build
type Download struct {
	Name      string `json:"name"`
	Checksums struct {
		SHA256 string `json:"sha256"`
	} `json:"checksums"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

// BuildInfo describes a single build of a project version
type BuildInfo struct {
	ID        int                 `json:"id"`
	Time      string              `json:"time"`
	Channel   string              `json:"channel"`
	Downloads map[string]Download `json:"downloads"`
}

// IsStable reports whether the build was published to the stable channel
func (b *BuildInfo) IsStable() bool {
	return b.Channel == ChannelStable
}

// ServerDownload returns the server JAR of the build
func (b *BuildInfo) ServerDownload() (Download, bool) {
	d, ok := b.Downloads[serverDownloadKey]
	return d, ok && d.URL != ""
}

// GetProject fetches a project with its versions
func (c *Client) GetProject(ctx context.Context, project string) (*Project, error) {
	url := fmt.Sprintf("%s/projects/%s", c.baseURL, project)

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

// GetVersionBuilds fetches the builds of a version, newest first
func (c *Client) GetVersionBuilds(ctx context.Context, project, version string) ([]BuildInfo, error) {
	url := fmt.Sprintf("%s/projects/%s/versions/%s/builds", c.baseURL, project, version)

	data, err := c.http.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version builds: %w", err)
	}

	var builds []BuildInfo
	if err := json.Unmarshal(data, &builds); err != nil {
		return nil, fmt.Errorf("failed to parse version builds: %w", err)
	}

	return builds, nil
}

// GetBuildInfo returns a specific build of a version
func (c *Client) GetBuildInfo(ctx context.Context, project, version string, build int) (*BuildInfo, error) {
	builds, err := c.GetVersionBuilds(ctx, project, version)
	if err != nil {
		return nil, err
	}

	for i := range builds {
		if builds[i].ID == build {
			return &builds[i], nil
		}
	}

	return nil, fmt.Errorf("build %d not found for version %s", build, version)
}

// GetLatestBuild returns the newest stable build of a version. A version that
// has no stable build yet yields its newest build of any channel, which the
// caller can tell apart with IsStable.
func (c *Client) GetLatestBuild(ctx context.Context, project, version string) (*BuildInfo, error) {
	builds, err := c.GetVersionBuilds(ctx, project, version)
	if err != nil {
		return nil, err
	}

	if build := newestBuild(builds, true); build != nil {
		return build, nil
	}

	if build := newestBuild(builds, false); build != nil {
		return build, nil
	}

	return nil, fmt.Errorf("no builds available for version %s", version)
}

// GetLatestStableVersion returns the newest version that has a stable build.
// A freshly released Minecraft version only has ALPHA or BETA builds for a
// while, so the newest listed version is not necessarily a usable default.
func (c *Client) GetLatestStableVersion(ctx context.Context, project string) (string, error) {
	p, err := c.GetProject(ctx, project)
	if err != nil {
		return "", err
	}

	if len(p.Versions) == 0 {
		return "", fmt.Errorf("no versions available for %s", project)
	}

	probes := p.Versions
	if len(probes) > latestVersionProbes {
		probes = probes[:latestVersionProbes]
	}

	var lastErr error
	for _, version := range probes {
		builds, err := c.GetVersionBuilds(ctx, project, version)
		if err != nil {
			lastErr = err
			continue
		}

		if newestBuild(builds, true) != nil {
			return version, nil
		}
	}

	if lastErr != nil {
		return "", lastErr
	}

	return p.Versions[0], nil
}

func newestBuild(builds []BuildInfo, stableOnly bool) *BuildInfo {
	for i := range builds {
		if stableOnly && !builds[i].IsStable() {
			continue
		}
		if _, ok := builds[i].ServerDownload(); ok {
			return &builds[i]
		}
	}
	return nil
}
