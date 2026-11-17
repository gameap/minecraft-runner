package preseed

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/gameap/minecraft-runner/internal/utils"
)

// spongeRepositories serve the libraries SpongeVanilla lists. Its installer
// finds them through the search of Sponge's Nexus, which hands out
// maven-public download links.
var spongeRepositories = []string{
	"https://repo.spongepowered.org/repository/maven-public/",
	"https://repo1.maven.org/maven2/",
}

type spongeLibraries struct {
	Dependencies map[string][]struct {
		Group   string `json:"group"`
		Module  string `json:"module"`
		Version string `json:"version"`
		SHA512  string `json:"sha512"`
		MD5     string `json:"md5"`
	} `json:"dependencies"`
}

// Sponge prepares what the installer of the SpongeVanilla jar at jarPath
// downloads: its libraries, the vanilla server and, up to Minecraft 1.21.11,
// Mojang's mappings.
//
// The installer reads Mojang's version manifest on every start and, when it
// cannot, carries on with the server it finds: from 26.1 the one extracted
// here, in 1.16.5 the one it remapped on an earlier start. 1.18.2 to 1.21.11
// pick the server before remapping, which does not start, so they need Mojang.
func Sponge(ctx context.Context, client *utils.HTTPClient, versions Versions, dir, jarPath string) error {
	jar, err := OpenJar(jarPath)
	if err != nil {
		return err
	}
	defer jar.Close()

	files, err := spongeLibraryFiles(jar)
	if err != nil || files == nil {
		return err
	}

	game, err := spongeMinecraftVersion(jar)
	if err != nil {
		return err
	}

	minecraftDir := "libraries/net/minecraft/" + game + "/"
	bundled := !versionAtMost(game, "1.17.1")
	server := minecraftDir + "minecraft_server.jar"
	if bundled {
		server = minecraftDir + "minecraft_server-bundle.jar"
	}
	mappings := ""
	if versionAtMost(game, "1.21.11") {
		mappings = "libraries/net/minecraft/mappings/" + game + "/server.txt"
	}

	vanilla, err := spongeVanillaFiles(ctx, versions, dir, game, server, mappings)
	if err != nil {
		return err
	}

	if err := Ensure(ctx, client, dir, "SpongeVanilla", append(files, vanilla...)); err != nil {
		return err
	}

	if !bundled {
		return nil
	}

	return extractBundledServer(dir, server, minecraftDir+"minecraft_server.jar")
}

// spongeLibraryFiles lists the libraries of sponge-libraries.json (SHA-512)
// or of the older libraries.json (MD5); nil when the jar has neither
func spongeLibraryFiles(jar *Jar) ([]File, error) {
	name := "sponge-libraries.json"
	data, ok, err := jar.Read(name)
	if err == nil && !ok {
		name = "libraries.json"
		data, ok, err = jar.Read(name)
	}
	if err != nil || !ok {
		return nil, err
	}

	var libraries spongeLibraries
	if err := json.Unmarshal(data, &libraries); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", name, err)
	}

	files := []File{}
	seen := make(map[string]bool)
	for _, collection := range slices.Sorted(func(yield func(string) bool) {
		for key := range libraries.Dependencies {
			if !yield(key) {
				return
			}
		}
	}) {
		for _, dep := range libraries.Dependencies[collection] {
			if dep.Group == "" || dep.Module == "" || dep.Version == "" || dep.SHA512 == "" && dep.MD5 == "" {
				return nil, fmt.Errorf("incomplete library %s:%s:%s in %s", dep.Group, dep.Module, dep.Version, name)
			}

			libPath := path.Join(strings.ReplaceAll(dep.Group, ".", "/"), dep.Module, dep.Version,
				dep.Module+"-"+dep.Version+".jar")
			if seen[libPath] {
				continue
			}
			seen[libPath] = true

			files = append(files, File{
				Path:   "libraries/" + libPath,
				URLs:   repositoryURLs(spongeRepositories, libPath),
				SHA512: dep.SHA512,
				MD5:    dep.MD5,
			})
		}
	}

	return files, nil
}

// spongeMinecraftVersion reads the Minecraft version from the build's
// Implementation-Version, such as 1.21.4-14.0.1-RC2773
func spongeMinecraftVersion(jar *Jar) (string, error) {
	manifest, err := jar.Manifest()
	if err != nil {
		return "", err
	}

	game, _, _ := strings.Cut(manifest["Implementation-Version"], "-")
	if game == "" || strings.ContainsAny(game, `/\`) || game == "." || game == ".." {
		return "", fmt.Errorf("the SpongeVanilla jar names no Minecraft version")
	}

	return game, nil
}

// spongeVanillaFiles describes the vanilla server and the mappings that are
// not in place yet. What is in place the installer checks itself, so Mojang
// is not asked on every start.
func spongeVanillaFiles(ctx context.Context, versions Versions, dir, game, server, mappings string) ([]File, error) {
	var missing []string
	for _, rel := range []string{server, mappings} {
		if rel == "" {
			continue
		}
		exists, err := pathExists(dir, rel)
		if err != nil {
			return nil, err
		}
		if !exists {
			missing = append(missing, rel)
		}
	}
	if len(missing) == 0 {
		return nil, nil
	}

	detail, err := versions.GetVersion(ctx, game)
	if err != nil {
		return nil, fmt.Errorf("failed to look up Minecraft %s: %w", game, err)
	}

	var files []File
	for _, rel := range missing {
		download := detail.Downloads.Server
		if rel == mappings {
			download = detail.Downloads.ServerMappings
		}
		if download.URL == "" {
			return nil, fmt.Errorf("minecraft %s has no download for %s", game, path.Base(rel))
		}

		files = append(files, File{Path: rel, URLs: mojangURLs(download.URL), SHA1: download.SHA1})
	}

	return files, nil
}

// extractBundledServer extracts the server from Mojang's bundle the way the
// installer does, to where it looks when Mojang is out of reach
func extractBundledServer(dir, bundle, server string) error {
	bundlePath, err := Resolve(dir, bundle)
	if err != nil {
		return err
	}
	serverPath, err := Resolve(dir, server)
	if err != nil {
		return err
	}

	jar, err := OpenJar(bundlePath)
	if err != nil {
		return err
	}
	defer jar.Close()

	data, ok, err := jar.Read("META-INF/versions.list")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	entries := lines(data)
	if len(entries) == 0 {
		return fmt.Errorf("%s lists no server", bundle)
	}

	fields := strings.Split(entries[0], "\t")
	if len(fields) != 3 {
		return fmt.Errorf("malformed line %q in the versions.list of %s", entries[0], bundle)
	}

	return jar.Extract("META-INF/versions/"+fields[2], serverPath, fields[0])
}
