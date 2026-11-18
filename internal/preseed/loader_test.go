package preseed

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gameap/minecraft-runner/pkg/api/mojang"
)

// fakeVersions serves the vanilla server and the mappings of every version
// from one URL each
type fakeVersions struct {
	url, sha1                 string
	mappingsURL, mappingsSHA1 string
	calls                     int
}

func (f *fakeVersions) GetVersion(_ context.Context, id string) (*mojang.VersionDetail, error) {
	f.calls++
	detail := &mojang.VersionDetail{ID: id}
	detail.Downloads.Server = mojang.Download{URL: f.url, SHA1: f.sha1}
	detail.Downloads.ServerMappings = mojang.Download{URL: f.mappingsURL, SHA1: f.mappingsSHA1}
	return detail, nil
}

func jarBytes(t *testing.T, entries map[string]string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "x.jar")
	writeJar(t, path, entries)
	return readFile(t, path)
}

func TestManifestLinesAreJavaReadable(t *testing.T) {
	long := make([]string, 0, 30)
	for i := range 30 {
		long = append(long, fmt.Sprintf("../../libraries/org/example/lib%d/1.0/lib%d-1.0.jar", i, i))
	}

	path := filepath.Join(t.TempDir(), "launch.jar")
	err := WriteLaunchJar(path, [][2]string{{"Main-Class", "a.B"}, {"Class-Path", strings.Join(long, " ")}},
		map[string]string{"x.properties": "k=v\n"})
	if err != nil {
		t.Fatal(err)
	}

	jar, err := OpenJar(path)
	if err != nil {
		t.Fatal(err)
	}
	defer jar.Close()

	raw, _, _ := jar.Read("META-INF/MANIFEST.MF")
	if !strings.HasPrefix(string(raw), "Manifest-Version: 1.0\r\n") || !strings.HasSuffix(string(raw), "\r\n\r\n") {
		t.Errorf("manifest = %q", raw)
	}
	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 72 {
			t.Errorf("line of %d bytes: %q", len(line), line)
		}
	}

	manifest, err := jar.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest["Main-Class"] != "a.B" || manifest["Class-Path"] != strings.Join(long, " ") {
		t.Errorf("manifest read back as %v", manifest)
	}
	if data, ok, _ := jar.Read("x.properties"); !ok || string(data) != "k=v\n" {
		t.Errorf("x.properties = %q, %v", data, ok)
	}
}

func TestVersionAtMost(t *testing.T) {
	for _, tt := range []struct {
		version, limit string
		want           bool
	}{
		{"0.12.5", "0.12.5", true},
		{"0.12.4", "0.12.5", true},
		{"0.12.6", "0.12.5", false},
		{"0.16.10", "0.12.5", false},
		{"0.12", "0.12.5", true},
		{"0.11.7+build.1", "0.12.5", true},
		{"1.0.0", "0.12.5", false},
	} {
		if got := versionAtMost(tt.version, tt.limit); got != tt.want {
			t.Errorf("versionAtMost(%q, %q) = %v", tt.version, tt.limit, got)
		}
	}
}

func TestFabric(t *testing.T) {
	loaderJar := jarBytes(t, map[string]string{
		"META-INF/MANIFEST.MF": "Manifest-Version: 1.0\r\nMain-Class: net.fabricmc.loader.impl.launch.server.FabricServerLauncher\r\n\r\n",
	})

	files := map[string]string{
		"/server.jar": "vanilla",
		"/maven/net/fabricmc/fabric-loader/0.16.10/fabric-loader-0.16.10.jar": loaderJar,
		"/maven/org/ow2/asm/asm/9.7.1/asm-9.7.1.jar":                          "asm",
		"/maven/org/ow2/asm/asm/9.7.1/asm-9.7.1.jar.sha1":                     sha1Hex("asm"),
	}
	r := newRepo(t, files)

	profile, _ := json.Marshal(map[string]any{
		"mainClass": "net.fabricmc.loader.impl.launch.knot.KnotServer",
		"libraries": []map[string]string{
			{"name": "org.ow2.asm:asm:9.7.1", "url": r.URL + "/maven/"},
			{"name": "net.fabricmc:fabric-loader:0.16.10", "url": r.URL + "/maven/", "sha256": sha256Hex(loaderJar)},
		},
	})
	r.mu.Lock()
	r.files["/v2/versions/loader/1.21.4/0.16.10/server/json"] = string(profile)
	r.mu.Unlock()

	saved := fabricMetaURL
	fabricMetaURL = r.URL
	t.Cleanup(func() { fabricMetaURL = saved })

	versions := &fakeVersions{url: r.URL + "/server.jar", sha1: sha1Hex("vanilla")}
	dir := t.TempDir()

	if err := Fabric(context.Background(), client(), versions, dir, "1.21.4", "0.16.10"); err != nil {
		t.Fatalf("Fabric: %v", err)
	}

	if got := readFile(t, filepath.Join(dir, ".fabric", "server", "1.21.4-server.jar")); got != "vanilla" {
		t.Errorf("vanilla server = %q", got)
	}

	launch := filepath.Join(dir, ".fabric", "server", "fabric-loader-server-0.16.10-minecraft-1.21.4.jar")
	jar, err := OpenJar(launch)
	if err != nil {
		t.Fatal(err)
	}
	manifest, _ := jar.Manifest()
	properties, _, _ := jar.Read("fabric-server-launch.properties")
	jar.Close()

	if manifest["Main-Class"] != "net.fabricmc.loader.impl.launch.server.FabricServerLauncher" {
		t.Errorf("Main-Class = %q", manifest["Main-Class"])
	}
	wantClassPath := "../../libraries/org/ow2/asm/asm/9.7.1/asm-9.7.1.jar " +
		"../../libraries/net/fabricmc/fabric-loader/0.16.10/fabric-loader-0.16.10.jar"
	if manifest["Class-Path"] != wantClassPath {
		t.Errorf("Class-Path = %q", manifest["Class-Path"])
	}
	if string(properties) != "launch.mainClass=net.fabricmc.loader.impl.launch.knot.KnotServer\n" {
		t.Errorf("fabric-server-launch.properties = %q", properties)
	}

	before := r.requests()
	if err := Fabric(context.Background(), client(), versions, dir, "1.21.4", "0.16.10"); err != nil {
		t.Fatal(err)
	}
	if r.requests() != before || versions.calls != 1 {
		t.Errorf("a prepared server cost %d requests and %d version lookups", r.requests()-before, versions.calls-1)
	}

	// A library that went missing makes the launcher reinstall, so it is put back
	os.Remove(filepath.Join(dir, "libraries", "org", "ow2", "asm", "asm", "9.7.1", "asm-9.7.1.jar"))
	if err := Fabric(context.Background(), client(), versions, dir, "1.21.4", "0.16.10"); err != nil {
		t.Fatal(err)
	}
	if !classPathComplete(launch) {
		t.Error("the missing library was not restored")
	}
}

