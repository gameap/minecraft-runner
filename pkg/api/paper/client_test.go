package paper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

const projectJSON = `{
  "project": {"id": "paper", "name": "Paper"},
  "versions": {
    "26.3": ["26.3", "26.3-rc-3"],
    "26.2": ["26.2"],
    "1.21": ["1.21.11", "1.21.10", "1.21.4"],
    "1.9": ["1.9.4"]
  }
}`

func buildJSON(id int, channel string) string {
	return fmt.Sprintf(`{"id": %d, "time": "2026-09-19T19:49:19Z", "channel": %q, "downloads": {
		"module:cmd_kick": {"name": "cmd_kick-%d.jar", "checksums": {"sha256": "aaaa"}, "size": 1, "url": "https://example.com/cmd_kick.jar"},
		"server:default": {"name": "paper-%d.jar", "checksums": {"sha256": "sha-%d"}, "size": 2, "url": "https://example.com/paper-%d.jar"}
	}}`, id, channel, id, id, id, id)
}

func newTestServer(t *testing.T, builds map[string]string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/projects/paper" {
			fmt.Fprint(w, projectJSON)
			return
		}

		version := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/projects/paper/versions/"), "/builds")
		body, ok := builds[version]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)

	return server
}

func TestProjectVersionsKeepAPIOrder(t *testing.T) {
	server := newTestServer(t, nil)

	// Decoding the groups into a map would shuffle them, so one run proves little
	for i := 0; i < 20; i++ {
		project, err := NewClientWithBaseURL(server.URL).GetProject(context.Background(), "paper")
		if err != nil {
			t.Fatalf("GetProject: %v", err)
		}

		want := []string{"26.3", "26.3-rc-3", "26.2", "1.21.11", "1.21.10", "1.21.4", "1.9.4"}
		if !reflect.DeepEqual(project.Versions, want) {
			t.Fatalf("Versions = %v, want %v", project.Versions, want)
		}
		if project.ID != "paper" || project.Name != "Paper" {
			t.Fatalf("project = %q/%q", project.ID, project.Name)
		}
	}
}

func TestFlattenVersionGroupsAcceptsPlainArray(t *testing.T) {
	got, err := flattenVersionGroups([]byte(`["1.21", "1.20"]`))
	if err != nil {
		t.Fatalf("flattenVersionGroups: %v", err)
	}
	if want := []string{"1.21", "1.20"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGetLatestBuild(t *testing.T) {
	server := newTestServer(t, map[string]string{
		"1.21.11": "[" + buildJSON(12, "BETA") + "," + buildJSON(11, "STABLE") + "," + buildJSON(10, "STABLE") + "]",
		"26.3":    "[" + buildJSON(25, "ALPHA") + "," + buildJSON(24, "ALPHA") + "]",
		"26.2":    "[]",
	})
	client := NewClientWithBaseURL(server.URL)

	t.Run("the newest stable build wins over a newer beta", func(t *testing.T) {
		build, err := client.GetLatestBuild(context.Background(), "paper", "1.21.11")
		if err != nil {
			t.Fatalf("GetLatestBuild: %v", err)
		}
		if build.ID != 11 || !build.IsStable() {
			t.Errorf("build = %d (%s), want 11 (STABLE)", build.ID, build.Channel)
		}

		download, ok := build.ServerDownload()
		if !ok {
			t.Fatal("no server download")
		}
		if download.Checksums.SHA256 != "sha-11" || download.Name != "paper-11.jar" {
			t.Errorf("download = %+v", download)
		}
	})

	t.Run("a version without stable builds falls back to its newest build", func(t *testing.T) {
		build, err := client.GetLatestBuild(context.Background(), "paper", "26.3")
		if err != nil {
			t.Fatalf("GetLatestBuild: %v", err)
		}
		if build.ID != 25 || build.IsStable() {
			t.Errorf("build = %d (%s), want 25 (ALPHA)", build.ID, build.Channel)
		}
	})

	t.Run("a version without builds is an error", func(t *testing.T) {
		if _, err := client.GetLatestBuild(context.Background(), "paper", "26.2"); err == nil {
			t.Error("expected an error")
		}
	})
}

func TestGetBuildInfo(t *testing.T) {
	server := newTestServer(t, map[string]string{
		"1.21.11": "[" + buildJSON(12, "BETA") + "," + buildJSON(11, "STABLE") + "]",
	})
	client := NewClientWithBaseURL(server.URL)

	build, err := client.GetBuildInfo(context.Background(), "paper", "1.21.11", 12)
	if err != nil {
		t.Fatalf("GetBuildInfo: %v", err)
	}
	if build.ID != 12 {
		t.Errorf("build = %d, want 12", build.ID)
	}

	if _, err := client.GetBuildInfo(context.Background(), "paper", "1.21.11", 99); err == nil {
		t.Error("expected an error for an unknown build")
	}
}

func TestGetLatestStableVersion(t *testing.T) {
	t.Run("versions that only have unstable builds are skipped", func(t *testing.T) {
		server := newTestServer(t, map[string]string{
			"26.3":      "[" + buildJSON(25, "ALPHA") + "]",
			"26.3-rc-3": "[" + buildJSON(3, "ALPHA") + "]",
			"26.2":      "[" + buildJSON(125, "STABLE") + "]",
			"1.21.11":   "[" + buildJSON(11, "STABLE") + "]",
		})

		version, err := NewClientWithBaseURL(server.URL).GetLatestStableVersion(context.Background(), "paper")
		if err != nil {
			t.Fatalf("GetLatestStableVersion: %v", err)
		}
		if version != "26.2" {
			t.Errorf("version = %q, want 26.2", version)
		}
	})

	t.Run("without any stable build the newest version is used", func(t *testing.T) {
		builds := map[string]string{}
		for _, v := range []string{"26.3", "26.3-rc-3", "26.2", "1.21.11", "1.21.10", "1.21.4", "1.9.4"} {
			builds[v] = "[" + buildJSON(1, "BETA") + "]"
		}
		server := newTestServer(t, builds)

		version, err := NewClientWithBaseURL(server.URL).GetLatestStableVersion(context.Background(), "paper")
		if err != nil {
			t.Fatalf("GetLatestStableVersion: %v", err)
		}
		if version != "26.3" {
			t.Errorf("version = %q, want 26.3", version)
		}
	})
}
