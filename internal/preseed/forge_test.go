package preseed

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func marshal(t *testing.T, v any) string {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func library(name, url, sha1 string) map[string]any {
	return map[string]any{"name": name, "downloads": map[string]any{
		"artifact": map[string]any{"url": url, "sha1": sha1, "size": 4},
	}}
}

const offlineInstaller = "net/minecraftforge/installer/SimpleInstaller.class"

func TestForgeInstallerSkipsSanitizedMappingsByChecksum(t *testing.T) {
	r := newRepo(t, map[string]string{
		"/server.jar":  "vanilla",
		"/server.txt":  sampleMappings,
		"/maven/a.jar": "aaaa",
		"/maven/b.jar": "bbbb",
	})

	profile := marshal(t, map[string]any{
		"spec":          1,
		"minecraft":     "1.21.4",
		"json":          "/version.json",
		"serverJarPath": "{LIBRARY_DIR}/net/minecraft/server/{MINECRAFT_VERSION}/server-{MINECRAFT_VERSION}-bundled.jar",
		"libraries": []any{
			library("net.minecraftforge:installertools:1.4.3", r.URL+"/maven/a.jar", sha1Hex("aaaa")),
			library("net.minecraftforge:forge:1.21.4-54.1.22:shim", r.URL+"/maven/missing.jar", sha1Hex("shim")),
		},
		"processors": []any{
			map[string]any{
				"jar":     "net.minecraftforge:installertools:1.4.3",
				"args":    []string{"--task", "DOWNLOAD_MOJMAPS", "--sanitize", "--version", "1.21.4", "--side", "{SIDE}", "--output", "{MOJMAPS}"},
				"outputs": map[string]string{"{MOJMAPS}": "{MOJMAPS_SHA}"},
			},
		},
		"data": map[string]any{
			"MOJMAPS":     map[string]string{"client": "[net.minecraft:client:1.21.4:mappings@tsrg]", "server": "[net.minecraft:server:1.21.4:mappings@tsrg]"},
			"MOJMAPS_SHA": map[string]string{"client": "'x'", "server": "'" + sha1Hex(sampleLegacyTSRG) + "'"},
		},
	})
	version := marshal(t, map[string]any{"libraries": []any{
		library("org.ow2.asm:asm:9.7", r.URL+"/maven/b.jar", sha1Hex("bbbb")),
		library("net.minecraftforge:forge:1.21.4-54.1.22", "", ""),
	}})

	dir := t.TempDir()
	writeJar(t, filepath.Join(dir, "forge-installer.jar"), map[string]string{
		"install_profile.json": profile,
		"version.json":         version,
		"maven/net/minecraftforge/forge/1.21.4-54.1.22/forge-1.21.4-54.1.22-shim.jar": "shim",
		offlineInstaller:     "accepts offline",
		"META-INF/FORGE.SF":  "signature",
		"META-INF/FORGE.RSA": "signature",
	})

	versions := &fakeVersions{
		url: r.URL + "/server.jar", sha1: sha1Hex("vanilla"),
		mappingsURL: r.URL + "/server.txt", mappingsSHA1: sha1Hex(sampleMappings),
	}

	install, err := ForgeInstaller(context.Background(), client(), versions, dir, "forge-installer.jar", "Forge")
	if err != nil {
		t.Fatalf("ForgeInstaller: %v", err)
	}
	if install.Installer != "forge-installer.jar" || !install.Offline || install.repacked {
		t.Errorf("install = %+v, want the original installer, offline", install)
	}

	for rel, want := range map[string]string{
		"libraries/net/minecraftforge/installertools/1.4.3/installertools-1.4.3.jar": "aaaa",
		"libraries/org/ow2/asm/asm/9.7/asm-9.7.jar":                                  "bbbb",
		"libraries/net/minecraft/server/1.21.4/server-1.21.4-bundled.jar":            "vanilla",
		"libraries/net/minecraft/server/1.21.4/server-1.21.4-mappings.tsrg":          sampleLegacyTSRG,
	} {
		if got := readFile(t, filepath.Join(dir, filepath.FromSlash(rel))); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}

	leftovers, _ := filepath.Glob(filepath.Join(dir, ".mojmaps-*"))
	if len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}

	before, lookups := r.requests(), versions.calls
	if _, err := ForgeInstaller(context.Background(), client(), versions, dir, "forge-installer.jar", "Forge"); err != nil {
		t.Fatal(err)
	}
	if r.requests() != before || versions.calls != lookups {
		t.Errorf("a prepared installer cost %d requests and %d lookups", r.requests()-before, versions.calls-lookups)
	}
}

