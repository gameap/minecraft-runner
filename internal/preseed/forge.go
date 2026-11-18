package preseed

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gameap/minecraft-runner/internal/utils"
	"github.com/gameap/minecraft-runner/pkg/api/mojang"
)

const (
	installProfile     = "install_profile.json"
	taskDownloadMojmap = "DOWNLOAD_MOJMAPS"
)

// minecraftLibraries serves the libraries of legacy profiles that name no
// repository
var minecraftLibraries = "https://libraries.minecraft.net/"

// legacyForgeRepositories maps repositories of old install profiles onto
// where the files live now
var legacyForgeRepositories = map[string]string{
	"http://files.minecraftforge.net/maven/":  "https://maven.minecraftforge.net/",
	"https://files.minecraftforge.net/maven/": "https://maven.minecraftforge.net/",
	"http://maven.minecraftforge.net/":        "https://maven.minecraftforge.net/",
}

// ForgeInstall tells how to run a Forge or NeoForge installer once what it
// downloads is in place
type ForgeInstall struct {
	// Installer is the installer to run, relative to the server directory:
	// the original one or a repacked copy
	Installer string
	// Offline tells that the installer has nothing left to download and
	// understands --offline
	Offline bool

	dir      string
	repacked bool
}

// Remove deletes the repacked installer and its log
func (i *ForgeInstall) Remove() {
	if i == nil || !i.repacked {
		return
	}
	os.Remove(filepath.Join(i.dir, i.Installer))
	os.Remove(filepath.Join(i.dir, i.Installer+".log"))
}

type forgeProfile struct {
	Minecraft     string                       `json:"minecraft"`
	JSON          string                       `json:"json"`
	ServerJarPath string                       `json:"serverJarPath"`
	Libraries     []forgeLibrary               `json:"libraries"`
	Processors    []forgeProcessor             `json:"processors"`
	Data          map[string]map[string]string `json:"data"`

	// Installers 1.x
	Install *struct {
		Minecraft string `json:"minecraft"`
	} `json:"install"`
	VersionInfo *struct {
		Libraries []struct {
			Name      string   `json:"name"`
			URL       string   `json:"url"`
			Checksums []string `json:"checksums"`
			ServerReq bool     `json:"serverreq"`
		} `json:"libraries"`
	} `json:"versionInfo"`
}

type forgeLibrary struct {
	Name      string `json:"name"`
	Downloads struct {
		Artifact struct {
			URL  string `json:"url"`
			SHA1 string `json:"sha1"`
		} `json:"artifact"`
	} `json:"downloads"`
}

type forgeProcessor struct {
	Sides   []string          `json:"sides"`
	Args    []string          `json:"args"`
	Outputs map[string]string `json:"outputs"`
}

func (p forgeProcessor) forServer() bool {
	return len(p.Sides) == 0 || slices.Contains(p.Sides, "server")
}

// arg returns the value that follows the option name
func (p forgeProcessor) arg(name string) string {
	if i := slices.Index(p.Args, name); i != -1 && i+1 < len(p.Args) {
		return p.Args[i+1]
	}
	return ""
}

// forgeInstaller prepares one installer
type forgeInstaller struct {
	client   *utils.HTTPClient
	versions Versions
	dir      string
	name     string
	jar      *Jar
	profile  forgeProfile
	vanilla  map[string]*mojang.VersionDetail
}

