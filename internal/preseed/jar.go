package preseed

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gameap/minecraft-runner/internal/utils"
)

// maxEntrySize bounds the metadata read from a jar
const maxEntrySize = 64 << 20

// Jar is an opened jar or zip
type Jar struct {
	reader *zip.ReadCloser
	files  map[string]*zip.File
}

// OpenJar opens the jar at path
func OpenJar(path string) (*Jar, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", path, err)
	}

	files := make(map[string]*zip.File, len(reader.File))
	for _, f := range reader.File {
		files[f.Name] = f
	}

	return &Jar{reader: reader, files: files}, nil
}

// Close closes the jar
func (j *Jar) Close() error {
	return j.reader.Close()
}

// Has tells whether the jar holds the file name
func (j *Jar) Has(name string) bool {
	_, ok := j.files[name]
	return ok
}

// Read returns the contents of the file name; ok is false when the jar does
// not hold it
func (j *Jar) Read(name string) (data []byte, ok bool, err error) {
	f, found := j.files[name]
	if !found {
		return nil, false, nil
	}

	rc, err := f.Open()
	if err != nil {
		return nil, true, fmt.Errorf("failed to read %s: %w", name, err)
	}
	defer rc.Close()

	data, err = io.ReadAll(io.LimitReader(rc, maxEntrySize+1))
	if err != nil {
		return nil, true, fmt.Errorf("failed to read %s: %w", name, err)
	}
	if len(data) > maxEntrySize {
		return nil, true, fmt.Errorf("%s is too large", name)
	}

	return data, true, nil
}

// Extract writes the file name to dest unless dest already holds it, checking
// it against its SHA-256
func (j *Jar) Extract(name, dest, sha256Sum string) error {
	if utils.VerifySHA256(dest, sha256Sum) == nil {
		return nil
	}

	f, found := j.files[name]
	if !found {
		return fmt.Errorf("%s is missing", name)
	}

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", name, err)
	}
	defer rc.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(dest), err)
	}

	part := dest + ".part"
	out, err := os.Create(part)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", dest, err)
	}
	defer os.Remove(part)

	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, hash), rc)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("failed to extract %s: %w", name, err)
	}

	if got := hex.EncodeToString(hash.Sum(nil)); !strings.EqualFold(got, sha256Sum) {
		return fmt.Errorf("%s has SHA-256 %s, want %s", name, got, sha256Sum)
	}

	if err := os.Rename(part, dest); err != nil {
		return fmt.Errorf("failed to write %s: %w", dest, err)
	}

	return nil
}

// lines splits text metadata into trimmed, non-empty lines
func lines(data []byte) []string {
	raw := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	result := make([]string, 0, len(raw))
	for _, line := range raw {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}

	return result
}

// Manifest reads the main attributes of the jar's META-INF/MANIFEST.MF
func (j *Jar) Manifest() (map[string]string, error) {
	data, ok, err := j.Read("META-INF/MANIFEST.MF")
	if err != nil || !ok {
		return nil, err
	}

	return parseManifest(data), nil
}

// parseManifest reads the main section of a manifest, joining continuation
// lines
func parseManifest(data []byte) map[string]string {
	attributes := make(map[string]string)

	var name, value string
	flush := func() {
		if name != "" {
			attributes[name] = value
		}
		name, value = "", ""
	}

	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case line == "":
			flush()
			return attributes
		case line[0] == ' ':
			value += line[1:]
		default:
			flush()
			if k, v, ok := strings.Cut(line, ": "); ok {
				name, value = k, v
			}
		}
	}
	flush()

	return attributes
}

// manifestLine writes one attribute the way java.util.jar.Manifest does: no
// line longer than 72 bytes, continued on lines that start with a space
func manifestLine(b *strings.Builder, name, value string) {
	line := name + ": " + value
	limit := 72
	for len(line) > limit {
		b.WriteString(line[:limit])
		b.WriteString("\r\n ")
		line = line[limit:]
		limit = 71
	}
	b.WriteString(line)
	b.WriteString("\r\n")
}

// WriteLaunchJar writes a jar that holds only a manifest with the given main
// attributes (Manifest-Version comes first) and the extra files. The jar is
// written next to path and renamed into place.
func WriteLaunchJar(path string, attributes [][2]string, extra map[string]string) error {
	var manifest strings.Builder
	manifestLine(&manifest, "Manifest-Version", "1.0")
	for _, attr := range attributes {
		manifestLine(&manifest, attr[0], attr[1])
	}
	manifest.WriteString("\r\n")

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(path), err)
	}

	tmp := path + ".tmp"
	if err := writeZip(tmp, append([][2]string{{"META-INF/MANIFEST.MF", manifest.String()}}, sortedEntries(extra)...)); err != nil {
		os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}

func sortedEntries(files map[string]string) [][2]string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	entries := make([][2]string, 0, len(names))
	for _, name := range names {
		entries = append(entries, [2]string{name, files[name]})
	}
	return entries
}

func writeZip(path string, entries [][2]string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", path, err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for _, entry := range entries {
		w, err := zw.Create(entry[0])
		if err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
		if _, err := io.WriteString(w, entry[1]); err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return f.Close()
}

// classPathComplete tells whether the jar at path exists and every Class-Path
// entry of its manifest is in place, which is how the Fabric server launcher
// decides that an install is complete
func classPathComplete(path string) bool {
	jar, err := OpenJar(path)
	if err != nil {
		return false
	}
	defer jar.Close()

	manifest, err := jar.Manifest()
	if err != nil || manifest["Main-Class"] == "" {
		return false
	}

	base := filepath.Dir(path)
	for _, entry := range strings.Fields(manifest["Class-Path"]) {
		if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(entry))); err != nil {
			return false
		}
	}

	return true
}
