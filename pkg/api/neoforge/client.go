package neoforge

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	apiBaseURL   = "https://maven.neoforged.net/api/maven/versions/releases/net/neoforged"
	mavenBaseURL = "https://maven.neoforged.net/releases/net/neoforged"

	// ArtifactNeoForge is the artifact of every release since Minecraft 1.20.2
	ArtifactNeoForge = "neoforge"

	// ArtifactLegacyForge is the artifact of the 1.20.1 releases, which still
	// used the Forge naming: "1.20.1-47.1.106"
	ArtifactLegacyForge = "forge"

	legacyMinecraftVersion = "1.20.1"

	// NeoForge started with Minecraft 1.20, and Minecraft left the "1.x"
	// numbering for year-based versions with 26.1
	firstMinorVersion = 20
	firstYearVersion  = 26
)

// Client is the NeoForge Maven API client
type Client struct {
	http       *utils.HTTPClient
	apiBaseURL string
}

// NewClient creates a new NeoForge API client
func NewClient() *Client {
	return NewClientWithBaseURL(apiBaseURL)
}

// NewClientWithBaseURL creates a NeoForge API client for a custom API root
func NewClientWithBaseURL(url string) *Client {
	return &Client{
		http:       utils.NewHTTPClient(false),
		apiBaseURL: url,
	}
}

// Release identifies an installable NeoForge release
type Release struct {
	Artifact         string // Maven artifact: "neoforge", or "forge" for 1.20.1
	ArtifactVersion  string // Maven version, the directory name under libraries/
	Version          string // Version as users know it: "21.1.251", "47.1.106"
	MinecraftVersion string
	Stable           bool
}

// InstallerName returns the installer file name of the release
func (r Release) InstallerName() string {
	return fmt.Sprintf("%s-%s-installer.jar", r.Artifact, r.ArtifactVersion)
}

// InstallerURL returns the installer download URL of the release
func (r Release) InstallerURL() string {
	return fmt.Sprintf("%s/%s/%s/%s", mavenBaseURL, r.Artifact, r.ArtifactVersion, r.InstallerName())
}

// GetReleases returns every release, oldest first
func (c *Client) GetReleases(ctx context.Context) ([]Release, error) {
	legacy, err := c.getVersions(ctx, ArtifactLegacyForge)
	if err != nil {
		return nil, err
	}

	modern, err := c.getVersions(ctx, ArtifactNeoForge)
	if err != nil {
		return nil, err
	}

	releases := make([]Release, 0, len(legacy)+len(modern))

	for _, version := range legacy {
		if release, ok := parseLegacyRelease(version); ok {
			releases = append(releases, release)
		}
	}

	for _, version := range modern {
		if release, ok := ParseRelease(version); ok {
			releases = append(releases, release)
		}
	}

	return releases, nil
}

// GetInstallerSHA256 fetches the checksum Maven publishes next to the installer
func (c *Client) GetInstallerSHA256(ctx context.Context, release Release) (string, error) {
	data, err := c.http.Get(ctx, release.InstallerURL()+".sha256")
	if err != nil {
		return "", fmt.Errorf("failed to fetch installer checksum: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func (c *Client) getVersions(ctx context.Context, artifact string) ([]string, error) {
	data, err := c.http.Get(ctx, fmt.Sprintf("%s/%s", c.apiBaseURL, artifact))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s versions: %w", artifact, err)
	}

	var response struct {
		Versions []string `json:"versions"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse %s versions: %w", artifact, err)
	}

	return response.Versions, nil
}

// ParseRelease derives the Minecraft version from a NeoForge version. NeoForge
// drops the leading "1." of the Minecraft version and appends its own build
// number: 1.21.1 -> 21.1.N, 1.21 -> 21.0.N. Year-based Minecraft versions keep
// all their components: 26.1.2 -> 26.1.2.N, 26.2 -> 26.2.0.N.
func ParseRelease(version string) (Release, bool) {
	base, suffix, _ := strings.Cut(version, "-")

	parts := strings.Split(base, ".")
	numbers := make([]int, 0, len(parts))
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return Release{}, false
		}
		numbers = append(numbers, n)
	}

	var minecraft string
	switch {
	case len(numbers) == 3 && numbers[0] >= firstMinorVersion && numbers[0] < firstYearVersion:
		minecraft = joinMinecraftVersion(1, numbers[0], numbers[1])
	case len(numbers) == 4 && numbers[0] >= firstYearVersion:
		minecraft = joinMinecraftVersion(numbers[0], numbers[1], numbers[2])
	default:
		return Release{}, false
	}

	return Release{
		Artifact:         ArtifactNeoForge,
		ArtifactVersion:  version,
		Version:          version,
		MinecraftVersion: minecraft,
		Stable:           suffix == "",
	}, true
}

// joinMinecraftVersion renders a Minecraft version without a trailing ".0"
func joinMinecraftVersion(major, minor, patch int) string {
	if patch == 0 {
		return fmt.Sprintf("%d.%d", major, minor)
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}

func parseLegacyRelease(artifactVersion string) (Release, bool) {
	version, ok := strings.CutPrefix(artifactVersion, legacyMinecraftVersion+"-")
	if !ok {
		return Release{}, false
	}

	return Release{
		Artifact:         ArtifactLegacyForge,
		ArtifactVersion:  artifactVersion,
		Version:          version,
		MinecraftVersion: legacyMinecraftVersion,
		Stable:           !strings.Contains(version, "-"),
	}, true
}
