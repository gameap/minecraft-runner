package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gameap/minecraft-runner/internal/providers"
)

const stateFileName = ".mcrun-state.json"

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
