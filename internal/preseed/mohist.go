package preseed

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/gameap/minecraft-runner/internal/utils"
)

// mohistRepositories serve the libraries Mohist lists in libraries.txt and
// mohist_libraries.txt, in the order Mohist tries them
var mohistRepositories = []string{"https://maven.mohistmc.com/", "https://mohistmc.github.io/maven/"}

// bannerRepositories are the Maven repositories Banner's installer knows,
// with Maven Central for what Mohist's repositories proxy
var bannerRepositories = []string{
	"https://maven.mohistmc.com/libraries/",
	"https://mohistmc.github.io/maven/libraries/",
	"https://repo.spongepowered.org/maven/",
	"https://maven.fabricmc.net/",
	"https://libraries.minecraft.net/",
	"https://repo1.maven.org/maven2/",
	"https://maven.izzel.io/releases/",
}

// vanillaLibrary matches the vanilla servers in Mohist's library lists
var vanillaLibrary = regexp.MustCompile(`(?:^|/)minecraft_server\.([^/]+)\.jar$|/net/minecraft/server/([^/]+)/server-([^/]+)\.jar$`)

// Mohist prepares what a MohistMC server (Mohist, Banner, Cauldron) at
// jarPath downloads on its first start. Mohist 1.12.2 and the builds with an
// installer.txt carry their libraries inside the jar and download nothing.
func Mohist(ctx context.Context, client *utils.HTTPClient, versions Versions, dir, jarPath string) error {
	jar, err := OpenJar(jarPath)
	if err != nil {
		return err
	}
	defer jar.Close()

	if jar.Has("installer.json") && jar.Has("fabric.mod.json") {
		return banner(ctx, client, versions, dir, jar)
	}

	for _, list := range []string{"libraries.txt", "mohist_libraries.txt"} {
		if jar.Has(list) {
			return mohistLibraries(ctx, client, versions, dir, jar, list)
		}
	}

	return nil
}

// mohistLibraries handles the "path|md5[|size]" lists. Mohist copies the
// libraries it carries under META-INF/ itself; the others are downloaded.
func mohistLibraries(
	ctx context.Context, client *utils.HTTPClient, versions Versions, dir string, jar *Jar, list string,
) error {
	data, _, err := jar.Read(list)
	if err != nil {
		return err
	}

	var files []File
	for _, line := range lines(data) {
		parts := strings.Split(line, "|")
		if len(parts) < 2 || !strings.HasPrefix(parts[0], "libraries/") {
			return fmt.Errorf("malformed line %q in %s", line, list)
		}

		if jar.Has("META-INF/" + parts[0]) {
			continue
		}

		file := File{Path: parts[0], URLs: repositoryURLs(mohistRepositories, parts[0]), MD5: parts[1]}
		if err := addMojangSource(ctx, versions, dir, &file); err != nil {
			return err
		}
		files = append(files, file)
	}

	return Ensure(ctx, client, dir, "Mohist", files)
}

// addMojangSource lets a missing vanilla server come from Mojang: Mohist's
// repositories lost some of them, Mohist 1.16.5's among them
func addMojangSource(ctx context.Context, versions Versions, dir string, file *File) error {
	version := vanillaVersion(file.Path)
	if version == "" {
		return nil
	}

	dest, err := Resolve(dir, file.Path)
	if err != nil {
		return err
	}
	if present(dest, *file) {
		return nil
	}

	server, err := VanillaServer(ctx, versions, version, file.Path)
	if err != nil {
		fmt.Printf("[WARNING] %v\n", err)
		return nil
	}
	file.URLs = append(file.URLs, server.URLs...)

	return nil
}

// vanillaVersion tells which vanilla server a library path holds, if any
func vanillaVersion(path string) string {
	match := vanillaLibrary.FindStringSubmatch(path)
	if match == nil || match[2] != match[3] {
		return ""
	}

	return match[1] + match[2]
}

type bannerInstall struct {
	Installer struct {
		Minecraft        string `json:"minecraft"`
		FabricLoader     string `json:"fabricLoader"`
		FabricLoaderHash string `json:"fabricLoaderHash"`
	} `json:"installer"`
	Libraries   map[string]string `json:"libraries"`
	FabricExtra map[string]string `json:"fabricExtra"`
}

