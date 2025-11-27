package mojang

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	versionManifestURL = "https://launchermeta.mojang.com/mc/game/version_manifest.json"
)

// Client is the Mojang API client
type Client struct {
	http *utils.HTTPClient
}

// NewClient creates a new Mojang API client
func NewClient() *Client {
	return &Client{
		http: utils.NewHTTPClient(false),
	}
}

// VersionManifest represents the Mojang version manifest
type VersionManifest struct {
	Latest   LatestVersions `json:"latest"`
	Versions []Version      `json:"versions"`
}

// LatestVersions contains the latest release and snapshot versions
type LatestVersions struct {
	Release  string `json:"release"`
	Snapshot string `json:"snapshot"`
}

// Version represents a Minecraft version entry
type Version struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	URL         string    `json:"url"`
	Time        time.Time `json:"time"`
	ReleaseTime time.Time `json:"releaseTime"`
}

// VersionDetail contains detailed information about a specific version
type VersionDetail struct {
	ID        string `json:"id"`
	Downloads struct {
		Server struct {
			SHA1 string `json:"sha1"`
			Size int64  `json:"size"`
			URL  string `json:"url"`
		} `json:"server"`
		Client struct {
			SHA1 string `json:"sha1"`
			Size int64  `json:"size"`
			URL  string `json:"url"`
		} `json:"client"`
	} `json:"downloads"`
	JavaVersion struct {
		Component    string `json:"component"`
		MajorVersion int    `json:"majorVersion"`
	} `json:"javaVersion"`
}

// GetVersionManifest fetches the version manifest from Mojang
func (c *Client) GetVersionManifest(ctx context.Context) (*VersionManifest, error) {
	data, err := c.http.Get(ctx, versionManifestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version manifest: %w", err)
	}

	var manifest VersionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse version manifest: %w", err)
	}

	return &manifest, nil
}

// GetVersionDetail fetches detailed information about a specific version
func (c *Client) GetVersionDetail(ctx context.Context, versionURL string) (*VersionDetail, error) {
	data, err := c.http.Get(ctx, versionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version detail: %w", err)
	}

	var detail VersionDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		return nil, fmt.Errorf("failed to parse version detail: %w", err)
	}

	return &detail, nil
}

// FindVersion finds a version by ID in the manifest
func (m *VersionManifest) FindVersion(id string) *Version {
	for _, v := range m.Versions {
		if v.ID == id {
			return &v
		}
	}
	return nil
}

// GetReleases returns only release versions from the manifest
func (m *VersionManifest) GetReleases() []Version {
	var releases []Version
	for _, v := range m.Versions {
		if v.Type == "release" {
			releases = append(releases, v)
		}
	}
	return releases
}
