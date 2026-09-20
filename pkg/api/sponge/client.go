package sponge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	baseURL = "https://dl-api.spongepowered.org/v2/groups/org.spongepowered/artifacts"

	// ArtifactVanilla is the standalone Sponge server
	ArtifactVanilla = "spongevanilla"

	// universalClassifier marks the runnable server JAR among the assets of a version
	universalClassifier = "universal"

	versionsPageSize = 25
)

// Client is the Sponge downloads API client
type Client struct {
	http    *utils.HTTPClient
	baseURL string
}

// NewClient creates a new Sponge API client
func NewClient() *Client {
	return NewClientWithBaseURL(baseURL)
}

// NewClientWithBaseURL creates a Sponge API client for a custom API root
func NewClientWithBaseURL(url string) *Client {
	return &Client{
		http:    utils.NewHTTPClient(false),
		baseURL: url,
	}
}

// Version is a published build of an artifact
type Version struct {
	Version     string
	Recommended bool
}

// Asset is a downloadable file of a version
type Asset struct {
	Classifier  string `json:"classifier"`
	Extension   string `json:"extension"`
	DownloadURL string `json:"downloadUrl"`
	SHA1        string `json:"sha1"`
	MD5         string `json:"md5"`
}

// GetMinecraftVersions fetches the Minecraft versions an artifact was built for, newest first
func (c *Client) GetMinecraftVersions(ctx context.Context, artifact string) ([]string, error) {
	data, err := c.http.Get(ctx, fmt.Sprintf("%s/%s", c.baseURL, artifact))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch artifact: %w", err)
	}

	var response struct {
		Tags struct {
			Minecraft []string `json:"minecraft"`
		} `json:"tags"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse artifact: %w", err)
	}

	return response.Tags.Minecraft, nil
}

// GetVersions fetches the builds for a Minecraft version, newest first
func (c *Client) GetVersions(ctx context.Context, artifact, mcVersion string, recommendedOnly bool) ([]Version, error) {
	query := url.Values{}
	query.Set("tags", "minecraft:"+mcVersion)
	query.Set("limit", fmt.Sprint(versionsPageSize))
	if recommendedOnly {
		query.Set("recommended", "true")
	}

	data, err := c.http.Get(ctx, fmt.Sprintf("%s/%s/versions?%s", c.baseURL, artifact, query.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch versions: %w", err)
	}

	var response struct {
		Artifacts json.RawMessage `json:"artifacts"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse versions: %w", err)
	}

	versions, err := parseOrderedVersions(response.Artifacts)
	if err != nil {
		return nil, fmt.Errorf("failed to parse versions: %w", err)
	}

	return versions, nil
}

// parseOrderedVersions reads `{"<version>": {"recommended": bool}, ...}` in
// document order. The API lists the newest build first, an order that decoding
// into a Go map would lose.
func parseOrderedVersions(raw json.RawMessage) ([]Version, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	dec := json.NewDecoder(bytes.NewReader(raw))

	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if tok != json.Delim('{') {
		return nil, fmt.Errorf("unexpected token %v", tok)
	}

	var versions []Version
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}

		name, ok := key.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected key %v", key)
		}

		var entry struct {
			Recommended bool `json:"recommended"`
		}
		if err := dec.Decode(&entry); err != nil {
			return nil, err
		}

		versions = append(versions, Version{Version: name, Recommended: entry.Recommended})
	}

	return versions, nil
}

// GetServerAsset fetches the runnable server JAR of a version
func (c *Client) GetServerAsset(ctx context.Context, artifact, version string) (*Asset, error) {
	data, err := c.http.Get(ctx, fmt.Sprintf("%s/%s/versions/%s", c.baseURL, artifact, version))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version: %w", err)
	}

	var response struct {
		Assets []Asset `json:"assets"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse version: %w", err)
	}

	for i := range response.Assets {
		asset := &response.Assets[i]
		if asset.Classifier == universalClassifier && asset.Extension == "jar" {
			return asset, nil
		}
	}

	return nil, fmt.Errorf("%s %s has no %s JAR", artifact, version, universalClassifier)
}
