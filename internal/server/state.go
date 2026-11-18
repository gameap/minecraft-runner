package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gameap/minecraft-runner/internal/providers"
)

const (
	stateFileName = ".mcrun-state.json"

	// installMarkerName is kept in the server directory while an installer
	// runs. A failed installer can leave files behind that look launchable,
	// such as Forge's argument file written before its processors failed.
	installMarkerName = ".mcrun-installing"
)

// State records the server mcrun last installed into a directory. It lets a
// server start while the provider's API is unreachable and tells which JAR a
// newer build has superseded.
type State struct {
	Mod         string                 `json:"mod"`
	Version     string                 `json:"version"`
	ModVersion  string                 `json:"mod_version,omitempty"`
	File        string                 `json:"file,omitempty"`
	Launch      providers.LaunchTarget `json:"launch"`
	ServerArgs  []string               `json:"server_args,omitempty"`
	InstalledAt time.Time              `json:"installed_at"`
}

// LoadState reads the state of a server directory; a directory without one yields nil
func LoadState(directory string) (*State, error) {
	data, err := os.ReadFile(filepath.Join(directory, stateFileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", stateFileName, err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", stateFileName, err)
	}

	return &state, nil
}

// Save writes the state through a temporary file, so that an interrupted write
// cannot leave a truncated state behind
func (s *State) Save(directory string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode %s: %w", stateFileName, err)
	}

	tmp, err := os.CreateTemp(directory, stateFileName+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary state file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write %s: %w", stateFileName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", stateFileName, err)
	}

	if err := os.Rename(tmpName, filepath.Join(directory, stateFileName)); err != nil {
		return fmt.Errorf("failed to move %s into place: %w", stateFileName, err)
	}

	return nil
}

// Matches reports whether the state satisfies a request. An empty version or
// mod version in the request means "whatever is current", which the installed
// server is the best available answer to when the API cannot be asked.
func (s *State) Matches(mod, version, modVersion string) bool {
	if s.Mod != mod {
		return false
	}
	if version != "" && version != s.Version {
		return false
	}
	if modVersion != "" && modVersion != s.ModVersion {
		return false
	}
	return true
}

// launchFilesExist checks that everything the launch target names is still on disk
func launchFilesExist(directory string, target providers.LaunchTarget) bool {
	if target.Jar == "" && len(target.ArgFiles) == 0 {
		return false
	}

	files := append([]string{}, target.ArgFiles...)
	if target.Jar != "" {
		files = append(files, target.Jar)
	}

	for _, file := range files {
		if _, err := os.Stat(filepath.Join(directory, filepath.FromSlash(file))); err != nil {
			return false
		}
	}

	return true
}

// installKey names one installation in the install marker, which holds a line
// for every installer run in the directory that did not finish
func installKey(provider, version, modVersion string) string {
	return strings.TrimSpace(provider + " " + version + " " + modVersion)
}

func unfinishedInstalls(directory string) []string {
	data, err := os.ReadFile(filepath.Join(directory, installMarkerName))
	if err != nil {
		return nil
	}

	var keys []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			keys = append(keys, line)
		}
	}

	return keys
}

// installUnfinished tells whether the installer run for key in directory did
// not finish
func installUnfinished(directory, key string) bool {
	return slices.Contains(unfinishedInstalls(directory), key)
}

// startInstall marks the installer run for key as not finished yet
func startInstall(directory, key string) error {
	keys := unfinishedInstalls(directory)
	if slices.Contains(keys, key) {
		return nil
	}

	data := strings.Join(append(keys, key), "\n") + "\n"
	if err := os.WriteFile(filepath.Join(directory, installMarkerName), []byte(data), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", installMarkerName, err)
	}
	return nil
}

// finishInstall clears the mark of the installer run for key. The marks of
// other installs that did not finish stay, so their leftovers are not taken
// for an installation.
func finishInstall(directory, key string) error {
	keys := unfinishedInstalls(directory)
	if !slices.Contains(keys, key) {
		return nil
	}

	path := filepath.Join(directory, installMarkerName)
	rest := slices.DeleteFunc(keys, func(k string) bool { return k == key })
	if len(rest) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove %s: %w", installMarkerName, err)
		}
		return nil
	}

	if err := os.WriteFile(path, []byte(strings.Join(rest, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", installMarkerName, err)
	}
	return nil
}
