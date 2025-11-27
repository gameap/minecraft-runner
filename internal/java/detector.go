package java

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// Installation represents a detected Java installation
type Installation struct {
	Path        string // Full path to java binary
	Version     int    // Major version (8, 11, 17, 21)
	FullVersion string // e.g., "21.0.9+10-LTS"
	Vendor      string // e.g., "Eclipse Adoptium"
	IsSystem    bool   // System-wide vs user/bundled
	Arch        string // x64, aarch64
}

// Detector finds Java installations on the system
type Detector struct{}

// NewDetector creates a new Java detector
func NewDetector() *Detector {
	return &Detector{}
}

// DetectAll finds all Java installations on the system
func (d *Detector) DetectAll() ([]Installation, error) {
	var installations []Installation
	seen := make(map[string]bool)

	// Check common locations
	searchPaths := d.getSearchPaths()

	for _, searchPath := range searchPaths {
		javas := d.findJavaBinaries(searchPath)
		for _, javaBin := range javas {
			if seen[javaBin] {
				continue
			}
			seen[javaBin] = true

			inst, err := d.getInstallationInfo(javaBin)
			if err != nil {
				continue
			}
			inst.IsSystem = d.isSystemPath(searchPath)
			installations = append(installations, *inst)
		}
	}

	// Also check PATH
	if pathJava, err := exec.LookPath("java"); err == nil {
		realPath, err := filepath.EvalSymlinks(pathJava)
		if err == nil {
			pathJava = realPath
		}
		if !seen[pathJava] {
			if inst, err := d.getInstallationInfo(pathJava); err == nil {
				inst.IsSystem = true
				installations = append(installations, *inst)
			}
		}
	}

	return installations, nil
}

// FindForVersion finds a Java installation suitable for a Minecraft version
func (d *Detector) FindForVersion(mcVersion string) (*Installation, error) {
	required := GetRequiredJavaVersion(mcVersion)
	installations, err := d.DetectAll()
	if err != nil {
		return nil, err
	}

	// First try to find exact match
	for _, inst := range installations {
		if inst.Version == required {
			return &inst, nil
		}
	}

	// Then find any compatible version (higher is ok)
	var best *Installation
	for i := range installations {
		inst := &installations[i]
		if inst.Version >= required {
			if best == nil || inst.Version < best.Version {
				best = inst
			}
		}
	}

	if best != nil {
		return best, nil
	}

	return nil, fmt.Errorf("no compatible Java found (need Java %d or higher)", required)
}

// getSearchPaths returns paths to search for Java installations
func (d *Detector) getSearchPaths() []string {
	home, _ := os.UserHomeDir()

	if runtime.GOOS == "windows" {
		paths := []string{
			`C:\Program Files\Java`,
			`C:\Program Files (x86)\Java`,
			`C:\Program Files\Eclipse Adoptium`,
			`C:\Program Files\Microsoft`,
			`C:\Program Files\Zulu`,
		}

		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, "Programs", "Eclipse Adoptium"))
		}

		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".mcrun", "java"),
				filepath.Join(home, "scoop", "apps", "openjdk"),
			)
		}

		return paths
	}

	// Linux/macOS
	paths := []string{
		"/usr/lib/jvm",
		"/usr/java",
		"/opt/java",
		"/opt/jdk",
	}

	if runtime.GOOS == "darwin" {
		paths = append(paths,
			"/Library/Java/JavaVirtualMachines",
			"/System/Library/Java/JavaVirtualMachines",
		)
		if home != "" {
			paths = append(paths, filepath.Join(home, "Library", "Java", "JavaVirtualMachines"))
		}
	}

	if home != "" {
		paths = append(paths,
			filepath.Join(home, ".sdkman", "candidates", "java"),
			filepath.Join(home, ".mcrun", "java"),
			filepath.Join(home, ".jdks"),
		)
	}

	return paths
}

