package preseed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gameap/minecraft-runner/internal/utils"
)

var (
	fabricMetaURL = "https://meta.fabricmc.net"
	quiltMetaURL  = "https://meta.quiltmc.org"
)

const (
	// fabricDataDir is where the Fabric server launcher keeps its install
	fabricDataDir = ".fabric/server"

	// fabricDefaultLauncher starts loaders whose manifest names no main class
	fabricDefaultLauncher = "net.fabricmc.loader.launch.server.FabricServerLauncher"
)

// loaderProfile is the server profile Fabric and Quilt meta publish for a
// game and loader version
type loaderProfile struct {
	MainClass         string `json:"mainClass"`
	LauncherMainClass string `json:"launcherMainClass"`
	Libraries         []struct {
		Name   string `json:"name"`
		URL    string `json:"url"`
		SHA512 string `json:"sha512"`
		SHA256 string `json:"sha256"`
		SHA1   string `json:"sha1"`
		MD5    string `json:"md5"`
	} `json:"libraries"`
}

func fetchProfile(ctx context.Context, client *utils.HTTPClient, url string) (*loaderProfile, error) {
	data, err := client.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the loader profile: %w", err)
	}

	var profile loaderProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse the loader profile: %w", err)
	}
	if len(profile.Libraries) == 0 {
		return nil, fmt.Errorf("the loader profile %s lists no libraries", url)
	}

	return &profile, nil
}

func profileURL(base, format, game, loader string) string {
	return base + fmt.Sprintf(format, url.PathEscape(game), url.PathEscape(loader))
}

// Fabric prepares what the Fabric server launcher installs on its first
// start: the vanilla server, the loader's libraries and the launch jar that
// lists them. The launcher finds them all in place and installs nothing.
// Loaders up to 0.12.5 have their libraries shaded into the launch jar
// instead; for them only the vanilla server is prepared.
func Fabric(ctx context.Context, client *utils.HTTPClient, versions Versions, dir, game, loader string) error {
	serverJar := fabricDataDir + "/" + game + "-server.jar"
	if err := ensureVanilla(ctx, client, versions, dir, game, serverJar); err != nil {
		return err
	}

	if versionAtMost(loader, "0.12.5") {
		return nil
	}

	launchJar := filepath.Join(dir, filepath.FromSlash(
		fmt.Sprintf("%s/fabric-loader-server-%s-minecraft-%s.jar", fabricDataDir, loader, game)))
	if classPathComplete(launchJar) {
		return nil
	}

	profile, err := fetchProfile(ctx, client,
		profileURL(fabricMetaURL, "/v2/versions/loader/%s/%s/server/json", game, loader))
	if err != nil {
		return err
	}

	files := make([]File, 0, len(profile.Libraries))
	classPath := make([]string, 0, len(profile.Libraries))
	loaderJar := ""

	for _, lib := range profile.Libraries {
		libPath, err := MavenPath(lib.Name)
		if err != nil {
			return err
		}

		file := File{
			Path:   "libraries/" + libPath,
			URLs:   []string{strings.TrimRight(lib.URL, "/") + "/" + libPath},
			SHA512: lib.SHA512, SHA256: lib.SHA256, SHA1: lib.SHA1, MD5: lib.MD5,
		}
		if file.SHA512 == "" && file.SHA256 == "" && file.SHA1 == "" && file.MD5 == "" {
			file.Sidecar = "sha1"
		}
		files = append(files, file)
		classPath = append(classPath, "../../"+file.Path)

		if strings.HasPrefix(lib.Name, "net.fabricmc:fabric-loader:") {
			loaderJar = file.Path
		}
	}

	if err := Ensure(ctx, client, dir, "Fabric", files); err != nil {
		return err
	}

	mainClass := fabricDefaultLauncher
	if loaderJar != "" {
		if manifestClass := jarMainClass(filepath.Join(dir, filepath.FromSlash(loaderJar))); manifestClass != "" {
			mainClass = manifestClass
		}
	}

	return WriteLaunchJar(launchJar,
		[][2]string{{"Main-Class", mainClass}, {"Class-Path", strings.Join(classPath, " ")}},
		map[string]string{"fabric-server-launch.properties": "launch.mainClass=" + profile.MainClass + "\n"})
}

// Quilt installs a Quilt server the way `quilt-installer install server`
// does: the vanilla server as server.jar, the loader's libraries and a launch
// jar, named launcher here, that lists them
func Quilt(ctx context.Context, client *utils.HTTPClient, versions Versions, dir, game, loader, launcher string) error {
	if err := ensureVanilla(ctx, client, versions, dir, game, "server.jar"); err != nil {
		return err
	}

	profile, err := fetchProfile(ctx, client,
		profileURL(quiltMetaURL, "/v3/versions/loader/%s/%s/server/json", game, loader))
	if err != nil {
		return err
	}
	if profile.LauncherMainClass == "" {
		return fmt.Errorf("the Quilt profile for %s %s names no launcher", game, loader)
	}

	files := make([]File, 0, len(profile.Libraries))
	classPath := make([]string, 0, len(profile.Libraries))

	for _, lib := range profile.Libraries {
		// The installer ignores classifiers
		parts := strings.SplitN(lib.Name, ":", 4)
		if len(parts) < 3 {
			return fmt.Errorf("malformed library %q in the Quilt profile", lib.Name)
		}
		libPath, err := MavenPath(strings.Join(parts[:3], ":"))
		if err != nil {
			return err
		}

		files = append(files, File{
			Path:    "libraries/" + libPath,
			URLs:    []string{strings.TrimRight(lib.URL, "/") + "/" + libPath},
			SHA1:    lib.SHA1,
			Sidecar: "sha1",
		})
		classPath = append(classPath, "libraries/"+libPath)
	}

	if err := Ensure(ctx, client, dir, "Quilt", files); err != nil {
		return err
	}

	sort.Strings(classPath)

	launcherPath, err := Resolve(dir, launcher)
	if err != nil {
		return err
	}

	return WriteLaunchJar(launcherPath,
		[][2]string{{"Main-Class", profile.LauncherMainClass}, {"Class-Path", strings.Join(classPath, " ")}}, nil)
}

// ensureVanilla puts the vanilla server at rel unless a file is there; the
// launchers do not check it either
func ensureVanilla(ctx context.Context, client *utils.HTTPClient, versions Versions, dir, game, rel string) error {
	dest, err := Resolve(dir, rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dest); err == nil {
		return nil
	}

	vanilla, err := VanillaServer(ctx, versions, game, rel)
	if err != nil {
		return err
	}

	return Ensure(ctx, client, dir, "the vanilla server", []File{vanilla})
}

func jarMainClass(jarPath string) string {
	jar, err := OpenJar(jarPath)
	if err != nil {
		return ""
	}
	defer jar.Close()

	manifest, err := jar.Manifest()
	if err != nil {
		return ""
	}

	return manifest["Main-Class"]
}

// versionAtMost compares dotted numeric versions, ignoring anything after
// the numbers (0.12.5+build.1 is 0.12.5)
func versionAtMost(version, limit string) bool {
	a, b := versionNumbers(version), versionNumbers(limit)
	for i := range max(len(a), len(b)) {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x < y
		}
	}
	return true
}

func versionNumbers(version string) []int {
	version, _, _ = strings.Cut(version, "+")
	version, _, _ = strings.Cut(version, "-")

	var numbers []int
	for part := range strings.SplitSeq(version, ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			break
		}
		numbers = append(numbers, n)
	}
	return numbers
}
