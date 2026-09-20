package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

// Config represents the application configuration
type Config struct {
	Defaults DefaultsConfig `yaml:"defaults"`
	Java     JavaConfig     `yaml:"java"`
	Server   ServerConfig   `yaml:"server"`
	Cache    CacheConfig    `yaml:"cache"`
}

// DefaultsConfig contains default values for server creation
type DefaultsConfig struct {
	Version    string `yaml:"version"`
	Mod        string `yaml:"mod"`
	ModVersion string `yaml:"mod_version"`
	AcceptEULA bool   `yaml:"accept_eula"`
	Memory     string `yaml:"memory"`
	MinMemory  string `yaml:"min_memory"`
}

// JavaConfig contains Java-related settings
type JavaConfig struct {
	PreferBundled bool           `yaml:"prefer_bundled"`
	AutoInstall   bool           `yaml:"auto_install"`
	Paths         map[int]string `yaml:"paths"`

	// Version and Path come from the per-server config and act as the
	// defaults of the --java and --java-path flags
	Version int    `yaml:"-"`
	Path    string `yaml:"-"`
}

// ServerConfig contains server-related settings
type ServerConfig struct {
	JVMArgs    []string          `yaml:"jvm_args"`
	Properties map[string]string `yaml:"properties"`

	// Network comes from the per-server config and acts as the defaults of
	// the --ip, --port, --query-port and --rcon-* flags
	Network ServerLocalValues `yaml:"-"`
}

// CacheConfig contains cache settings
type CacheConfig struct {
	Directory  string `yaml:"directory"`
	MaxAgeDays int    `yaml:"max_age_days"`
}

// ServerLocalConfig represents per-server configuration (.mcrun.yaml)
type ServerLocalConfig struct {
	Version    string            `yaml:"version"`
	Mod        string            `yaml:"mod"`
	ModVersion string            `yaml:"mod_version"`
	Java       JavaLocalConfig   `yaml:"java"`
	Server     ServerLocalValues `yaml:"server"`
}

// JavaLocalConfig contains per-server Java settings
type JavaLocalConfig struct {
	Version int      `yaml:"version"`
	Memory  string   `yaml:"memory"`
	MinMem  string   `yaml:"min_memory"`
	Args    []string `yaml:"args"`
	Path    string   `yaml:"path"`
}

// ServerLocalValues contains per-server values
type ServerLocalValues struct {
	IP           string `yaml:"ip"`
	Port         int    `yaml:"port"`
	QueryPort    int    `yaml:"query_port"`
	RconPort     int    `yaml:"rcon_port"`
	RconPassword string `yaml:"rcon_password"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		Defaults: DefaultsConfig{
			Mod:        "vanilla",
			AcceptEULA: false,
			Memory:     "2G",
			MinMemory:  "1G",
		},
		Java: JavaConfig{
			PreferBundled: false,
			AutoInstall:   true,
			Paths:         make(map[int]string),
		},
		Server: ServerConfig{
			JVMArgs: []string{
				"-XX:+UseG1GC",
				"-XX:+ParallelRefProcEnabled",
				"-XX:MaxGCPauseMillis=200",
				"-XX:+UnlockExperimentalVMOptions",
				"-XX:+DisableExplicitGC",
				"-XX:+AlwaysPreTouch",
			},
			Properties: make(map[string]string),
		},
		Cache: CacheConfig{
			Directory:  filepath.Join(homeDir, ".mcrun", "cache"),
			MaxAgeDays: 30,
		},
	}
}

// Load loads configuration from file
func Load(configPath string, serverDir string) (*Config, error) {
	cfg := DefaultConfig()

	// Try to load global config
	globalPath := configPath
	if globalPath == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			globalPath = filepath.Join(homeDir, ".mcrun", "config.yaml")
		}
	}

	if globalPath != "" {
		if err := loadFromFile(globalPath, cfg); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load %s: %w", globalPath, err)
		}
	}

	// Try to load local server config
	if serverDir != "" {
		localPath := filepath.Join(serverDir, ".mcrun.yaml")
		localCfg := &ServerLocalConfig{}
		err := loadFromFile(localPath, localCfg)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load %s: %w", localPath, err)
		}
		if err == nil {
			mergeLocalConfig(cfg, localCfg)
		}
	}

	return cfg, nil
}

// loadFromFile loads YAML from a file into the target struct
func loadFromFile(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, target)
}

// mergeLocalConfig merges ServerLocalConfig into Config
func mergeLocalConfig(cfg *Config, local *ServerLocalConfig) {
	if local.Version != "" {
		cfg.Defaults.Version = local.Version
	}
	if local.Mod != "" {
		cfg.Defaults.Mod = local.Mod
	}
	if local.ModVersion != "" {
		cfg.Defaults.ModVersion = local.ModVersion
	}
	if local.Java.Memory != "" {
		cfg.Defaults.Memory = local.Java.Memory
	}
	if local.Java.MinMem != "" {
		cfg.Defaults.MinMemory = local.Java.MinMem
	}

	// A path next to a version pins the binary of that Java version; a path on
	// its own is the binary to run the server with
	switch {
	case local.Java.Version != 0 && local.Java.Path != "":
		if cfg.Java.Paths == nil {
			cfg.Java.Paths = make(map[int]string)
		}
		cfg.Java.Paths[local.Java.Version] = local.Java.Path
		cfg.Java.Version = local.Java.Version
	case local.Java.Version != 0:
		cfg.Java.Version = local.Java.Version
	case local.Java.Path != "":
		cfg.Java.Path = local.Java.Path
	}

	if len(local.Java.Args) > 0 {
		cfg.Server.JVMArgs = append(cfg.Server.JVMArgs, local.Java.Args...)
	}

	cfg.Server.Network = local.Server
}

// Save saves the config to a file
func (c *Config) Save(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetConfigDir returns the mcrun config directory
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".mcrun"), nil
}

// GetCacheDir returns the cache directory, creating it if needed
func (c *Config) GetCacheDir() (string, error) {
	if err := os.MkdirAll(c.Cache.Directory, 0755); err != nil {
		return "", err
	}
	return c.Cache.Directory, nil
}
