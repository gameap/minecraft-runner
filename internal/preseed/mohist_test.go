package preseed

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func useRepositories(t *testing.T, target *[]string, repos ...string) {
	t.Helper()

	saved := *target
	*target = repos
	t.Cleanup(func() { *target = saved })
}

func TestMohistLibraries(t *testing.T) {
	r := newRepo(t, map[string]string{
		"/github/libraries/org/example/b/1.0/b-1.0.jar":           "bbbb",
		"/mojang/server.jar":                                      "vanilla",
		"/maven/libraries/org/example/broken/1.0/broken-1.0.jar":  "corrupt",
		"/github/libraries/org/example/broken/1.0/broken-1.0.jar": "fixed",
	})
	useRepositories(t, &mohistRepositories, r.URL+"/maven/", r.URL+"/github/")

	dir := t.TempDir()
	jarPath := filepath.Join(dir, "mohist-1.16.5-362.jar")
	writeJar(t, jarPath, map[string]string{
		"libraries.txt": "libraries/org/example/a/1.0/a-1.0.jar|" + md5Hex("aaaa") + "|4\n" +
			"libraries/org/example/b/1.0/b-1.0.jar|" + md5Hex("bbbb") + "|4\r\n" +
			"libraries/minecraft_server.1.16.5.jar|" + md5Hex("vanilla") + "|7\n" +
			"libraries/org/example/broken/1.0/broken-1.0.jar|" + md5Hex("fixed") + "|5\n",
		"META-INF/libraries/org/example/a/1.0/a-1.0.jar": "aaaa",
	})

	versions := &fakeVersions{url: r.URL + "/mojang/server.jar", sha1: sha1Hex("vanilla")}
	if err := Mohist(context.Background(), client(), versions, dir, jarPath); err != nil {
		t.Fatalf("Mohist: %v", err)
	}
	if versions.calls != 1 {
		t.Errorf("%d vanilla lookups, want 1", versions.calls)
	}

	for rel, want := range map[string]string{
		"libraries/org/example/b/1.0/b-1.0.jar":           "bbbb",
		"libraries/minecraft_server.1.16.5.jar":           "vanilla",
		"libraries/org/example/broken/1.0/broken-1.0.jar": "fixed",
	} {
		if got := readFile(t, filepath.Join(dir, filepath.FromSlash(rel))); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "libraries", "org", "example", "a")); !os.IsNotExist(err) {
		t.Error("a library the jar carries was downloaded")
	}

	if err := Mohist(context.Background(), client(), versions, dir, jarPath); err != nil {
		t.Fatal(err)
	}
	if versions.calls != 1 {
		t.Error("the vanilla server was looked up again although it is in place")
	}
}

func TestVanillaVersion(t *testing.T) {
	for path, want := range map[string]string{
		"libraries/minecraft_server.1.16.5.jar":                                           "1.16.5",
		"libraries/net/minecraft/1.12.2/minecraft_server.1.12.2.jar":                      "1.12.2",
		"libraries/net/minecraft/server/1.7.10/server-1.7.10.jar":                         "1.7.10",
		"libraries/net/minecraft/server/1.16.5-20210115/server-1.16.5-20210115-extra.jar": "",
		"libraries/org/eclipse/jetty/server-9.4.jar":                                      "",
	} {
		if got := vanillaVersion(path); got != want {
			t.Errorf("%s: version %q, want %q", path, got, want)
		}
	}
}

func TestMohistCarryingEverythingDownloadsNothing(t *testing.T) {
	r := newRepo(t, nil)
	useRepositories(t, &mohistRepositories, r.URL+"/")

	dir := t.TempDir()
	jarPath := filepath.Join(dir, "mohist.jar")
	writeJar(t, jarPath, map[string]string{
		"mohist_libraries.txt": "libraries/net/minecraft/1.12.2/minecraft_server.1.12.2.jar|" + md5Hex("vanilla") + "\n",
		"META-INF/libraries/net/minecraft/1.12.2/minecraft_server.1.12.2.jar": "vanilla",
		"installer.txt": "com/example/x.jar|abc|1\n",
	})

	if err := Mohist(context.Background(), client(), &fakeVersions{}, dir, jarPath); err != nil {
		t.Fatal(err)
	}
	if r.requests() != 0 {
		t.Errorf("%d requests for a self-contained jar", r.requests())
	}
}