// ForgeInstaller puts in place everything the Forge or NeoForge installer
// in dir would download: libraries, the vanilla server and Mojang's mappings.
// An installer that would download the mappings itself, with no checksum to
// skip it by, is repacked without that step. name is Forge or NeoForge.
func ForgeInstaller(ctx context.Context, client *utils.HTTPClient, versions Versions, dir, installer, name string) (*ForgeInstall, error) {
	installerPath, err := Resolve(dir, installer)
	if err != nil {
		return nil, err
	}

	jar, err := OpenJar(installerPath)
	if err != nil {
		return nil, err
	}
	defer jar.Close()

	raw, ok, err := jar.Read(installProfile)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%s has no install profile", installer)
	}

	f := &forgeInstaller{client: client, versions: versions, dir: dir, name: name, jar: jar,
		vanilla: make(map[string]*mojang.VersionDetail)}
	if err := json.Unmarshal(raw, &f.profile); err != nil {
		return nil, fmt.Errorf("failed to parse the install profile of %s: %w", installer, err)
	}

	install := &ForgeInstall{Installer: installer, Offline: f.understandsOffline(), dir: dir}

	if f.profile.VersionInfo != nil {
		return install, f.legacy(ctx)
	}

	replace, err := f.prepare(ctx, raw)
	if err != nil {
		return nil, err
	}
	if len(replace) == 0 || !install.Offline {
		return install, nil
	}

	repacked := strings.TrimSuffix(installer, ".jar") + "-offline.jar"
	repackedPath, err := Resolve(dir, repacked)
	if err != nil {
		return nil, err
	}
	if err := repackJar(installerPath, repackedPath, replace); err != nil {
		return nil, err
	}

	return &ForgeInstall{Installer: repacked, Offline: true, dir: dir, repacked: true}, nil
}

// understandsOffline tells whether the installer has the --offline option
func (f *forgeInstaller) understandsOffline() bool {
	data, _, _ := f.jar.Read("net/minecraftforge/installer/SimpleInstaller.class")
	return bytes.Contains(data, []byte("offline"))
}

// prepare handles installers 2.x and NeoForge's. It returns the profile
// files the repacked installer gets instead of the original ones.
func (f *forgeInstaller) prepare(ctx context.Context, rawProfile []byte) (map[string][]byte, error) {
	versionFile := strings.TrimPrefix(f.profile.JSON, "/")
	rawVersion, ok, err := f.jar.Read(versionFile)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("the installer has no %s", f.profile.JSON)
	}

	var version struct {
		Libraries []forgeLibrary `json:"libraries"`
	}
	if err := json.Unmarshal(rawVersion, &version); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", versionFile, err)
	}

	files, err := f.libraryFiles(append(slices.Clone(version.Libraries), f.profile.Libraries...))
	if err != nil {
		return nil, err
	}

	serverJar, err := f.serverJarPath()
	if err != nil {
		return nil, err
	}
	if exists, err := f.exists(serverJar); err != nil {
		return nil, err
	} else if !exists {
		server, err := f.vanillaFile(ctx, f.profile.Minecraft, serverJar)
		if err != nil {
			return nil, err
		}
		files = append(files, server)
	}

	if err := Ensure(ctx, f.client, f.dir, f.name, files); err != nil {
		return nil, err
	}

	removed := make(map[int]bool)
	for i, proc := range f.profile.Processors {
		if !proc.forServer() || proc.arg("--task") != taskDownloadMojmap {
			continue
		}
		skips, err := f.mojmaps(ctx, proc)
		if err != nil {
			return nil, err
		}
		if !skips {
			removed[i] = true
		}
	}

	profileSums, err := f.unknownChecksums(f.profile.Libraries)
	if err != nil {
		return nil, err
	}
	versionSums, err := f.unknownChecksums(version.Libraries)
	if err != nil {
		return nil, err
	}

	replace := make(map[string][]byte)
	if len(removed) > 0 || len(profileSums) > 0 {
		patched, err := patchProfile(rawProfile, removed, profileSums)
		if err != nil {
			return nil, fmt.Errorf("failed to patch the install profile: %w", err)
		}
		replace[installProfile] = patched
	}
	if len(versionSums) > 0 {
		patched, err := patchProfile(rawVersion, nil, versionSums)
		if err != nil {
			return nil, fmt.Errorf("failed to patch %s: %w", versionFile, err)
		}
		replace[versionFile] = patched
	}

	return replace, nil
}

// libraryFiles lists the libraries the installer downloads: the ones with a
// URL that it does not carry itself
func (f *forgeInstaller) libraryFiles(libraries []forgeLibrary) ([]File, error) {
	files := make([]File, 0, len(libraries))

	for _, lib := range libraries {
		artifact := lib.Downloads.Artifact
		if artifact.URL == "" {
			continue
		}

		libPath, err := MavenPath(lib.Name)
		if err != nil {
			return nil, err
		}
		if f.jar.Has("maven/" + libPath) {
			continue
		}

		file := File{Path: "libraries/" + libPath, URLs: []string{artifact.URL}, SHA1: artifact.SHA1}
		if file.SHA1 == "" {
			file.Sidecar = "sha1"
		}
		files = append(files, file)
	}

	return files, nil
}

