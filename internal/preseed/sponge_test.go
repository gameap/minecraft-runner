package preseed

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func sha512Hex(s string) string {
	sum := sha512.Sum512([]byte(s))
	return hex.EncodeToString(sum[:])
}

func spongeJar(t *testing.T, dir, version, librariesFile, libraries string) string {
	t.Helper()

	jarPath := filepath.Join(dir, "spongevanilla-"+version+"-universal.jar")
	writeJar(t, jarPath, map[string]string{
		"META-INF/MANIFEST.MF": "Manifest-Version: 1.0\r\nImplementation-Version: " + version + "\r\n\r\n",
		librariesFile:          libraries,
	})

	return jarPath
}

func TestSpongeBundledWithMappings(t *testing.T) {
	bundle := jarBytes(t, map[string]string{
		"META-INF/versions.list":                     sha256Hex("server") + "\t1.21.4\t1.21.4/server-1.21.4.jar\n",
		"META-INF/versions/1.21.4/server-1.21.4.jar": "server",
	})
	r := newRepo(t, map[string]string{
		"/mojang/bundle.jar":   bundle,
		"/mojang/mappings.txt": "mappings",
		"/public/com/lmax/disruptor/3.4.4/disruptor-3.4.4.jar":           "disruptor",
		"/central/org/ow2/asm/asm-tree/9.7/asm-tree-9.7.jar":             "asm-tree",
		"/public/org/ow2/asm/asm-tree/9.7/asm-tree-9.7.jar":              "corrupt",
		"/public/com/google/guava/guava/33.0.0-jre/guava-33.0.0-jre.jar": "guava",
	})
	useRepositories(t, &spongeRepositories, r.URL+"/public/", r.URL+"/central/")

	dir := t.TempDir()
	jarPath := spongeJar(t, dir, "1.21.4-14.0.1-RC2773", "sponge-libraries.json", `{"dependencies": {
		"bootstrap": [
			{"group": "com.lmax", "module": "disruptor", "version": "3.4.4", "sha512": "`+sha512Hex("disruptor")+`"},
			{"group": "org.ow2.asm", "module": "asm-tree", "version": "9.7", "sha512": "`+sha512Hex("asm-tree")+`"}
		],
		"main": [
			{"group": "com.google.guava", "module": "guava", "version": "33.0.0-jre", "sha512": "`+sha512Hex("guava")+`"},
			{"group": "com.lmax", "module": "disruptor", "version": "3.4.4", "sha512": "`+sha512Hex("disruptor")+`"}
		]
	}}`)

	versions := &fakeVersions{
		url: r.URL + "/mojang/bundle.jar", sha1: sha1Hex(bundle),
		mappingsURL: r.URL + "/mojang/mappings.txt", mappingsSHA1: sha1Hex("mappings"),
	}
	if err := Sponge(context.Background(), client(), versions, dir, jarPath); err != nil {
		t.Fatalf("Sponge: %v", err)
	}

	for rel, want := range map[string]string{
		"libraries/com/lmax/disruptor/3.4.4/disruptor-3.4.4.jar":           "disruptor",
		"libraries/org/ow2/asm/asm-tree/9.7/asm-tree-9.7.jar":              "asm-tree",
		"libraries/com/google/guava/guava/33.0.0-jre/guava-33.0.0-jre.jar": "guava",
		"libraries/net/minecraft/1.21.4/minecraft_server-bundle.jar":       bundle,
		"libraries/net/minecraft/1.21.4/minecraft_server.jar":              "server",
		"libraries/net/minecraft/mappings/1.21.4/server.txt":               "mappings",
	} {
		if got := readFile(t, filepath.Join(dir, filepath.FromSlash(rel))); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}

	before := r.requests()
	if err := Sponge(context.Background(), client(), versions, dir, jarPath); err != nil {
		t.Fatal(err)
	}
	if r.requests() != before || versions.calls != 1 {
		t.Errorf("a prepared server cost %d requests and %d lookups", r.requests()-before, versions.calls-1)
	}
}

