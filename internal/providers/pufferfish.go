package providers

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/gameap/minecraft-runner/pkg/api/jenkins"
)

const (
	pufferfishJenkinsURL = "https://ci.pufferfish.host"

	// pufferfishJobPrefix is followed by the Minecraft line a job builds: "Pufferfish-1.21"
	pufferfishJobPrefix = "Pufferfish-"
)

// pufferfishArtifactVersion extracts the Minecraft version from an artifact
// name such as "pufferfish-paperclip-1.21.10-R0.1-SNAPSHOT-mojmap.jar"
var pufferfishArtifactVersion = regexp.MustCompile(`paperclip-(\d+(?:\.\d+)+)-`)

// PufferfishProvider provides Pufferfish server downloads. Pufferfish publishes
// through Jenkins with one job per Minecraft line, each build targeting
// whichever patch version was current when it ran.
type PufferfishProvider struct {
	client *jenkins.Client
}

// pufferfishBuild is a Jenkins build that produced a server JAR
type pufferfishBuild struct {
	job              string
	number           int
	minecraftVersion string
	artifactPath     string
}

// NewPufferfishProvider creates a new Pufferfish provider
func NewPufferfishProvider() *PufferfishProvider {
	return &PufferfishProvider{
		client: jenkins.NewClient(pufferfishJenkinsURL),
	}
}

// Name returns the provider name
func (p *PufferfishProvider) Name() string {
	return "pufferfish"
}

// ListVersions returns the Minecraft versions Pufferfish has builds for, newest first
func (p *PufferfishProvider) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	lines, err := p.minecraftLines(ctx)
	if err != nil {
		return nil, err
	}

	var versions []VersionInfo
	seen := make(map[string]bool)

	for _, line := range lines {
		builds, err := p.jobBuilds(ctx, line)
		if err != nil {
			return nil, err
		}

		for _, build := range builds {
			if seen[build.minecraftVersion] {
				continue
			}
			seen[build.minecraftVersion] = true

			versions = append(versions, VersionInfo{
				MinecraftVersion: build.minecraftVersion,
				ModVersion:       strconv.Itoa(build.number),
				IsStable:         true,
				Type:             "release",
			})
		}
	}

	sortVersionInfosDesc(versions)

	return versions, nil
}

// ListModVersions returns builds for a specific Minecraft version, newest first
func (p *PufferfishProvider) ListModVersions(ctx context.Context, mcVersion string) ([]VersionInfo, error) {
	builds, err := p.jobBuilds(ctx, minecraftLine(mcVersion))
	if err != nil {
		return nil, err
	}

	var versions []VersionInfo
	for _, build := range builds {
		if build.minecraftVersion != mcVersion {
			continue
		}
		versions = append(versions, VersionInfo{
			MinecraftVersion: mcVersion,
			ModVersion:       strconv.Itoa(build.number),
			IsStable:         true,
			Type:             "build",
		})
	}

	return versions, nil
}

// GetLatestVersion returns the Minecraft version of the newest Pufferfish build
func (p *PufferfishProvider) GetLatestVersion(ctx context.Context) (*VersionInfo, error) {
	lines, err := p.minecraftLines(ctx)
	if err != nil {
		return nil, err
	}

	for _, line := range lines {
		builds, err := p.jobBuilds(ctx, line)
		if err != nil {
			return nil, err
		}
		if len(builds) > 0 {
			return &VersionInfo{
				MinecraftVersion: builds[0].minecraftVersion,
				ModVersion:       strconv.Itoa(builds[0].number),
				IsStable:         true,
				Type:             "release",
			}, nil
		}
	}

	return nil, fmt.Errorf("no Pufferfish builds available")
}

// GetServerJar returns download info for a specific version
func (p *PufferfishProvider) GetServerJar(ctx context.Context, mcVersion, modVersion string) (*ServerJar, error) {
	builds, err := p.jobBuilds(ctx, minecraftLine(mcVersion))
	if err != nil {
		return nil, err
	}

	build, err := pickPufferfishBuild(builds, mcVersion, modVersion)
	if err != nil {
		return nil, err
	}

	return &ServerJar{
		Version:    build.minecraftVersion,
		ModVersion: strconv.Itoa(build.number),
		URL:        p.client.Job(build.job).GetArtifactURL(build.number, build.artifactPath),
		Filename:   fmt.Sprintf("pufferfish-%s-%d.jar", build.minecraftVersion, build.number),
	}, nil
}

// pickPufferfishBuild returns the newest build for the Minecraft version. A
// version given as a line ("1.21") matches the newest build of that line.
func pickPufferfishBuild(builds []pufferfishBuild, mcVersion, modVersion string) (*pufferfishBuild, error) {
	number := 0
	if modVersion != "" {
		n, err := strconv.Atoi(modVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid build number: %s", modVersion)
		}
		number = n
	}

	available := make(map[string]bool)

	for i := range builds {
		build := &builds[i]
		available[build.minecraftVersion] = true

		if build.minecraftVersion != mcVersion && minecraftLine(build.minecraftVersion) != mcVersion {
			continue
		}
		if number == 0 || build.number == number {
			return build, nil
		}
	}

	if number != 0 {
		return nil, fmt.Errorf("build %d of Pufferfish not found for Minecraft %s", number, mcVersion)
	}

	versions := make([]string, 0, len(available))
	for version := range available {
		versions = append(versions, version)
	}
	sortMCVersionsDesc(versions)

	return nil, fmt.Errorf("no Pufferfish build found for Minecraft %s (available: %s)",
		mcVersion, strings.Join(versions, ", "))
}

// minecraftLines returns the Minecraft lines Pufferfish has a job for, newest first
func (p *PufferfishProvider) minecraftLines(ctx context.Context) ([]string, error) {
	jobs, err := p.client.GetJobs(ctx)
	if err != nil {
		return nil, err
	}

	var lines []string
	for _, job := range jobs {
		line, ok := strings.CutPrefix(job.Name, pufferfishJobPrefix)
		if !ok || strings.Contains(line, "-") {
			continue
		}
		lines = append(lines, line)
	}

	sortMCVersionsDesc(lines)

	return lines, nil
}

// jobBuilds returns the successful builds of a Minecraft line that produced a server JAR, newest first
func (p *PufferfishProvider) jobBuilds(ctx context.Context, line string) ([]pufferfishBuild, error) {
	job := pufferfishJobPrefix + line

	jenkinsBuilds, err := p.client.Job(job).GetBuildsWithArtifacts(ctx)
	if err != nil {
		return nil, fmt.Errorf("no Pufferfish builds for Minecraft %s: %w", line, err)
	}

	var builds []pufferfishBuild
	for _, b := range jenkinsBuilds {
		if !b.IsSuccessful() {
			continue
		}

		for _, artifact := range b.Artifacts {
			match := pufferfishArtifactVersion.FindStringSubmatch(artifact.FileName)
			if match == nil {
				continue
			}

			builds = append(builds, pufferfishBuild{
				job:              job,
				number:           b.Number,
				minecraftVersion: match[1],
				artifactPath:     artifact.RelativePath,
			})
			break
		}
	}

	return builds, nil
}

// minecraftLine reduces a Minecraft version to its line: "1.21.8" -> "1.21"
func minecraftLine(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return strings.Join(parts, ".")
}

// PostDownload handles post-download steps (none for Pufferfish)
func (p *PufferfishProvider) PostDownload(_ context.Context, _ string, _ *ServerJar, _ string) error {
	return nil
}

// GetRecommendedJavaVersion returns the recommended Java version
func (p *PufferfishProvider) GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return GetRecommendedJavaVersion(ctx, mcVersion)
}