// banner prepares what Banner's installer checks before it runs: its
// libraries, the Fabric loader with the libraries the loader lists and the
// vanilla server. Missing any of them, it downloads Mojang's version manifest.
func banner(ctx context.Context, client *utils.HTTPClient, versions Versions, dir string, jar *Jar) error {
	data, _, err := jar.Read("installer.json")
	if err != nil {
		return err
	}

	var install bannerInstall
	if err := json.Unmarshal(data, &install); err != nil {
		return fmt.Errorf("failed to parse Banner's installer.json: %w", err)
	}
	game, loader := install.Installer.Minecraft, install.Installer.FabricLoader
	if game == "" || loader == "" {
		return fmt.Errorf("banner's installer.json names no Minecraft or Fabric version")
	}

	libraries := make(map[string]string, len(install.Libraries)+len(install.FabricExtra)+1)
	for coord, sum := range install.Libraries {
		libraries[coord] = sum
	}
	for coord, sum := range install.FabricExtra {
		libraries[coord] = sum
	}
	loaderCoord := "net.fabricmc:fabric-loader:" + loader
	libraries[loaderCoord] = install.Installer.FabricLoaderHash

	files := make([]File, 0, len(libraries)+1)
	for _, coord := range slices.Sorted(func(yield func(string) bool) {
		for coord := range libraries {
			if !yield(coord) {
				return
			}
		}
	}) {
		libPath, err := MavenPath(coord)
		if err != nil {
			return err
		}
		files = append(files, File{Path: "libraries/" + libPath, URLs: repositoryURLs(bannerRepositories, libPath), SHA1: libraries[coord]})
	}

	// The server Banner carries replaces the vanilla one
	serverJar := fmt.Sprintf("libraries/net/minecraft/server/%[1]s/server-%[1]s.jar", game)
	if !jar.Has("META-INF/jars/server-" + game + ".jar") {
		if exists, err := pathExists(dir, serverJar); err != nil {
			return err
		} else if !exists {
			server, err := VanillaServer(ctx, versions, game, serverJar)
			if err != nil {
				return err
			}
			files = append(files, server)
		}
	}

	if err := Ensure(ctx, client, dir, "Banner", files); err != nil {
		return err
	}

	loaderPath, _ := MavenPath(loaderCoord)
	loaderLibraries, err := fabricInstallerLibraries(dir, "libraries/"+loaderPath)
	if err != nil {
		return err
	}

	return Ensure(ctx, client, dir, "Fabric", loaderLibraries)
}

// fabricInstallerLibraries lists the libraries of the fabric-installer.json
// a Fabric loader jar carries
func fabricInstallerLibraries(dir, loaderJar string) ([]File, error) {
	loaderPath, err := Resolve(dir, loaderJar)
	if err != nil {
		return nil, err
	}

	jar, err := OpenJar(loaderPath)
	if err != nil {
		return nil, err
	}
	defer jar.Close()

	data, ok, err := jar.Read("fabric-installer.json")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%s has no fabric-installer.json", loaderJar)
	}

	var installer struct {
		Libraries map[string][]struct {
			Name string `json:"name"`
			URL  string `json:"url"`
			SHA1 string `json:"sha1"`
		} `json:"libraries"`
	}
	if err := json.Unmarshal(data, &installer); err != nil {
		return nil, fmt.Errorf("failed to parse the fabric-installer.json of %s: %w", loaderJar, err)
	}

	var files []File
	for _, side := range []string{"common", "server"} {
		for _, lib := range installer.Libraries[side] {
			libPath, err := MavenPath(lib.Name)
			if err != nil {
				return nil, err
			}

			urls := repositoryURLs(bannerRepositories, libPath)
			if lib.URL != "" {
				urls = append([]string{strings.TrimRight(lib.URL, "/") + "/" + libPath}, urls...)
			}

			file := File{Path: "libraries/" + libPath, URLs: urls, SHA1: lib.SHA1}
			if file.SHA1 == "" {
				file.Sidecar = "sha1"
			}
			files = append(files, file)
		}
	}

	return files, nil
}
