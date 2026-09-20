//go:build live

package providers

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// TestLiveResolve resolves the default server of every provider against the
// real download APIs. It needs network access and is not part of the regular
// test run:
//
//	go test -tags live ./internal/providers/ -run TestLiveResolve -v
func TestLiveResolve(t *testing.T) {
	all := []Provider{
		NewVanillaProvider(),
		NewPaperProvider(),
		NewFoliaProvider(),
		NewPurpurProvider(),
		NewLeafProvider(),
		NewPufferfishProvider(),
		NewFabricProvider(),
		NewQuiltProvider(),
		NewForgeProvider(),
		NewNeoForgeProvider(),
		NewMohistProvider(),
		NewBannerProvider(),
		NewSpongeVanillaProvider(),
		NewSpigotProvider(),
		NewCraftBukkitProvider(),
		NewCauldronProvider(),
		NewWaterfallProvider(),
		NewVelocityProvider(),
		NewBungeecordProvider(),
	}

	for _, provider := range all {
		t.Run(provider.Name(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			latest, err := provider.GetLatestVersion(ctx)
			if err != nil {
				t.Fatalf("GetLatestVersion: %v", err)
			}

			jar, err := provider.GetServerJar(ctx, latest.MinecraftVersion, "")
			if err != nil {
				t.Fatalf("GetServerJar(%s): %v", latest.MinecraftVersion, err)
			}

			if jar.URL == "" || jar.Filename == "" || jar.Version == "" {
				t.Fatalf("incomplete jar: %+v", jar)
			}

			assertDownloadable(ctx, t, jar.URL)

			t.Logf("%s %s (mod %s): %s [sha256=%t sha1=%t md5=%t]", provider.Name(), jar.Version,
				jar.ModVersion, jar.Filename, jar.SHA256 != "", jar.SHA1 != "", jar.MD5 != "")
		})
	}
}

// TestLiveResolveForgeEras covers the Forge releases whose artifact version
// cannot be assembled from the Minecraft and Forge versions alone
func TestLiveResolveForgeEras(t *testing.T) {
	provider := NewForgeProvider()

	for _, mcVersion := range []string{"1.7.10", "1.8.9", "1.12.2", "1.16.5", "1.20.1", "1.21.1"} {
		t.Run(mcVersion, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			jar, err := provider.GetServerJar(ctx, mcVersion, "")
			if err != nil {
				t.Fatalf("GetServerJar: %v", err)
			}

			assertDownloadable(ctx, t, jar.URL)

			t.Logf("forge %s -> %s", mcVersion, jar.Filename)
		})
	}
}

func assertDownloadable(ctx context.Context, t *testing.T, url string) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("invalid download URL %q: %v", url, err)
	}
	req.Header.Set("Range", "bytes=0-0")
	req.Header.Set("User-Agent", "mcrun-live-test (+https://github.com/gameap/minecraft-runner)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("download of %s failed: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("download of %s answered %d", url, resp.StatusCode)
	}
}
