package providers

import (
	"strings"
	"testing"

	"github.com/gameap/minecraft-runner/pkg/api/leaf"
	"github.com/gameap/minecraft-runner/pkg/api/mohist"
	"github.com/gameap/minecraft-runner/pkg/api/neoforge"
)

func neoForgeReleases(t *testing.T, versions ...string) []neoforge.Release {
	t.Helper()

	releases := make([]neoforge.Release, 0, len(versions))
	for _, version := range versions {
		release, ok := neoforge.ParseRelease(version)
		if !ok {
			t.Fatalf("ParseRelease(%q) failed", version)
		}
		releases = append(releases, release)
	}
	return releases
}

func TestPickNeoForgeRelease(t *testing.T) {
	releases := neoForgeReleases(t,
		"21.1.1-beta", "21.1.250", "21.1.251", "21.1.252-beta", "26.3.0.6-beta", "26.3.0.7-beta")

	tests := []struct {
		name, mc, mod, want string
		wantErr             bool
	}{
		{name: "newest stable wins over a newer beta", mc: "1.21.1", want: "21.1.251"},
		{name: "explicit version", mc: "1.21.1", mod: "21.1.250", want: "21.1.250"},
		{name: "only betas yet", mc: "26.3", want: "26.3.0.7-beta"},
		{name: "unknown Minecraft version", mc: "1.19.2", wantErr: true},
		{name: "version of another Minecraft version", mc: "26.3", mod: "21.1.251", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release, err := pickNeoForgeRelease(releases, tt.mc, tt.mod)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", release)
				}
				return
			}
			if err != nil {
				t.Fatalf("pickNeoForgeRelease: %v", err)
			}
			if release.Version != tt.want {
				t.Errorf("version = %q, want %q", release.Version, tt.want)
			}
		})
	}
}

func TestPickPufferfishBuild(t *testing.T) {
	builds := []pufferfishBuild{
		{job: "Pufferfish-1.21", number: 39, minecraftVersion: "1.21.10"},
		{job: "Pufferfish-1.21", number: 38, minecraftVersion: "1.21.10"},
		{job: "Pufferfish-1.21", number: 33, minecraftVersion: "1.21.8"},
		{job: "Pufferfish-1.21", number: 32, minecraftVersion: "1.21.8"},
	}

	tests := []struct {
		name, mc, mod string
		want          int
		wantErr       string
	}{
		{name: "newest build of the exact version", mc: "1.21.8", want: 33},
		{name: "a line matches its newest build", mc: "1.21", want: 39},
		{name: "explicit build", mc: "1.21.8", mod: "32", want: 32},
		{name: "build of another version", mc: "1.21.8", mod: "39", wantErr: "not found"},
		{name: "unbuilt version lists what exists", mc: "1.21.5", wantErr: "1.21.10, 1.21.8"},
		{name: "invalid build number", mc: "1.21.8", mod: "abc", wantErr: "invalid build number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			build, err := pickPufferfishBuild(builds, tt.mc, tt.mod)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("pickPufferfishBuild: %v", err)
			}
			if build.number != tt.want {
				t.Errorf("build = %d, want %d", build.number, tt.want)
			}
		})
	}
}

func TestPufferfishArtifactVersion(t *testing.T) {
	match := pufferfishArtifactVersion.FindStringSubmatch("pufferfish-paperclip-1.21.10-R0.1-SNAPSHOT-mojmap.jar")
	if match == nil || match[1] != "1.21.10" {
		t.Errorf("match = %v, want 1.21.10", match)
	}

	if got := minecraftLine("1.21.8"); got != "1.21" {
		t.Errorf("minecraftLine(1.21.8) = %q", got)
	}
	if got := minecraftLine("1.21"); got != "1.21" {
		t.Errorf("minecraftLine(1.21) = %q", got)
	}
}

func leafBuild(number int, channel string) leaf.Build {
	build := leaf.Build{Build: number, Channel: channel}
	build.Downloads.Primary.Name = "leaf.jar"
	return build
}

func TestPickLeafBuild(t *testing.T) {
	builds := []leaf.Build{leafBuild(10, "default"), leafBuild(11, "default"), leafBuild(12, "experimental")}

	build, err := pickLeafBuild(builds, "1.21.8", "")
	if err != nil || build.Build != 11 {
		t.Errorf("default channel: build = %+v, err = %v, want 11", build, err)
	}

	build, err = pickLeafBuild([]leaf.Build{leafBuild(5, "experimental"), leafBuild(6, "experimental")}, "26.2", "")
	if err != nil || build.Build != 6 {
		t.Errorf("experimental only: build = %+v, err = %v, want 6", build, err)
	}

	build, err = pickLeafBuild(builds, "1.21.8", "12")
	if err != nil || build.Build != 12 {
		t.Errorf("explicit: build = %+v, err = %v, want 12", build, err)
	}

	if _, err := pickLeafBuild(builds, "1.21.8", "99"); err == nil {
		t.Error("expected an error for an unknown build")
	}
	if _, err := pickLeafBuild(nil, "1.21.8", ""); err == nil {
		t.Error("expected an error without builds")
	}
}

func TestPickMohistBuild(t *testing.T) {
	builds := []mohist.Build{{ID: 471}, {ID: 470}, {ID: 54}}

	build, err := pickMohistBuild(builds, "1.20.1", "")
	if err != nil || build.ID != 471 {
		t.Errorf("newest: build = %+v, err = %v", build, err)
	}

	build, err = pickMohistBuild(builds, "1.20.1", "54")
	if err != nil || build.ID != 54 {
		t.Errorf("explicit: build = %+v, err = %v", build, err)
	}

	if _, err := pickMohistBuild(builds, "1.20.1", "999"); err == nil {
		t.Error("expected an error for an unknown build")
	}
	if _, err := pickMohistBuild(builds, "1.20.1", "abc"); err == nil {
		t.Error("expected an error for an invalid build number")
	}
	if _, err := pickMohistBuild(nil, "1.21.4", ""); err == nil {
		t.Error("expected an error without builds")
	}
}
