package preseed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gameap/minecraft-runner/internal/utils"
)

// quantumLeaperRepositories are the repositories QuantumLeaper (Leaf's
// launcher) downloads libraries from; Maven Central replaces the Google mirror
// it uses by default
var quantumLeaperRepositories = []string{
	"https://repo1.maven.org/maven2/",
	"https://repo.papermc.io/repository/maven-public/",
	"https://maven.leafmc.one/snapshots/",
}

// quantumLeaperConfig is QuantumLeaper's config/leaf-loader.yml with its
// defaults. Without the file the first start writes it and exits.
const quantumLeaperConfig = `maven-central-mirrors:
- https://maven-central.storage-download.googleapis.com/maven2/
download-retries: 2
auto-update:
  enabled: false
  channel: stable
`

// Paperclip prepares a server started through Paperclip or QuantumLeaper
// (Paper, Folia, Purpur, Pufferfish, Leaf): the vanilla server the launcher
// patches and, for QuantumLeaper, the libraries it would download. A jar of
// another kind is left alone.
func Paperclip(ctx context.Context, client *utils.HTTPClient, dir, jarPath string) error {
	jar, err := OpenJar(jarPath)
	if err != nil {
		return err
	}
	defer jar.Close()

	switch {
	case jar.Has("META-INF/download-context"):
		return paperclip3(ctx, client, dir, jar)
	case jar.Has("patch.properties"):
		data, _, err := jar.Read("patch.properties")
		if err != nil {
			return err
		}
		return legacyPaperclip(ctx, client, dir, parseProperties(data))
	case jar.Has("patch.json"):
		data, _, err := jar.Read("patch.json")
		if err != nil {
			return err
		}
		var patch map[string]string
		if err := json.Unmarshal(data, &patch); err != nil {
			return fmt.Errorf("failed to parse patch.json: %w", err)
		}
		return legacyPaperclip(ctx, client, dir, patch)
	}

	return nil
}

// paperclip3 handles Paperclip 3 and QuantumLeaper: download-context names the
// vanilla jar, libraries.list the libraries, which are bundled in the jar,
// taken from the vanilla jar, patched or (QuantumLeaper only) downloaded
func paperclip3(ctx context.Context, client *utils.HTTPClient, dir string, jar *Jar) error {
	data, _, err := jar.Read("META-INF/download-context")
	if err != nil {
		return err
	}

	fields := strings.Split(strings.TrimSpace(string(data)), "\t")
	if len(fields) != 3 {
		return fmt.Errorf("malformed download-context: %q", data)
	}

	vanilla := File{Path: "cache/" + fields[2], URLs: mojangURLs(fields[1]), SHA256: fields[0]}
	if err := Ensure(ctx, client, dir, "the vanilla server", []File{vanilla}); err != nil {
		return err
	}

	quantumLeaper := jar.Has("META-INF/leaper.properties")
	if quantumLeaper {
		if err := writeIfMissing(filepath.Join(dir, "config", "leaf-loader.yml"), quantumLeaperConfig); err != nil {
			return err
		}
	}

	libraries, err := downloadedLibraries(jar, filepath.Join(dir, filepath.FromSlash(vanilla.Path)))
	if err != nil {
		return err
	}
	if len(libraries) == 0 {
		return nil
	}
	if !quantumLeaper {
		return fmt.Errorf("%d libraries are neither bundled nor in the vanilla server, the first is %s",
			len(libraries), libraries[0].Path)
	}

	return Ensure(ctx, client, dir, "libraries", libraries)
}