func TestBanner(t *testing.T) {
	r := newRepo(t, map[string]string{
		"/server.jar": "vanilla",
		"/mohist/org/yaml/snakeyaml/2.2/snakeyaml-2.2.jar":                      "yaml",
		"/sponge/net/fabricmc/intermediary/1.21.1/intermediary-1.21.1.jar":      "intermediary",
		"/fabric/org/ow2/asm/asm/9.8/asm-9.8.jar":                               "asm",
		"/fabric/net/fabricmc/sponge-mixin/0.15.5/sponge-mixin-0.15.5.jar":      "mixin",
		"/fabric/net/fabricmc/sponge-mixin/0.15.5/sponge-mixin-0.15.5.jar.sha1": sha1Hex("mixin"),
	})

	loaderJar := jarBytes(t, map[string]string{
		"fabric-installer.json": `{"libraries": {
			"common": [{"name": "org.ow2.asm:asm:9.8", "url": "` + r.URL + `/fabric/", "sha1": "` + sha1Hex("asm") + `"}],
			"server": [{"name": "net.fabricmc:sponge-mixin:0.15.5", "url": "` + r.URL + `/fabric/"}],
			"client": [{"name": "org.lwjgl:lwjgl:3.3.3", "url": "` + r.URL + `/fabric/"}]
		}}`,
	})
	r.mu.Lock()
	r.files["/sponge/net/fabricmc/fabric-loader/0.16.14/fabric-loader-0.16.14.jar"] = loaderJar
	r.mu.Unlock()
	loaderSum := sha1Hex(loaderJar)

	useRepositories(t, &bannerRepositories, r.URL+"/mohist/", r.URL+"/sponge/")

	installJSON := `{
		"installer": {"minecraft": "1.21.1", "fabricLoader": "0.16.14", "fabricLoaderHash": "` + loaderSum + `"},
		"libraries": {"org.yaml:snakeyaml:2.2": "` + sha1Hex("yaml") + `"},
		"fabricExtra": {"net.fabricmc:intermediary:1.21.1": "` + sha1Hex("intermediary") + `"}
	}`

	dir := t.TempDir()
	jarPath := filepath.Join(dir, "banner-1.21.1-170.jar")
	writeJar(t, jarPath, map[string]string{"installer.json": installJSON, "fabric.mod.json": "{}"})

	versions := &fakeVersions{url: r.URL + "/server.jar", sha1: sha1Hex("vanilla")}
	if err := Mohist(context.Background(), client(), versions, dir, jarPath); err != nil {
		t.Fatalf("Banner: %v", err)
	}

	for rel, want := range map[string]string{
		"libraries/org/yaml/snakeyaml/2.2/snakeyaml-2.2.jar":                 "yaml",
		"libraries/net/fabricmc/intermediary/1.21.1/intermediary-1.21.1.jar": "intermediary",
		"libraries/org/ow2/asm/asm/9.8/asm-9.8.jar":                          "asm",
		"libraries/net/fabricmc/sponge-mixin/0.15.5/sponge-mixin-0.15.5.jar": "mixin",
		"libraries/net/minecraft/server/1.21.1/server-1.21.1.jar":            "vanilla",
	} {
		if got := readFile(t, filepath.Join(dir, filepath.FromSlash(rel))); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "libraries", "org", "lwjgl")); !os.IsNotExist(err) {
		t.Error("client libraries are not needed")
	}

	before := r.requests()
	if err := Mohist(context.Background(), client(), versions, dir, jarPath); err != nil {
		t.Fatal(err)
	}
	if r.requests() != before {
		t.Errorf("a prepared Banner cost %d requests", r.requests()-before)
	}
}

func TestBannerCarryingItsServer(t *testing.T) {
	r := newRepo(t, map[string]string{})
	useRepositories(t, &bannerRepositories, r.URL+"/")

	loaderJar := jarBytes(t, map[string]string{"fabric-installer.json": `{"libraries": {}}`})
	r.files["/net/fabricmc/fabric-loader/0.16.14/fabric-loader-0.16.14.jar"] = loaderJar

	dir := t.TempDir()
	jarPath := filepath.Join(dir, "banner.jar")
	writeJar(t, jarPath, map[string]string{
		"installer.json": `{"installer": {"minecraft": "1.20.1", "fabricLoader": "0.16.14", "fabricLoaderHash": "` +
			sha1Hex(loaderJar) + `"}, "libraries": {}}`,
		"fabric.mod.json":                 "{}",
		"META-INF/jars/server-1.20.1.jar": "banner's server",
	})

	versions := &fakeVersions{}
	if err := Mohist(context.Background(), client(), versions, dir, jarPath); err != nil {
		t.Fatal(err)
	}
	if versions.calls != 0 {
		t.Error("the vanilla server was looked up although Banner carries its own")
	}
	if _, err := os.Stat(filepath.Join(dir, "libraries", "net", "minecraft")); !os.IsNotExist(err) {
		t.Error("Banner extracts its server itself")
	}
}
