package providers

import (
	"context"
	"fmt"
	"sync"

	"github.com/gameap/minecraft-runner/pkg/api/mojang"
)

// ltsJavaVersions are the Java LTS releases available from Adoptium
var ltsJavaVersions = []int{8, 11, 17, 21, 25}

// javaResolver resolves the Java version for a Minecraft version using
// Mojang version metadata, with a heuristic fallback for offline use
type javaResolver struct {
	client *mojang.Client

	mu    sync.Mutex
	cache map[string]int
}

func newJavaResolver(client *mojang.Client) *javaResolver {
	return &javaResolver{
		client: client,
		cache:  make(map[string]int),
	}
}

var defaultJavaResolver = newJavaResolver(mojang.NewClient())

// GetRecommendedJavaVersion returns the recommended Java version for a Minecraft version
func GetRecommendedJavaVersion(ctx context.Context, mcVersion string) int {
	return defaultJavaResolver.Resolve(ctx, mcVersion)
}

// Resolve returns the Java version for a Minecraft version, preferring Mojang metadata
func (r *javaResolver) Resolve(ctx context.Context, mcVersion string) int {
	r.mu.Lock()
	if version, ok := r.cache[mcVersion]; ok {
		r.mu.Unlock()
		return version
	}
	r.mu.Unlock()

	version, err := r.resolveFromMojang(ctx, mcVersion)
	if err != nil || version <= 0 {
		version = fallbackJavaVersion(mcVersion)
	} else {
		version = roundUpToLTS(version)
	}

	r.mu.Lock()
	r.cache[mcVersion] = version
	r.mu.Unlock()

	return version
}

func (r *javaResolver) resolveFromMojang(ctx context.Context, mcVersion string) (int, error) {
	manifest, err := r.client.GetVersionManifest(ctx)
	if err != nil {
		return 0, err
	}

	v := manifest.FindVersion(mcVersion)
	if v == nil {
		return 0, fmt.Errorf("version %s not found in manifest", mcVersion)
	}

	detail, err := r.client.GetVersionDetail(ctx, v.URL)
	if err != nil {
		return 0, err
	}

	return detail.JavaVersion.MajorVersion, nil
}

// roundUpToLTS maps a Java version to the nearest LTS release available
// from Adoptium (e.g. 16 -> 17); versions above the newest known LTS pass through
func roundUpToLTS(version int) int {
	for _, lts := range ltsJavaVersions {
		if lts >= version {
			return lts
		}
	}
	return version
}

// fallbackJavaVersion guesses the Java version from the Minecraft version
// string when Mojang metadata is unavailable
func fallbackJavaVersion(mcVersion string) int {
	if year, ok := parseSnapshotYear(mcVersion); ok {
		// Weekly snapshots map by year: 24w+ is the 1.20.5-1.21.x era (Java 21),
		// 21w-23w is the 1.17-1.20.4 era (Java 17), older snapshots run on Java 8
		switch {
		case year >= 26:
			return 25
		case year >= 24:
			return 21
		case year >= 21:
			return 17
		}
		return 8
	}

	major, minor, patch := parseMinecraftVersion(mcVersion)

	// Year-based versions (26.x and later) require Java 25
	if major >= 26 {
		return 25
	}

	if major == 1 {
		switch {
		case minor >= 21:
			return 21
		case minor == 20 && patch >= 5:
			return 21
		case minor >= 17:
			return 17
		}
	}

	return 8
}

// parseSnapshotYear extracts the year from weekly snapshot IDs like "24w14a"
func parseSnapshotYear(version string) (int, bool) {
	var year, week int
	if n, _ := fmt.Sscanf(version, "%dw%d", &year, &week); n == 2 {
		return year, true
	}
	return 0, false
}

// parseMinecraftVersion parses a Minecraft version string into components
func parseMinecraftVersion(version string) (major, minor, patch int) {
	// Handle formats like "1.20.4", "1.20", "1.20.4-pre1", etc.
	var m, n, p int
	fmt.Sscanf(version, "%d.%d.%d", &m, &n, &p)
	if m == 0 {
		fmt.Sscanf(version, "%d.%d", &m, &n)
	}
	return m, n, p
}