func TestForgeInstallerRepacksWithoutMappingsDownload(t *testing.T) {
	r := newRepo(t, map[string]string{
		"/server.jar":       "vanilla",
		"/server.txt":       sampleMappings,
		"/maven/a.jar":      "aaaa",
		"/maven/a.jar.sha1": sha1Hex("aaaa"),
	})

	mojmaps := func(side string) map[string]any {
		return map[string]any{
			"jar":   "net.neoforged.installertools:installertools:2.1.2",
			"sides": []string{side},
			"args":  []string{"--task", "DOWNLOAD_MOJMAPS", "--version", "1.21.1", "--side", "{SIDE}", "--output", "{MOJMAPS}"},
		}
	}
	profile := marshal(t, map[string]any{
		"spec":      1,
		"minecraft": "1.21.1",
		"json":      "/version.json",
		"libraries": []any{},
		"processors": []any{
			mojmaps("client"),
			mojmaps("server"),
			map[string]any{"jar": "net.neoforged.installertools:binarypatcher:2.1.2", "args": []string{"--clean", "{MC_SRG}"}},
		},
		"data": map[string]any{
			"MOJMAPS": map[string]string{"server": "[net.minecraft:server:1.21.1-20240808.144430:mappings@txt]"},
		},
	})
	version := marshal(t, map[string]any{"id": "neoforge", "libraries": []any{
		library("org.example:nosum:1.0", r.URL+"/maven/a.jar", ""),
	}})

	dir := t.TempDir()
	writeJar(t, filepath.Join(dir, "neoforge-installer.jar"), map[string]string{
		"install_profile.json":  profile,
		"version.json":          version,
		"data/":                 "",
		"data/server.lzma":      "patches",
		offlineInstaller:        "accepts offline",
		"META-INF/MANIFEST.MF":  "Manifest-Version: 1.0\r\nMain-Class: x.Y\r\n\r\n",
		"META-INF/NEOFORGE.SF":  "signature",
		"META-INF/NEOFORGE.DSA": "signature",
	})

	versions := &fakeVersions{
		url: r.URL + "/server.jar", sha1: sha1Hex("vanilla"),
		mappingsURL: r.URL + "/server.txt", mappingsSHA1: sha1Hex(sampleMappings),
	}

	install, err := ForgeInstaller(context.Background(), client(), versions, dir, "neoforge-installer.jar", "NeoForge")
	if err != nil {
		t.Fatalf("ForgeInstaller: %v", err)
	}
	if install.Installer != "neoforge-installer-offline.jar" || !install.Offline {
		t.Fatalf("install = %+v, want a repacked offline installer", install)
	}

	mappings := "libraries/net/minecraft/server/1.21.1-20240808.144430/server-1.21.1-20240808.144430-mappings.txt"
	if got := readFile(t, filepath.Join(dir, filepath.FromSlash(mappings))); got != sampleMappings {
		t.Errorf("the raw mappings were not put in place: %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "minecraft_server.1.21.1.jar")); got != "vanilla" {
		t.Errorf("vanilla server = %q", got)
	}

	jar, err := OpenJar(filepath.Join(dir, install.Installer))
	if err != nil {
		t.Fatal(err)
	}

	var repacked struct {
		Processors []forgeProcessor `json:"processors"`
	}
	data, _, _ := jar.Read(installProfile)
	if err := json.Unmarshal(data, &repacked); err != nil {
		t.Fatal(err)
	}
	if len(repacked.Processors) != 2 || repacked.Processors[0].Sides[0] != "client" ||
		repacked.Processors[1].arg("--clean") != "{MC_SRG}" {
		t.Errorf("processors left: %+v", repacked.Processors)
	}

	var repackedVersion struct {
		ID        string         `json:"id"`
		Libraries []forgeLibrary `json:"libraries"`
	}
	data, _, _ = jar.Read("version.json")
	if err := json.Unmarshal(data, &repackedVersion); err != nil {
		t.Fatal(err)
	}
	if repackedVersion.ID != "neoforge" || repackedVersion.Libraries[0].Downloads.Artifact.SHA1 != sha1Hex("aaaa") {
		t.Errorf("version.json = %s", data)
	}

	if patches, _, _ := jar.Read("data/server.lzma"); string(patches) != "patches" {
		t.Errorf("data/server.lzma = %q", patches)
	}
	if jar.Has("META-INF/NEOFORGE.SF") || jar.Has("META-INF/NEOFORGE.DSA") || !jar.Has("META-INF/MANIFEST.MF") {
		t.Error("the signature must go, the manifest must stay")
	}
	jar.Close()

	install.Remove()
	if _, err := os.Stat(filepath.Join(dir, install.Installer)); !os.IsNotExist(err) {
		t.Error("the repacked installer was not removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "neoforge-installer.jar")); err != nil {
		t.Error("the original installer is gone")
	}
}

func TestForgeInstallerWithoutOfflineModeIsNotRepacked(t *testing.T) {
	r := newRepo(t, map[string]string{"/server.jar": "vanilla", "/server.txt": sampleMappings})

	dir := t.TempDir()
	writeJar(t, filepath.Join(dir, "forge-installer.jar"), map[string]string{
		"install_profile.json": marshal(t, map[string]any{
			"minecraft": "1.17.1",
			"json":      "/version.json",
			"processors": []any{map[string]any{
				"args": []string{"--task", "DOWNLOAD_MOJMAPS", "--version", "1.17.1", "--side", "{SIDE}", "--output", "{MOJMAPS}"},
			}},
			"data": map[string]any{"MOJMAPS": map[string]string{"server": "[net.minecraft:server:1.17.1:mappings@txt]"}},
		}),
		"version.json": `{"libraries": []}`,
	})

	versions := &fakeVersions{url: r.URL + "/server.jar", sha1: sha1Hex("vanilla"),
		mappingsURL: r.URL + "/server.txt", mappingsSHA1: sha1Hex(sampleMappings)}

	install, err := ForgeInstaller(context.Background(), client(), versions, dir, "forge-installer.jar", "Forge")
	if err != nil {
		t.Fatal(err)
	}
	if install.Offline || install.Installer != "forge-installer.jar" {
		t.Errorf("install = %+v, want the original installer, online", install)
	}
}

func TestLegacyForgeInstaller(t *testing.T) {
	r := newRepo(t, map[string]string{
		"/server.jar": "vanilla",
		"/forge/org/scala-lang/scala-library/2.11.1/scala-library-2.11.1.jar": "scala",
		"/mc/net/minecraft/launchwrapper/1.12/launchwrapper-1.12.jar":         "wrapper",
		"/mc/net/minecraft/launchwrapper/1.12/launchwrapper-1.12.jar.sha1":    sha1Hex("wrapper"),
	})

	saved := minecraftLibraries
	minecraftLibraries = r.URL + "/mc/"
	t.Cleanup(func() { minecraftLibraries = saved })

	profile := marshal(t, map[string]any{
		"install": map[string]any{"minecraft": "1.7.10", "filePath": "forge-universal.jar"},
		"versionInfo": map[string]any{"libraries": []any{
			map[string]any{"name": "net.minecraftforge:forge:1.7.10-10.13.4.1614-1.7.10", "url": r.URL + "/forge/"},
			map[string]any{"name": "net.minecraft:launchwrapper:1.12", "serverreq": true},
			map[string]any{"name": "org.scala-lang:scala-library:2.11.1", "url": r.URL + "/forge/", "serverreq": true,
				"checksums": []string{sha1Hex("packed"), sha1Hex("scala")}},
			map[string]any{"name": "org.lwjgl:lwjgl:2.9.1", "clientreq": true},
		}},
	})

	dir := t.TempDir()
	writeJar(t, filepath.Join(dir, "forge-installer.jar"), map[string]string{
		"install_profile.json": profile,
		"forge-universal.jar":  "forge",
		offlineInstaller:       "accepts offline",
	})

	versions := &fakeVersions{url: r.URL + "/server.jar", sha1: sha1Hex("vanilla")}
	install, err := ForgeInstaller(context.Background(), client(), versions, dir, "forge-installer.jar", "Forge")
	if err != nil {
		t.Fatalf("ForgeInstaller: %v", err)
	}
	if !install.Offline || install.repacked {
		t.Errorf("install = %+v", install)
	}

	for rel, want := range map[string]string{
		"minecraft_server.1.7.10.jar":                                            "vanilla",
		"libraries/net/minecraft/launchwrapper/1.12/launchwrapper-1.12.jar":      "wrapper",
		"libraries/org/scala-lang/scala-library/2.11.1/scala-library-2.11.1.jar": "scala",
	} {
		if got := readFile(t, filepath.Join(dir, filepath.FromSlash(rel))); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	for _, rel := range []string{"libraries/org/lwjgl", "libraries/net/minecraftforge"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s is not needed by the server", rel)
		}
	}
}

func TestSignatureFile(t *testing.T) {
	for name, want := range map[string]bool{
		"META-INF/FORGE.SF":    true,
		"META-INF/forge.rsa":   true,
		"META-INF/X.EC":        true,
		"META-INF/SIG-X":       true,
		"META-INF/MANIFEST.MF": false,
		"META-INF/sub/X.SF":    false,
		"X.SF":                 false,
	} {
		if got := signatureFile(name); got != want {
			t.Errorf("signatureFile(%q) = %v", name, got)
		}
	}
}
