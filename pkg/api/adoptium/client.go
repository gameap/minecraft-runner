package adoptium

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	baseURL = "https://api.adoptium.net/v3"
)

// Client is the Adoptium API client
type Client struct {
	http *utils.HTTPClient
}

// NewClient creates a new Adoptium API client
func NewClient() *Client {
	return &Client{
		http: utils.NewHTTPClient(false),
	}
}

// AvailableReleases contains information about available Java releases
type AvailableReleases struct {
	AvailableLTSReleases     []int `json:"available_lts_releases"`
	AvailableReleases        []int `json:"available_releases"`
	MostRecentFeatureRelease int   `json:"most_recent_feature_release"`
	MostRecentLTS            int   `json:"most_recent_lts"`
}

// Asset represents a downloadable Java asset
type Asset struct {
	Binary struct {
		Architecture string `json:"architecture"`
		ImageType    string `json:"image_type"`
		JVMImpl      string `json:"jvm_impl"`
		OS           string `json:"os"`
		Package      struct {
			Checksum      string `json:"checksum"`
			ChecksumLink  string `json:"checksum_link"`
			DownloadCount int    `json:"download_count"`
			Link          string `json:"link"`
			MetadataLink  string `json:"metadata_link"`
			Name          string `json:"name"`
			Size          int64  `json:"size"`
		} `json:"package"`
		Installer *struct {
			Checksum string `json:"checksum"`
			Link     string `json:"link"`
			Name     string `json:"name"`
			Size     int64  `json:"size"`
		} `json:"installer,omitempty"`
		ScmRef    string `json:"scm_ref"`
		UpdatedAt string `json:"updated_at"`
	} `json:"binary"`
	ReleaseName string `json:"release_name"`
	Version     struct {
		Build          int    `json:"build"`
		Major          int    `json:"major"`
		Minor          int    `json:"minor"`
		OpenJDKVersion string `json:"openjdk_version"`
		Security       int    `json:"security"`
		Semver         string `json:"semver"`
	} `json:"version"`
}

// GetAvailableReleases fetches available Java releases
func (c *Client) GetAvailableReleases(ctx context.Context) (*AvailableReleases, error) {
	url := fmt.Sprintf("%s/info/available_releases", baseURL)
	data, err := c.http.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch available releases: %w", err)
	}

	var releases AvailableReleases
	if err := json.Unmarshal(data, &releases); err != nil {
		return nil, fmt.Errorf("failed to parse available releases: %w", err)
	}

	return &releases, nil
}

// GetLatestAssets fetches the latest assets for a specific version
func (c *Client) GetLatestAssets(ctx context.Context, version int, os, arch, imageType string) ([]Asset, error) {
	url := fmt.Sprintf("%s/assets/latest/%d/hotspot?os=%s&architecture=%s&image_type=%s",
		baseURL, version, os, arch, imageType)

	data, err := c.http.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest assets: %w", err)
	}

	var assets []Asset
	if err := json.Unmarshal(data, &assets); err != nil {
		return nil, fmt.Errorf("failed to parse latest assets: %w", err)
	}

	return assets, nil
}

// GetLatestAsset returns the latest asset for a specific configuration
func (c *Client) GetLatestAsset(ctx context.Context, version int, os, arch, imageType string) (*Asset, error) {
	assets, err := c.GetLatestAssets(ctx, version, os, arch, imageType)
	if err != nil {
		return nil, err
	}

	if len(assets) == 0 {
		return nil, fmt.Errorf("no assets found for Java %d on %s/%s", version, os, arch)
	}

	return &assets[0], nil
}

// GetAssetForCurrentPlatform returns the asset for the current OS and architecture
func (c *Client) GetAssetForCurrentPlatform(ctx context.Context, version int, imageType string) (*Asset, error) {
	os := normalizeOS(runtime.GOOS)
	arch := normalizeArch(runtime.GOARCH)

	return c.GetLatestAsset(ctx, version, os, arch, imageType)
}

// normalizeOS converts Go's GOOS to Adoptium API OS names
func normalizeOS(goos string) string {
	switch goos {
	case "darwin":
		return "mac"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

// normalizeArch converts Go's GOARCH to Adoptium API architecture names
func normalizeArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	case "arm64":
		return "aarch64"
	case "386":
		return "x86"
	default:
		return goarch
	}
}
