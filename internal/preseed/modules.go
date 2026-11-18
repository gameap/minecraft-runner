package preseed

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/gameap/minecraft-runner/internal/utils"
)

// ProxyModules puts into modules/ the modules BungeeCord and Waterfall would
// download on start. Once modules.yml exists only the modules it lists are
// prepared, so a module the owner removed is not brought back.
func ProxyModules(ctx context.Context, client *utils.HTTPClient, dir string, modules []File) error {
	listed, err := listedModules(filepath.Join(dir, "modules.yml"))
	if err != nil {
		return err
	}

	files := make([]File, 0, len(modules))
	for _, m := range modules {
		name := strings.TrimSuffix(path.Base(m.Path), ".jar")
		if listed == nil || listed[name] {
			files = append(files, m)
		}
	}

	return Ensure(ctx, client, dir, "the proxy modules", files)
}

// listedModules reads the jenkins:// modules of modules.yml; nil means the file
// is not there yet
func listedModules(file string) (map[string]bool, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", file, err)
	}

	var config struct {
		Modules []string `yaml:"modules"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", file, err)
	}

	listed := make(map[string]bool, len(config.Modules))
	for _, module := range config.Modules {
		if name, ok := strings.CutPrefix(strings.TrimSpace(module), "jenkins://"); ok {
			listed[name] = true
		}
	}

	return listed, nil
}