func TestSpongeWithoutMappings(t *testing.T) {
	bundle := jarBytes(t, map[string]string{
		"META-INF/versions.list":                     sha256Hex("server") + "\t26.1.2\t26.1.2/server-26.1.2.jar\n",
		"META-INF/versions/26.1.2/server-26.1.2.jar": "server",
	})
	r := newRepo(t, map[string]string{"/mojang/bundle.jar": bundle})

	dir := t.TempDir()
	jarPath := spongeJar(t, dir, "26.1.2-19.0.0-RC2789", "sponge-libraries.json", `{"dependencies": {}}`)

	versions := &fakeVersions{url: r.URL + "/mojang/bundle.jar", sha1: sha1Hex(bundle)}
	if err := Sponge(context.Background(), client(), versions, dir, jarPath); err != nil {
		t.Fatalf("Sponge: %v", err)
	}

	if got := readFile(t, filepath.Join(dir, "libraries", "net", "minecraft", "26.1.2", "minecraft_server.jar")); got != "server" {
		t.Errorf("extracted server = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "libraries", "net", "minecraft", "mappings")); !os.IsNotExist(err) {
		t.Error("mappings were prepared for an unobfuscated version")
	}
}

func TestSpongeRepairsATruncatedBundle(t *testing.T) {
	bundle := jarBytes(t, map[string]string{
		"META-INF/versions.list":                     sha256Hex("server") + "\t26.1.2\t26.1.2/server-26.1.2.jar\n",
		"META-INF/versions/26.1.2/server-26.1.2.jar": "server",
	})
	r := newRepo(t, map[string]string{"/mojang/bundle.jar": bundle})

	dir := t.TempDir()
	jarPath := spongeJar(t, dir, "26.1.2-19.0.0-RC2789", "sponge-libraries.json", `{"dependencies": {}}`)
	versions := &fakeVersions{url: r.URL + "/mojang/bundle.jar", sha1: sha1Hex(bundle)}
	if err := Sponge(context.Background(), client(), versions, dir, jarPath); err != nil {
		t.Fatalf("Sponge: %v", err)
	}

	bundlePath := filepath.Join(dir, "libraries", "net", "minecraft", "26.1.2", "minecraft_server-bundle.jar")
	if err := os.WriteFile(bundlePath, []byte(bundle[:len(bundle)/2]), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Sponge(context.Background(), client(), versions, dir, jarPath); err != nil {
		t.Fatalf("Sponge with a truncated bundle: %v", err)
	}
	if readFile(t, bundlePath) != bundle {
		t.Error("the truncated bundle was not fetched again")
	}
	if versions.calls != 2 {
		t.Errorf("%d lookups, want 2", versions.calls)
	}
}

func TestSpongeUnbundledWithMD5(t *testing.T) {
	r := newRepo(t, map[string]string{
		"/mojang/server.jar":   "vanilla",
		"/mojang/mappings.txt": "mappings",
		"/public/cpw/mods/modlauncher/8.1.3/modlauncher-8.1.3.jar": "modlauncher",
	})
	useRepositories(t, &spongeRepositories, r.URL+"/public/")

	dir := t.TempDir()
	jarPath := spongeJar(t, dir, "1.16.5-8.2.1-RC2708", "libraries.json", `{"dependencies": {"bootstrap": [
		{"group": "cpw.mods", "module": "modlauncher", "version": "8.1.3", "md5": "`+md5Hex("modlauncher")+`"}
	]}}`)

	versions := &fakeVersions{
		url: r.URL + "/mojang/server.jar", sha1: sha1Hex("vanilla"),
		mappingsURL: r.URL + "/mojang/mappings.txt", mappingsSHA1: sha1Hex("mappings"),
	}
	if err := Sponge(context.Background(), client(), versions, dir, jarPath); err != nil {
		t.Fatalf("Sponge: %v", err)
	}

	for rel, want := range map[string]string{
		"libraries/cpw/mods/modlauncher/8.1.3/modlauncher-8.1.3.jar": "modlauncher",
		"libraries/net/minecraft/1.16.5/minecraft_server.jar":        "vanilla",
		"libraries/net/minecraft/mappings/1.16.5/server.txt":         "mappings",
	} {
		if got := readFile(t, filepath.Join(dir, filepath.FromSlash(rel))); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
}

func TestSpongeWithoutInstaller(t *testing.T) {
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "spongevanilla-1.12.2-7.4.7.jar")
	writeJar(t, jarPath, map[string]string{"META-INF/MANIFEST.MF": "Manifest-Version: 1.0\r\n\r\n"})

	versions := &fakeVersions{}
	if err := Sponge(context.Background(), client(), versions, dir, jarPath); err != nil {
		t.Fatal(err)
	}
	if versions.calls != 0 {
		t.Error("a jar without an installer needs nothing")
	}
}

func TestSpongeRejectsBadVersion(t *testing.T) {
	dir := t.TempDir()
	jarPath := spongeJar(t, dir, "../x", "sponge-libraries.json", `{"dependencies": {}}`)

	if err := Sponge(context.Background(), client(), &fakeVersions{}, dir, jarPath); err == nil {
		t.Fatal("a version naming a path was accepted")
	}
}