// unknownChecksums returns the SHA-1 of the downloaded libraries the profile
// names none for, by their index. NeoForge downloads such a library again on
// every install, and fails offline.
func (f *forgeInstaller) unknownChecksums(libraries []forgeLibrary) (map[int]string, error) {
	sums := make(map[int]string)

	for i, lib := range libraries {
		if lib.Downloads.Artifact.URL == "" || lib.Downloads.Artifact.SHA1 != "" {
			continue
		}

		libPath, err := MavenPath(lib.Name)
		if err != nil {
			return nil, err
		}
		if f.jar.Has("maven/" + libPath) {
			continue
		}

		sum, err := utils.CalculateSHA1(filepath.Join(f.dir, "libraries", filepath.FromSlash(libPath)))
		if err != nil {
			return nil, err
		}
		sums[i] = sum
	}

	return sums, nil
}

// serverJarPath is where the installer looks for the vanilla server
func (f *forgeInstaller) serverJarPath() (string, error) {
	template := f.profile.ServerJarPath
	if template == "" {
		template = "{ROOT}/minecraft_server.{MINECRAFT_VERSION}.jar"
	}

	rel := strings.NewReplacer("{ROOT}", "", "{LIBRARY_DIR}", "/libraries", "{MINECRAFT_VERSION}", f.profile.Minecraft).
		Replace(template)
	if strings.ContainsAny(rel, "{}[]'") {
		return "", fmt.Errorf("unsupported server jar path %q", template)
	}

	return strings.TrimPrefix(rel, "/"), nil
}

func (f *forgeInstaller) exists(rel string) (bool, error) {
	return pathExists(f.dir, rel)
}

func (f *forgeInstaller) version(ctx context.Context, game string) (*mojang.VersionDetail, error) {
	if detail, ok := f.vanilla[game]; ok {
		return detail, nil
	}

	detail, err := f.versions.GetVersion(ctx, game)
	if err != nil {
		return nil, fmt.Errorf("failed to look up Minecraft %s: %w", game, err)
	}
	f.vanilla[game] = detail

	return detail, nil
}

func (f *forgeInstaller) vanillaFile(ctx context.Context, game, rel string) (File, error) {
	detail, err := f.version(ctx, game)
	if err != nil {
		return File{}, err
	}
	if detail.Downloads.Server.URL == "" {
		return File{}, fmt.Errorf("minecraft %s has no server download", game)
	}

	return File{Path: rel, URLs: mojangURLs(detail.Downloads.Server.URL), SHA1: detail.Downloads.Server.SHA1}, nil
}

// mojmaps puts in place the mappings a DOWNLOAD_MOJMAPS processor downloads,
// and tells whether the processor then skips itself: it does when it declares
// the checksum of its output and finds the file in place.
func (f *forgeInstaller) mojmaps(ctx context.Context, proc forgeProcessor) (bool, error) {
	output := proc.arg("--output")
	rel, err := f.dataPath(output)
	if err != nil {
		return false, err
	}

	want := ""
	if sum, ok := proc.Outputs[output]; ok {
		if want, err = f.dataLiteral(sum); err != nil {
			return false, err
		}
	}
	skips := want != ""

	dest, err := Resolve(f.dir, rel)
	if err != nil {
		return false, err
	}
	if skips && present(dest, File{SHA1: want}) {
		return true, nil
	}

	game := proc.arg("--version")
	if game == "" {
		game = f.profile.Minecraft
	}
	detail, err := f.version(ctx, game)
	if err != nil {
		return false, err
	}
	mappings := detail.Downloads.ServerMappings
	if mappings.URL == "" {
		return false, fmt.Errorf("minecraft %s publishes no server mappings", game)
	}

	if !slices.Contains(proc.Args, "--sanitize") {
		if skips && !strings.EqualFold(want, mappings.SHA1) {
			return false, fmt.Errorf("the installer expects mappings %s, Mojang publishes %s", want, mappings.SHA1)
		}
		file := File{Path: rel, URLs: mojangURLs(mappings.URL), SHA1: mappings.SHA1}
		return skips, Ensure(ctx, f.client, f.dir, "Mojang's mappings", []File{file})
	}

	tmp, err := os.MkdirTemp(f.dir, ".mojmaps-")
	if err != nil {
		return false, fmt.Errorf("failed to create a temporary directory: %w", err)
	}
	defer os.RemoveAll(tmp)

	file := File{Path: "server.txt", URLs: mojangURLs(mappings.URL), SHA1: mappings.SHA1}
	if err := Ensure(ctx, f.client, tmp, "Mojang's mappings", []File{file}); err != nil {
		return false, err
	}

	sanitized, err := f.sanitize(filepath.Join(tmp, "server.txt"), want)
	if err != nil {
		return false, err
	}

	return skips, writeFileAtomic(dest, sanitized)
}