// findJavaBinaries finds Java binaries in a directory
func (d *Detector) findJavaBinaries(searchPath string) []string {
	var binaries []string

	if _, err := os.Stat(searchPath); os.IsNotExist(err) {
		return binaries
	}

	javaExe := "java"
	if runtime.GOOS == "windows" {
		javaExe = "java.exe"
	}

	// Walk the directory looking for java binaries
	filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Limit depth to avoid too deep traversal
		relPath, _ := filepath.Rel(searchPath, path)
		if strings.Count(relPath, string(filepath.Separator)) > 4 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if info.Name() == javaExe {
			// Verify it's in a bin directory
			if filepath.Base(filepath.Dir(path)) == "bin" {
				binaries = append(binaries, path)
			}
		}

		return nil
	})

	return binaries
}

// getInstallationInfo gets version info for a Java binary
func (d *Detector) getInstallationInfo(javaBin string) (*Installation, error) {
	cmd := exec.Command(javaBin, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run java -version: %w", err)
	}

	outputStr := string(output)

	// Parse version
	version, fullVersion := parseJavaVersion(outputStr)
	if version == 0 {
		return nil, fmt.Errorf("could not parse Java version from output")
	}

	// Parse vendor
	vendor := parseJavaVendor(outputStr)

	// Detect architecture
	arch := runtime.GOARCH
	if strings.Contains(strings.ToLower(outputStr), "64-bit") || strings.Contains(outputStr, "amd64") {
		arch = "x64"
	} else if strings.Contains(outputStr, "aarch64") {
		arch = "aarch64"
	}

	return &Installation{
		Path:        javaBin,
		Version:     version,
		FullVersion: fullVersion,
		Vendor:      vendor,
		Arch:        arch,
	}, nil
}

// parseJavaVersion parses the Java version from -version output
func parseJavaVersion(output string) (int, string) {
	// Match patterns like:
	// openjdk version "21.0.1" 2023-10-17
	// java version "1.8.0_311"
	// openjdk version "17.0.9" 2023-10-17

	re := regexp.MustCompile(`(?:openjdk|java) version "([^"]+)"`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return 0, ""
	}

	fullVersion := matches[1]

	// Parse major version
	// For versions like 1.8.x, the major version is 8
	// For versions like 17.x, the major version is 17
	parts := strings.Split(fullVersion, ".")
	if len(parts) == 0 {
		return 0, fullVersion
	}

	firstPart, err := strconv.Atoi(strings.TrimSuffix(parts[0], "-ea"))
	if err != nil {
		return 0, fullVersion
	}

	if firstPart == 1 && len(parts) > 1 {
		// Old versioning scheme (1.8, 1.7, etc.)
		secondPart, err := strconv.Atoi(parts[1])
		if err == nil {
			return secondPart, fullVersion
		}
	}

	return firstPart, fullVersion
}

// parseJavaVendor extracts the vendor from -version output
func parseJavaVendor(output string) string {
	outputLower := strings.ToLower(output)

	vendors := map[string]string{
		"temurin":   "Eclipse Adoptium",
		"adoptium":  "Eclipse Adoptium",
		"openjdk":   "OpenJDK",
		"oracle":    "Oracle",
		"zulu":      "Azul Zulu",
		"corretto":  "Amazon Corretto",
		"microsoft": "Microsoft",
		"bellsoft":  "BellSoft Liberica",
		"graalvm":   "GraalVM",
	}

	for key, name := range vendors {
		if strings.Contains(outputLower, key) {
			return name
		}
	}

	return "Unknown"
}

// isSystemPath checks if a path is a system-wide location
func (d *Detector) isSystemPath(path string) bool {
	if runtime.GOOS == "windows" {
		return strings.HasPrefix(strings.ToLower(path), `c:\program files`)
	}

	systemPaths := []string{"/usr", "/opt", "/Library/Java", "/System/Library/Java"}
	for _, sp := range systemPaths {
		if strings.HasPrefix(path, sp) {
			return true
		}
	}

	return false
}
