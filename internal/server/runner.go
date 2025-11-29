package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gameap/minecraft-runner/internal/config"
	"github.com/gameap/minecraft-runner/internal/java"
	"github.com/gameap/minecraft-runner/internal/providers"
)

// RunOptions contains options for running a server
type RunOptions struct {
	Version      string
	Mod          string
	ModVersion   string
	Directory    string
	IP           string
	Port         int
	QueryPort    int
	RconPort     int
	RconPassword string
	Memory       string
	MinMemory    string
	JVMArgs      []string
	AcceptEULA   bool
	JavaPath     string
	JavaVersion  int
}

// Runner handles server execution
type Runner struct {
	downloader  *Downloader
	javaManager *java.Manager
	registry    *providers.Registry
	config      *config.Config
}

// NewRunner creates a new server runner
func NewRunner(registry *providers.Registry, javaManager *java.Manager, cfg *config.Config) *Runner {
	return &Runner{
		downloader:  NewDownloader(registry),
		javaManager: javaManager,
		registry:    registry,
		config:      cfg,
	}
}

// Run downloads (if needed) and runs a Minecraft server
func (r *Runner) Run(ctx context.Context, opts RunOptions) error {
	// Get provider
	provider, err := r.registry.Get(opts.Mod)
	if err != nil {
		return fmt.Errorf("unknown server type '%s': %w", opts.Mod, err)
	}

	// Download server if needed
	jar, err := r.downloader.Download(ctx, DownloadOptions{
		Version:    opts.Version,
		Mod:        opts.Mod,
		ModVersion: opts.ModVersion,
		Directory:  opts.Directory,
	})
	if err != nil {
		return fmt.Errorf("failed to download server: %w", err)
	}

	// Update version from downloaded JAR
	if opts.Version == "" {
		opts.Version = jar.Version
	}

	// Find Java
	javaInst, err := r.findJava(ctx, opts, provider)
	if err != nil {
		return fmt.Errorf("failed to find Java: %w", err)
	}

	// Run post-download steps (e.g., Forge installer)
	if jar.RequiresInstall {
		jarPath := filepath.Join(opts.Directory, jar.Filename)
		if err := provider.PostDownload(ctx, jarPath, javaInst.Path); err != nil {
			return fmt.Errorf("post-download failed: %w", err)
		}
	}

	// Create EULA if requested
	if opts.AcceptEULA || (r.config != nil && r.config.Defaults.AcceptEULA) {
		if err := config.CreateEULA(opts.Directory); err != nil {
			return fmt.Errorf("failed to create EULA: %w", err)
		}
	}

	// Create default server.properties if it doesn't exist
	if err := config.CreateDefaultServerProperties(opts.Directory); err != nil {
		return fmt.Errorf("failed to create server.properties: %w", err)
	}

	// Update server.properties
	if err := r.updateServerProperties(opts); err != nil {
		return fmt.Errorf("failed to update server.properties: %w", err)
	}

	// Find the server JAR to run
	serverJar, err := FindServerJar(opts.Directory, opts.Mod, opts.Version)
	if err != nil {
		return fmt.Errorf("failed to find server JAR: %w", err)
	}

	// Build and run command
	args := r.buildJVMArgs(opts, serverJar)

	fmt.Printf("\nStarting Minecraft server %s (%s)...\n", opts.Version, opts.Mod)
	fmt.Printf("Java: %s (version %d)\n", javaInst.Path, javaInst.Version)
	fmt.Printf("JAR: %s\n\n", filepath.Base(serverJar))

	cmd := exec.CommandContext(ctx, javaInst.Path, args...)
	cmd.Dir = opts.Directory
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// findJava finds an appropriate Java installation
func (r *Runner) findJava(ctx context.Context, opts RunOptions, provider providers.Provider) (*java.Installation, error) {
	// Use custom path if specified
	if opts.JavaPath != "" {
		return r.javaManager.GetByPath(opts.JavaPath)
	}

	// Use specific version if specified
	if opts.JavaVersion != 0 {
		return r.javaManager.GetByVersion(ctx, opts.JavaVersion, true)
	}

	// Get recommended version for this Minecraft version
	return r.javaManager.GetForMinecraftVersion(ctx, opts.Version, true)
}

// buildJVMArgs constructs JVM arguments for running the server
func (r *Runner) buildJVMArgs(opts RunOptions, jarPath string) []string {
	var args []string

	// Memory settings
	memory := opts.Memory
	if memory == "" && r.config != nil {
		memory = r.config.Defaults.Memory
	}
	if memory == "" {
		memory = "1G"
	}

	minMemory := opts.MinMemory
	if minMemory == "" && r.config != nil {
		minMemory = r.config.Defaults.MinMemory
	}
	if minMemory == "" {
		minMemory = "1G"
	}

	args = append(args, fmt.Sprintf("-Xmx%s", memory))
	args = append(args, fmt.Sprintf("-Xms%s", minMemory))

	// Default optimization flags
	defaultArgs := []string{
		"-XX:+UseG1GC",
		"-XX:+ParallelRefProcEnabled",
		"-XX:MaxGCPauseMillis=200",
		"-XX:+UnlockExperimentalVMOptions",
		"-XX:+DisableExplicitGC",
		"-XX:+AlwaysPreTouch",
		"-XX:G1NewSizePercent=30",
		"-XX:G1MaxNewSizePercent=40",
		"-XX:G1HeapRegionSize=8M",
		"-XX:G1ReservePercent=20",
		"-XX:G1HeapWastePercent=5",
		"-XX:G1MixedGCCountTarget=4",
		"-XX:InitiatingHeapOccupancyPercent=15",
		"-XX:G1MixedGCLiveThresholdPercent=90",
		"-XX:G1RSetUpdatingPauseTimePercent=5",
		"-XX:SurvivorRatio=32",
		"-XX:+PerfDisableSharedMem",
		"-XX:MaxTenuringThreshold=1",
		"-Dusing.aikars.flags=https://mcflags.emc.gs",
		"-Daikars.new.flags=true",
	}

	// Add config JVM args or defaults
	if r.config != nil && len(r.config.Server.JVMArgs) > 0 {
		args = append(args, r.config.Server.JVMArgs...)
	} else {
		args = append(args, defaultArgs...)
	}

	// Add custom JVM args
	args = append(args, opts.JVMArgs...)

	// JAR and nogui
	args = append(args, "-jar", jarPath, "--nogui")

	return args
}

// updateServerProperties updates server.properties with provided options
func (r *Runner) updateServerProperties(opts RunOptions) error {
	props, err := config.LoadServerProperties(opts.Directory)
	if err != nil {
		return err
	}

	// Only update if values are provided
	if opts.IP != "" {
		props.SetIP(opts.IP)
	}

	if opts.Port > 0 {
		props.SetPort(opts.Port)
	}

	if opts.QueryPort > 0 {
		props.SetQueryPort(opts.QueryPort)
	}

	if opts.RconPort > 0 && opts.RconPassword != "" {
		props.SetRcon(opts.RconPort, opts.RconPassword)
	}

	// Apply config defaults
	if r.config != nil {
		for key, value := range r.config.Server.Properties {
			if props.Get(key) == "" {
				props.Set(key, value)
			}
		}
	}

	// Only save if we have a properties file to update
	if _, err := os.Stat(filepath.Join(opts.Directory, "server.properties")); err == nil {
		return props.Save()
	}

	return nil
}

// GetDefaultJVMArgs returns the default JVM arguments as a string
func GetDefaultJVMArgs() string {
	args := []string{
		"-XX:+UseG1GC",
		"-XX:+ParallelRefProcEnabled",
		"-XX:MaxGCPauseMillis=200",
	}
	return strings.Join(args, " ")
}