// sanitize converts the mappings the way the processor would. Without a
// checksum to meet, nothing checks the result but the next processors, which
// read the mappings, so any SrgUtils ordering does.
func (f *forgeInstaller) sanitize(rawPath, want string) ([]byte, error) {
	raw, err := os.Open(rawPath)
	if err != nil {
		return nil, err
	}
	defer raw.Close()

	if want == "" {
		m, err := parseMappings(raw)
		if err != nil {
			return nil, err
		}
		return m.tsrg(srgUtilsLegacyOrder), nil
	}

	sanitized, ok, err := SanitizeMappings(raw, want)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("could not reproduce the sanitized mappings %s", want)
	}

	return sanitized, nil
}

// dataPath resolves a {KEY} argument naming a [maven:coordinate] in the
// profile data onto its place in libraries/
func (f *forgeInstaller) dataPath(arg string) (string, error) {
	value, err := f.dataValue(arg)
	if err != nil {
		return "", err
	}

	coord, ok := strings.CutPrefix(value, "[")
	if coord, ok = strings.CutSuffix(coord, "]"); !ok {
		return "", fmt.Errorf("unsupported processor argument %s = %q", arg, value)
	}

	libPath, err := MavenPath(coord)
	if err != nil {
		return "", err
	}

	return "libraries/" + libPath, nil
}

// dataLiteral resolves a {KEY} argument, or a 'literal', onto the literal
func (f *forgeInstaller) dataLiteral(arg string) (string, error) {
	value := arg
	if strings.HasPrefix(arg, "{") {
		var err error
		if value, err = f.dataValue(arg); err != nil {
			return "", err
		}
	}

	literal, ok := strings.CutPrefix(value, "'")
	if literal, ok = strings.CutSuffix(literal, "'"); !ok {
		return "", fmt.Errorf("unsupported processor argument %s = %q", arg, value)
	}

	return literal, nil
}

func (f *forgeInstaller) dataValue(arg string) (string, error) {
	key, ok := strings.CutPrefix(arg, "{")
	if key, ok = strings.CutSuffix(key, "}"); !ok {
		return "", fmt.Errorf("unsupported processor argument %q", arg)
	}

	value := f.profile.Data[key]["server"]
	if value == "" {
		return "", fmt.Errorf("the install profile has no server value for %s", key)
	}

	return value, nil
}

// legacy handles installers 1.x: the libraries the server requires and the
// vanilla server in the server directory
func (f *forgeInstaller) legacy(ctx context.Context) error {
	if f.profile.Install == nil || f.profile.Install.Minecraft == "" {
		return fmt.Errorf("the install profile names no Minecraft version")
	}

	files := make([]File, 0, len(f.profile.VersionInfo.Libraries)+1)

	for _, lib := range f.profile.VersionInfo.Libraries {
		if !lib.ServerReq {
			continue
		}

		libPath, err := MavenPath(lib.Name)
		if err != nil {
			return err
		}

		repo := lib.URL
		if repo == "" {
			repo = minecraftLibraries
		}
		if moved, ok := legacyForgeRepositories[repo]; ok {
			repo = moved
		}

		file := File{Path: "libraries/" + libPath, URLs: []string{repo + libPath}, AnySHA1: lib.Checksums}
		if len(file.AnySHA1) == 0 {
			file.Sidecar = "sha1"
		}
		files = append(files, file)
	}

	game := f.profile.Install.Minecraft
	serverJar := "minecraft_server." + game + ".jar"
	if exists, err := f.exists(serverJar); err != nil {
		return err
	} else if !exists {
		server, err := f.vanillaFile(ctx, game, serverJar)
		if err != nil {
			return err
		}
		files = append(files, server)
	}

	return Ensure(ctx, f.client, f.dir, f.name, files)
}