// downloadedLibraries lists the libraries.list entries that the launcher
// finds neither in its own jar nor in the vanilla jar and does not patch
func downloadedLibraries(jar *Jar, vanillaPath string) ([]File, error) {
	data, ok, err := jar.Read("META-INF/libraries.list")
	if err != nil || !ok {
		return nil, err
	}

	patched := make(map[string]bool)
	if patches, _, err := jar.Read("META-INF/patches.list"); err != nil {
		return nil, err
	} else {
		for _, line := range lines(patches) {
			if fields := strings.Split(line, "\t"); len(fields) == 7 && fields[0] == "libraries" {
				patched[fields[6]] = true
			}
		}
	}

	vanilla, err := OpenJar(vanillaPath)
	if err != nil {
		return nil, err
	}
	defer vanilla.Close()

	var files []File
	for _, line := range lines(data) {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			return nil, fmt.Errorf("malformed libraries.list entry: %q", line)
		}

		path := fields[2]
		bundled := "META-INF/libraries/" + path
		if patched[path] || jar.Has(bundled) || vanilla.Has(bundled) {
			continue
		}

		urls := repositoryURLs(quantumLeaperRepositories, path)
		if snapshot := snapshotPath(path); snapshot != "" {
			urls = append(urls, repositoryURLs(quantumLeaperRepositories, snapshot)...)
		}

		files = append(files, File{Path: "libraries/" + path, URLs: urls, SHA256: fields[0]})
	}

	return files, nil
}

// snapshotPath maps a timestamped snapshot, .../1.0-20250101.120000-3/x.jar,
// onto its -SNAPSHOT directory, as QuantumLeaper does
func snapshotPath(path string) string {
	fileStart := strings.LastIndexByte(path, '/')
	if fileStart <= 0 {
		return ""
	}
	dirStart := strings.LastIndexByte(path[:fileStart], '/')
	if dirStart < 0 {
		return ""
	}

	version := path[dirStart+1 : fileStart]
	buildDash := strings.LastIndexByte(version, '-')
	if buildDash < 0 {
		return ""
	}
	stampDash := strings.LastIndexByte(version[:buildDash], '-')
	if stampDash <= 0 || buildDash-stampDash != 16 || version[stampDash+9] != '.' {
		return ""
	}

	return path[:dirStart+1] + version[:stampDash] + "-SNAPSHOT" + path[fileStart:]
}

// legacyPaperclip handles Paperclip 1 and 2 (Paper up to 1.17.1, Purpur up
// to 1.16.5), which cache the vanilla jar as cache/mojang_<version>.jar and
// skip it once cache/patched_<version>.jar is in place
func legacyPaperclip(ctx context.Context, client *utils.HTTPClient, dir string, patch map[string]string) error {
	version, source, original := patch["version"], patch["sourceUrl"], patch["originalHash"]
	if version == "" || source == "" || original == "" {
		return fmt.Errorf("incomplete Paperclip patch data: %v", patch)
	}

	if patched := patch["patchedHash"]; patched != "" {
		dest, err := Resolve(dir, "cache/patched_"+version+".jar")
		if err != nil {
			return err
		}
		if present(dest, File{SHA256: patched}) {
			return nil
		}
	}

	vanilla := File{Path: "cache/mojang_" + version + ".jar", URLs: mojangURLs(source), SHA256: strings.ToLower(original)}

	return Ensure(ctx, client, dir, "the vanilla server", []File{vanilla})
}

// mojangURLs adds piston-data for files addressed by their SHA-1 on Mojang's
// older launcher.mojang.com
func mojangURLs(raw string) []string {
	u, err := url.Parse(raw)
	if err != nil || u.Host != "launcher.mojang.com" || !strings.HasPrefix(u.Path, "/v1/objects/") {
		return []string{raw}
	}

	alternative := *u
	alternative.Host = "piston-data.mojang.com"

	return []string{raw, alternative.String()}
}

// parseProperties reads the subset of Java properties Paperclip writes
func parseProperties(data []byte) map[string]string {
	result := make(map[string]string)

	for _, line := range lines(data) {
		if line[0] == '#' || line[0] == '!' {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		result[strings.TrimSpace(unescapeProperty(key))] = strings.TrimSpace(unescapeProperty(value))
	}

	return result
}

func unescapeProperty(s string) string {
	var b strings.Builder
	escaped := false

	for _, r := range s {
		if escaped || r != '\\' {
			b.WriteRune(r)
			escaped = false
			continue
		}
		escaped = true
	}

	return b.String()
}

func writeIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}
