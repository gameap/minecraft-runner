package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gameap/minecraft-runner/pkg/api/mojang"
)

func TestFallbackJavaVersion(t *testing.T) {
	tests := []struct {
		mcVersion string
		want      int
	}{
		{"1.8.8", 8},
		{"1.16.5", 8},
		{"1.17", 17},
		{"1.17.1", 17},
		{"1.18.2", 17},
		{"1.20.4", 17},
		{"1.20.4-pre1", 17},
		{"1.20.5", 21},
		{"1.21.4", 21},
		{"26.2", 25},
		{"26.3-snapshot-5", 25},
		{"27.1", 25},
		{"26w07a", 25},
		{"25w05a", 21},
		{"24w14a", 21},
		{"23w31a", 17},
		{"21w19a", 17},
		{"20w14a", 8},
		{"", 8},
		{"latest", 8},
	}

	for _, tt := range tests {
		if got := fallbackJavaVersion(tt.mcVersion); got != tt.want {
			t.Errorf("fallbackJavaVersion(%q) = %d, want %d", tt.mcVersion, got, tt.want)
		}
	}
}

func TestRoundUpToLTS(t *testing.T) {
	tests := []struct {
		version int
		want    int
	}{
		{8, 8},
		{11, 11},
		{16, 17},
		{20, 21},
		{21, 21},
		{22, 25},
		{25, 25},
		{26, 26},
	}

	for _, tt := range tests {
		if got := roundUpToLTS(tt.version); got != tt.want {
			t.Errorf("roundUpToLTS(%d) = %d, want %d", tt.version, got, tt.want)
		}
	}
}

func newMojangTestServer(t *testing.T, javaVersions map[string]int, manifestHits *int) *httptest.Server {
	t.Helper()

	var srv *httptest.Server
	mux := http.NewServeMux()

	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, _ *http.Request) {
		if manifestHits != nil {
			*manifestHits++
		}
		entries := make([]string, 0, len(javaVersions))
		for id := range javaVersions {
			entries = append(entries, fmt.Sprintf(`{"id":%q,"type":"release","url":"%s/detail/%s.json"}`, id, srv.URL, id))
		}
		fmt.Fprintf(w, `{"latest":{"release":"26.2","snapshot":"26.2"},"versions":[%s]}`, strings.Join(entries, ","))
	})

	mux.HandleFunc("/detail/", func(w http.ResponseWriter, req *http.Request) {
		id := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/detail/"), ".json")
		fmt.Fprintf(w, `{"id":%q,"javaVersion":{"component":"java-runtime","majorVersion":%d}}`, id, javaVersions[id])
	})

	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestResolver(srv *httptest.Server) *javaResolver {
	return newJavaResolver(mojang.NewClientWithManifestURL(srv.URL + "/manifest.json"))
}

func TestResolveUsesMojangMajorVersion(t *testing.T) {
	srv := newMojangTestServer(t, map[string]int{"26.2": 25, "1.21.4": 21}, nil)
	r := newTestResolver(srv)

	if got := r.Resolve(context.Background(), "26.2"); got != 25 {
		t.Errorf("Resolve(26.2) = %d, want 25", got)
	}
	if got := r.Resolve(context.Background(), "1.21.4"); got != 21 {
		t.Errorf("Resolve(1.21.4) = %d, want 21", got)
	}
}

func TestResolveRoundsNonLTSUpToLTS(t *testing.T) {
	srv := newMojangTestServer(t, map[string]int{"1.17": 16}, nil)
	r := newTestResolver(srv)

	if got := r.Resolve(context.Background(), "1.17"); got != 17 {
		t.Errorf("Resolve(1.17) = %d, want 17", got)
	}
}

func TestResolveFallsBackWhenMajorVersionMissing(t *testing.T) {
	srv := newMojangTestServer(t, map[string]int{"1.8.8": 0}, nil)
	r := newTestResolver(srv)

	if got := r.Resolve(context.Background(), "1.8.8"); got != 8 {
		t.Errorf("Resolve(1.8.8) = %d, want 8", got)
	}
}

func TestResolveFallsBackWhenMojangUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(http.NotFound))
	t.Cleanup(srv.Close)
	r := newTestResolver(srv)

	if got := r.Resolve(context.Background(), "26.2"); got != 25 {
		t.Errorf("Resolve(26.2) = %d, want 25", got)
	}
	if got := r.Resolve(context.Background(), "1.20.4"); got != 17 {
		t.Errorf("Resolve(1.20.4) = %d, want 17", got)
	}
}

func TestResolveFallsBackWhenVersionMissingFromManifest(t *testing.T) {
	srv := newMojangTestServer(t, map[string]int{"1.20.4": 17}, nil)
	r := newTestResolver(srv)

	if got := r.Resolve(context.Background(), "26.2"); got != 25 {
		t.Errorf("Resolve(26.2) = %d, want 25", got)
	}
}

func TestResolveCachesResult(t *testing.T) {
	hits := 0
	srv := newMojangTestServer(t, map[string]int{"26.2": 25}, &hits)
	r := newTestResolver(srv)

	r.Resolve(context.Background(), "26.2")
	r.Resolve(context.Background(), "26.2")

	if hits != 1 {
		t.Errorf("manifest fetched %d times, want 1", hits)
	}
}