// patchProfile drops processors from an install profile and fills in library
// checksums, keeping everything else as it is
func patchProfile(raw []byte, removeProcessors map[int]bool, sha1s map[int]string) ([]byte, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}

	if len(removeProcessors) > 0 {
		var processors []json.RawMessage
		if err := json.Unmarshal(doc["processors"], &processors); err != nil {
			return nil, err
		}

		kept := make([]json.RawMessage, 0, len(processors))
		for i, proc := range processors {
			if !removeProcessors[i] {
				kept = append(kept, proc)
			}
		}

		encoded, err := json.Marshal(kept)
		if err != nil {
			return nil, err
		}
		doc["processors"] = encoded
	}

	if len(sha1s) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(doc["libraries"]))
		decoder.UseNumber()

		var libraries []map[string]any
		if err := decoder.Decode(&libraries); err != nil {
			return nil, err
		}

		for i, sum := range sha1s {
			if i >= len(libraries) {
				return nil, fmt.Errorf("no library %d", i)
			}
			downloads, _ := libraries[i]["downloads"].(map[string]any)
			artifact, _ := downloads["artifact"].(map[string]any)
			if artifact == nil {
				return nil, fmt.Errorf("library %d has no download", i)
			}
			artifact["sha1"] = sum
		}

		encoded, err := json.Marshal(libraries)
		if err != nil {
			return nil, err
		}
		doc["libraries"] = encoded
	}

	return json.Marshal(doc)
}

// repackJar copies the jar at src to dst with the entries in replace swapped
// and the signature dropped, which the changed entries would break
func repackJar(src, dst string, replace map[string][]byte) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", src, err)
	}
	defer reader.Close()

	tmp := dst + ".tmp"
	if err := writeRepacked(tmp, reader, replace); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to repack %s: %w", filepath.Base(src), err)
	}

	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to write %s: %w", dst, err)
	}

	return nil
}

func writeRepacked(path string, reader *zip.ReadCloser, replace map[string][]byte) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	for _, entry := range reader.File {
		if signatureFile(entry.Name) {
			continue
		}

		if data, ok := replace[entry.Name]; ok {
			w, err := zw.CreateHeader(&zip.FileHeader{Name: entry.Name, Method: zip.Deflate, Modified: entry.Modified})
			if err != nil {
				return err
			}
			if _, err := w.Write(data); err != nil {
				return err
			}
			continue
		}

		if err := copyRawEntry(zw, entry); err != nil {
			return err
		}
	}

	if err := zw.Close(); err != nil {
		return err
	}

	return out.Close()
}

func copyRawEntry(zw *zip.Writer, entry *zip.File) error {
	// A directory may carry an empty deflate stream, which zip.Writer refuses
	if strings.HasSuffix(entry.Name, "/") {
		_, err := zw.CreateHeader(&zip.FileHeader{Name: entry.Name, Method: zip.Store, Modified: entry.Modified})
		return err
	}

	header := entry.FileHeader
	w, err := zw.CreateRaw(&header)
	if err != nil {
		return err
	}

	r, err := entry.OpenRaw()
	if err != nil {
		return err
	}

	_, err = io.Copy(w, r)
	return err
}

// signatureFile tells whether name belongs to a jar signature
func signatureFile(name string) bool {
	dir, file := path.Split(strings.ToUpper(name))
	if dir != "META-INF/" {
		return false
	}

	switch path.Ext(file) {
	case ".SF", ".RSA", ".DSA", ".EC":
		return true
	}
	return strings.HasPrefix(file, "SIG-")
}

// writeFileAtomic writes data next to dest and renames it into place
func writeFileAtomic(dest string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(dest), err)
	}

	part := dest + ".part"
	if err := os.WriteFile(part, data, 0644); err != nil {
		os.Remove(part)
		return fmt.Errorf("failed to write %s: %w", dest, err)
	}

	if err := os.Rename(part, dest); err != nil {
		os.Remove(part)
		return fmt.Errorf("failed to write %s: %w", dest, err)
	}

	return nil
}
