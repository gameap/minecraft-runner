package preseed

import (
	"archive/zip"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/gameap/minecraft-runner/internal/utils"
)

func TestMain(m *testing.M) {
	utils.SetMirrors(nil)
	os.Exit(m.Run())
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func sha1Hex(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// repo is a fake download server that counts requests per path
type repo struct {
	*httptest.Server
	mu    sync.Mutex
	files map[string]string
	hits  map[string]int
}

func newRepo(t *testing.T, files map[string]string) *repo {
	t.Helper()

	r := &repo{files: files, hits: map[string]int{}}
	r.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.hits[req.URL.Path]++
		body, ok := r.files[req.URL.Path]
		r.mu.Unlock()

		if !ok {
			http.NotFound(w, req)
			return
		}
		fmt.Fprint(w, body)
	}))
	t.Cleanup(r.Close)

	return r
}

func (r *repo) requests() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := 0
	for _, hits := range r.hits {
		n += hits
	}
	return n
}

func writeJar(t *testing.T, path string, entries map[string]string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(entries[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func client() *utils.HTTPClient {
	return utils.NewHTTPClient(false)
}

func TestMavenPath(t *testing.T) {
	for coord, want := range map[string]string{
		"net.fabricmc:fabric-loader:0.16.10":              "net/fabricmc/fabric-loader/0.16.10/fabric-loader-0.16.10.jar",
		"net.minecraftforge:forge:1.20.1-47.4.10:server":  "net/minecraftforge/forge/1.20.1-47.4.10/forge-1.20.1-47.4.10-server.jar",
		"de.oceanlabs.mcp:mcp_config:1.20.1-20230612@zip": "de/oceanlabs/mcp/mcp_config/1.20.1-20230612/mcp_config-1.20.1-20230612.zip",
		"net.minecraft:server:1.20.1:mappings@txt":        "net/minecraft/server/1.20.1/server-1.20.1-mappings.txt",
	} {
		if got, err := MavenPath(coord); err != nil || got != want {
			t.Errorf("MavenPath(%q) = %q, %v; want %q", coord, got, err, want)
		}
	}

	for _, coord := range []string{"a:b", "a:b:c:d:e", "a:..:1", "a:b/c:1", "a:b:1@", ":b:1"} {
		if got, err := MavenPath(coord); err == nil {
			t.Errorf("MavenPath(%q) = %q, want an error", coord, got)
		}
	}
}

func TestResolveStaysInsideTheDirectory(t *testing.T) {
	dir := t.TempDir()

	if got, err := Resolve(dir, "libraries/a/b.jar"); err != nil || got != filepath.Join(dir, "libraries", "a", "b.jar") {
		t.Errorf("Resolve = %q, %v", got, err)
	}

	for _, rel := range []string{"", ".", "..", "../x", "a/../../x", "/etc/passwd", `a\b`, "C:/x"} {
		if got, err := Resolve(dir, rel); err == nil {
			t.Errorf("Resolve(%q) = %q, want an error", rel, got)
		}
	}
}

func TestSnapshotPath(t *testing.T) {
	for path, want := range map[string]string{
		"org/x/lib/1.0-20250101.120000-3/lib-1.0-20250101.120000-3.jar": "org/x/lib/1.0-SNAPSHOT/lib-1.0-20250101.120000-3.jar",
		"org/x/lib/1.0/lib-1.0.jar":                                     "",
		"org/x/lib/1.0-SNAPSHOT/lib-1.0-SNAPSHOT.jar":                   "",
		"lib.jar": "",
	} {
		if got := snapshotPath(path); got != want {
			t.Errorf("snapshotPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestEnsure(t *testing.T) {
	r := newRepo(t, map[string]string{
		"/good/a.jar":      "a",
		"/bad/b.jar":       "tampered",
		"/good/b.jar":      "b",
		"/side/c.jar":      "c",
		"/side/c.jar.sha1": sha1Hex("c") + "  c.jar\n",
		"/plain/d.jar":     "d",
	})
	dir := t.TempDir()

	files := []File{
		{Path: "libs/a.jar", URLs: []string{r.URL + "/good/a.jar"}, SHA256: sha256Hex("a")},
		{Path: "libs/b.jar", URLs: []string{r.URL + "/missing/b.jar", r.URL + "/bad/b.jar", r.URL + "/good/b.jar"}, MD5: md5Hex("b")},
		{Path: "libs/c.jar", URLs: []string{r.URL + "/side/c.jar"}, Sidecar: "sha1"},
		{Path: "libs/d.jar", URLs: []string{r.URL + "/plain/d.jar"}, Sidecar: "sha1"},
	}

	if err := Ensure(context.Background(), client(), dir, "test", files); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	for name, want := range map[string]string{"a.jar": "a", "b.jar": "b", "c.jar": "c", "d.jar": "d"} {
		if got := readFile(t, filepath.Join(dir, "libs", name)); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "libs", "*.part*")); len(matches) != 0 {
		t.Errorf("leftovers: %v", matches)
	}

	before := r.requests()
	if err := Ensure(context.Background(), client(), dir, "test", files); err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	if r.requests() != before {
		t.Errorf("a prepared directory cost %d more requests", r.requests()-before)
	}

	// A file that went bad is fetched again
	os.WriteFile(filepath.Join(dir, "libs", "a.jar"), []byte("rotten"), 0644)
	if err := Ensure(context.Background(), client(), dir, "test", files); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, "libs", "a.jar")); got != "a" {
		t.Errorf("a.jar = %q after repair", got)
	}
}

func TestEnsureReportsCancellation(t *testing.T) {
	r := newRepo(t, map[string]string{"/x.jar": "x"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	files := make([]File, 0, 20)
	for i := range 20 {
		files = append(files, File{Path: fmt.Sprintf("libs/%d.jar", i), URLs: []string{r.URL + "/x.jar"}, SHA1: sha1Hex("x")})
	}

	if err := Ensure(ctx, client(), t.TempDir(), "test", files); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the cancellation", err)
	}
}

func TestEnsureFailsOnMismatch(t *testing.T) {
	r := newRepo(t, map[string]string{"/x.jar": "tampered"})
	dir := t.TempDir()

	err := Ensure(context.Background(), client(), dir, "test", []File{
		{Path: "x.jar", URLs: []string{r.URL + "/x.jar"}, SHA1: sha1Hex("x")},
	})
	if err == nil || !strings.Contains(err.Error(), "x.jar") {
		t.Fatalf("err = %v, want a mismatch naming x.jar", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "x.jar")); !os.IsNotExist(statErr) {
		t.Error("a file that failed its checksum must not be left in place")
	}
}

func TestPaperclip3(t *testing.T) {
	vanilla := map[string]string{"META-INF/libraries/com/mojang/brigadier/1.0/brigadier-1.0.jar": "brigadier"}
	vanillaPath := filepath.Join(t.TempDir(), "vanilla.jar")
	writeJar(t, vanillaPath, vanilla)
	vanillaBytes := readFile(t, vanillaPath)

	r := newRepo(t, map[string]string{
		"/v1/objects/abc/server.jar":                         vanillaBytes,
		"/maven2/org/x/needed/1.0/needed-1.0.jar":            "needed",
		"/maven2/org/x/needed2/1.0/needed2-1.0.jar":          "needed2",
		"/maven2/org/x/bundled/1.0/bundled-1.0.jar":          "never",
		"/maven2/com/mojang/brigadier/1.0/brigadier-1.0.jar": "never",
	})
	saved := quantumLeaperRepositories
	quantumLeaperRepositories = []string{r.URL + "/maven2/"}
	t.Cleanup(func() { quantumLeaperRepositories = saved })

	libraries := strings.Join([]string{
		sha256Hex("needed") + "\torg.x:needed:1.0\torg/x/needed/1.0/needed-1.0.jar",
		sha256Hex("bundled") + "\torg.x:bundled:1.0\torg/x/bundled/1.0/bundled-1.0.jar",
		sha256Hex("brigadier") + "\tcom.mojang:brigadier:1.0\tcom/mojang/brigadier/1.0/brigadier-1.0.jar",
		sha256Hex("patched") + "\torg.x:patched:2.0\torg/x/patched/2.0/patched-2.0.jar",
		sha256Hex("needed2") + "\torg.x:needed2:1.0\torg/x/needed2/1.0/needed2-1.0.jar",
	}, "\n")
	downloadContext := sha256Hex(vanillaBytes) + "\t" + r.URL + "/v1/objects/abc/server.jar\tmojang_26.3.jar"

	t.Run("QuantumLeaper", func(t *testing.T) {
		dir := t.TempDir()
		jarPath := filepath.Join(dir, "leaf-26.3-22.jar")
		writeJar(t, jarPath, map[string]string{
			"META-INF/download-context":                            downloadContext,
			"META-INF/leaper.properties":                           "update-api-type=BIBLIOTHEK\n",
			"META-INF/libraries.list":                              libraries,
			"META-INF/libraries/org/x/bundled/1.0/bundled-1.0.jar": "bundled",
			"META-INF/patches.list":                                "libraries\ta\tb\tc\torg/x/patched/1.0/patched-1.0.jar\torg/x/patched/2.0/patched-1.0.jar.patch\torg/x/patched/2.0/patched-2.0.jar",
		})

		if err := Paperclip(context.Background(), client(), dir, jarPath); err != nil {
			t.Fatalf("Paperclip: %v", err)
		}

		if got := readFile(t, filepath.Join(dir, "cache", "mojang_26.3.jar")); got != vanillaBytes {
			t.Error("the vanilla jar was not cached")
		}
		for path, want := range map[string]string{
			"libraries/org/x/needed/1.0/needed-1.0.jar":   "needed",
			"libraries/org/x/needed2/1.0/needed2-1.0.jar": "needed2",
		} {
			if got := readFile(t, filepath.Join(dir, filepath.FromSlash(path))); got != want {
				t.Errorf("%s = %q", path, got)
			}
		}
		for _, path := range []string{"libraries/org/x/bundled", "libraries/com/mojang", "libraries/org/x/patched"} {
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(path))); !os.IsNotExist(err) {
				t.Errorf("%s must be left to the launcher", path)
			}
		}
		if got := readFile(t, filepath.Join(dir, "config", "leaf-loader.yml")); got != quantumLeaperConfig {
			t.Errorf("leaf-loader.yml = %q", got)
		}

		before := r.requests()
		if err := Paperclip(context.Background(), client(), dir, jarPath); err != nil {
			t.Fatal(err)
		}
		if r.requests() != before {
			t.Error("a prepared server must cost no requests")
		}
	})

	t.Run("QuantumLeaper with its own Maven Central mirror", func(t *testing.T) {
		dir := t.TempDir()
		r.mu.Lock()
		r.files["/custom/org/x/needed/1.0/needed-1.0.jar"] = "needed"
		builtIn := r.hits["/maven2/org/x/needed/1.0/needed-1.0.jar"]
		r.mu.Unlock()
		config := "maven-central-mirrors:\n- " + r.URL + "/custom\n- " + quantumLeaperDefaultMirror + "\ndownload-retries: 2\n"
		if err := writeIfMissing(filepath.Join(dir, "config", "leaf-loader.yml"), config); err != nil {
			t.Fatal(err)
		}
		jarPath := filepath.Join(dir, "leaf-26.3-22.jar")
		writeJar(t, jarPath, map[string]string{
			"META-INF/download-context":                            downloadContext,
			"META-INF/leaper.properties":                           "update-api-type=BIBLIOTHEK\n",
			"META-INF/libraries.list":                              libraries,
			"META-INF/libraries/org/x/bundled/1.0/bundled-1.0.jar": "bundled",
			"META-INF/patches.list":                                "libraries\ta\tb\tc\torg/x/patched/1.0/patched-1.0.jar\torg/x/patched/2.0/patched-1.0.jar.patch\torg/x/patched/2.0/patched-2.0.jar",
		})

		if err := Paperclip(context.Background(), client(), dir, jarPath); err != nil {
			t.Fatalf("Paperclip: %v", err)
		}

		if got := readFile(t, filepath.Join(dir, "libraries", "org", "x", "needed", "1.0", "needed-1.0.jar")); got != "needed" {
			t.Errorf("needed-1.0.jar = %q", got)
		}
		if r.hits["/custom/org/x/needed/1.0/needed-1.0.jar"] != 1 || r.hits["/maven2/org/x/needed/1.0/needed-1.0.jar"] != builtIn {
			t.Error("the configured mirror must be asked before the built-in repositories")
		}
		if got := readFile(t, filepath.Join(dir, "libraries", "org", "x", "needed2", "1.0", "needed2-1.0.jar")); got != "needed2" {
			t.Errorf("needed2-1.0.jar = %q, the built-in repositories must stay the fallback", got)
		}

		repositories, err := quantumLeaperMirrors(dir)
		if err != nil {
			t.Fatal(err)
		}
		want := append([]string{r.URL + "/custom/"}, quantumLeaperRepositories...)
		if !reflect.DeepEqual(repositories, want) {
			t.Errorf("repositories = %v, want %v", repositories, want)
		}
	})

	t.Run("Paperclip 3 bundles every library", func(t *testing.T) {
		dir := t.TempDir()
		jarPath := filepath.Join(dir, "paper.jar")
		writeJar(t, jarPath, map[string]string{
			"META-INF/download-context": downloadContext,
			"META-INF/libraries.list": sha256Hex("brigadier") +
				"\tcom.mojang:brigadier:1.0\tcom/mojang/brigadier/1.0/brigadier-1.0.jar",
		})

		if err := Paperclip(context.Background(), client(), dir, jarPath); err != nil {
			t.Fatalf("Paperclip: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "config", "leaf-loader.yml")); !os.IsNotExist(err) {
			t.Error("leaf-loader.yml is QuantumLeaper's only")
		}
	})
}

func TestLegacyPaperclip(t *testing.T) {
	r := newRepo(t, map[string]string{"/v1/objects/5faf/server.jar": "vanilla-1.8.8"})

	properties := fmt.Sprintf("#comment\nversion=1.8.8\npatchedHash=%s\nsourceUrl=%s\noriginalHash=%s\npatch=paperMC.patch\n",
		strings.ToUpper(sha256Hex("patched")),
		strings.ReplaceAll(r.URL, ":", `\:`)+"/v1/objects/5faf/server.jar",
		strings.ToUpper(sha256Hex("vanilla-1.8.8")))

	dir := t.TempDir()
	jarPath := filepath.Join(dir, "paper-1.8.8-445.jar")
	writeJar(t, jarPath, map[string]string{"patch.properties": properties})

	if err := Paperclip(context.Background(), client(), dir, jarPath); err != nil {
		t.Fatalf("Paperclip: %v", err)
	}
	if got := readFile(t, filepath.Join(dir, "cache", "mojang_1.8.8.jar")); got != "vanilla-1.8.8" {
		t.Errorf("mojang_1.8.8.jar = %q", got)
	}

	// Once patched the vanilla jar is not needed any more
	other := t.TempDir()
	os.MkdirAll(filepath.Join(other, "cache"), 0755)
	os.WriteFile(filepath.Join(other, "cache", "patched_1.8.8.jar"), []byte("patched"), 0644)
	before := r.requests()
	if err := Paperclip(context.Background(), client(), other, jarPath); err != nil {
		t.Fatal(err)
	}
	if r.requests() != before {
		t.Error("a patched server must not fetch the vanilla jar")
	}
}

func TestPaperclipIgnoresOtherJars(t *testing.T) {
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "velocity.jar")
	writeJar(t, jarPath, map[string]string{"META-INF/MANIFEST.MF": "Manifest-Version: 1.0\n"})

	if err := Paperclip(context.Background(), client(), dir, jarPath); err != nil {
		t.Errorf("Paperclip: %v", err)
	}
}

func TestMojangURLs(t *testing.T) {
	got := mojangURLs("https://launcher.mojang.com/v1/objects/5faf/server.jar")
	want := []string{
		"https://launcher.mojang.com/v1/objects/5faf/server.jar",
		"https://piston-data.mojang.com/v1/objects/5faf/server.jar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mojangURLs = %v", got)
	}
}

func TestProxyModules(t *testing.T) {
	r := newRepo(t, map[string]string{"/cmd_alert.jar": "alert", "/cmd_kick.jar": "kick"})
	modules := []File{
		{Path: "modules/cmd_alert.jar", URLs: []string{r.URL + "/cmd_alert.jar"}, SHA256: sha256Hex("alert")},
		{Path: "modules/cmd_kick.jar", URLs: []string{r.URL + "/cmd_kick.jar"}, SHA256: sha256Hex("kick")},
	}

	t.Run("a new proxy gets every module", func(t *testing.T) {
		dir := t.TempDir()
		if err := ProxyModules(context.Background(), client(), dir, modules); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"cmd_alert.jar", "cmd_kick.jar"} {
			if _, err := os.Stat(filepath.Join(dir, "modules", name)); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
	})

	t.Run("modules.yml decides", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "modules.yml"), []byte("modules:\n- jenkins://cmd_alert\nversion: 3\n"), 0644)
		if err := ProxyModules(context.Background(), client(), dir, modules); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "modules", "cmd_kick.jar")); !os.IsNotExist(err) {
			t.Error("a module removed from modules.yml must stay removed")
		}
	})
}