func TestFabricShadedLoadersGetTheVanillaServerOnly(t *testing.T) {
	r := newRepo(t, map[string]string{"/server.jar": "vanilla"})
	versions := &fakeVersions{url: r.URL + "/server.jar", sha1: sha1Hex("vanilla")}
	dir := t.TempDir()

	if err := Fabric(context.Background(), client(), versions, dir, "1.16.5", "0.12.5"); err != nil {
		t.Fatalf("Fabric: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".fabric", "server", "1.16.5-server.jar")); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "libraries")); !os.IsNotExist(err) {
		t.Error("the launcher shades libraries of 0.12.5 itself")
	}
}

func TestQuilt(t *testing.T) {
	files := map[string]string{
		"/server.jar": "vanilla",
		"/quilt/org/quiltmc/quilt-loader/0.29.2/quilt-loader-0.29.2.jar":      "loader",
		"/quilt/org/quiltmc/quilt-loader/0.29.2/quilt-loader-0.29.2.jar.sha1": sha1Hex("loader"),
		"/fabric/net/fabricmc/intermediary/1.21.1/intermediary-1.21.1.jar":    "intermediary",
	}
	r := newRepo(t, files)

	profile, _ := json.Marshal(map[string]any{
		"mainClass":         "org.quiltmc.loader.impl.launch.knot.KnotServer",
		"launcherMainClass": "org.quiltmc.loader.impl.launch.server.QuiltServerLauncher",
		"libraries": []map[string]string{
			{"name": "org.quiltmc:quilt-loader:0.29.2", "url": r.URL + "/quilt/"},
			{"name": "net.fabricmc:intermediary:1.21.1:v2", "url": r.URL + "/fabric/"},
		},
	})
	r.mu.Lock()
	r.files["/v3/versions/loader/1.21.1/0.29.2/server/json"] = string(profile)
	r.mu.Unlock()

	saved := quiltMetaURL
	quiltMetaURL = r.URL
	t.Cleanup(func() { quiltMetaURL = saved })

	dir := t.TempDir()
	versions := &fakeVersions{url: r.URL + "/server.jar", sha1: sha1Hex("vanilla")}

	if err := Quilt(context.Background(), client(), versions, dir, "1.21.1", "0.29.2", "quilt-server-launch-1.21.1-0.29.2.jar"); err != nil {
		t.Fatalf("Quilt: %v", err)
	}

	if got := readFile(t, filepath.Join(dir, "server.jar")); got != "vanilla" {
		t.Errorf("server.jar = %q", got)
	}

	jar, err := OpenJar(filepath.Join(dir, "quilt-server-launch-1.21.1-0.29.2.jar"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, _ := jar.Manifest()
	jar.Close()

	if manifest["Main-Class"] != "org.quiltmc.loader.impl.launch.server.QuiltServerLauncher" {
		t.Errorf("Main-Class = %q", manifest["Main-Class"])
	}
	want := "libraries/net/fabricmc/intermediary/1.21.1/intermediary-1.21.1.jar " +
		"libraries/org/quiltmc/quilt-loader/0.29.2/quilt-loader-0.29.2.jar"
	if manifest["Class-Path"] != want {
		t.Errorf("Class-Path = %q", manifest["Class-Path"])
	}
	if !classPathComplete(filepath.Join(dir, "quilt-server-launch-1.21.1-0.29.2.jar")) {
		t.Error("the launcher lists files that are not there")
	}
}
