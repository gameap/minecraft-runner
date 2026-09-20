package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gameap/minecraft-runner/internal/config"
	"github.com/gameap/minecraft-runner/internal/java"
	"github.com/gameap/minecraft-runner/internal/providers"
	"github.com/gameap/minecraft-runner/internal/utils"
)

const (
	// stopTimeout is how long a server gets to save its world and exit after a
	// stop request before it is killed; it matches systemd's default TimeoutStopSec
	stopTimeout = 90 * time.Second

	// userJVMArgsFile is where Forge and NeoForge server packs keep their JVM arguments
	userJVMArgsFile = "user_jvm_args.txt"

	defaultMemory = "1G"
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

// ExitError reports that the server process ended with a non-zero exit code
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("server exited with code %d", e.Code)
}

// Runner handles server execution
type Runner struct {
	downloader  *Downloader
	javaManager *java.Manager
	registry    *providers.Registry
	config      *config.Config
}

// installation is a server that is ready to be started
type installation struct {
	version    string
	launch     providers.LaunchTarget
	serverArgs []string
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

// Run downloads and installs a Minecraft server if needed, then runs it
func (r *Runner) Run(ctx context.Context, opts RunOptions) error {
	provider, err := r.registry.Get(opts.Mod)
	if err != nil {
		return fmt.Errorf("unknown server type '%s': %w", opts.Mod, err)
	}

	if err := os.MkdirAll(opts.Directory, 0755); err != nil {
		return fmt.Errorf("failed to create server directory: %w", err)
	}

	inst, javaInst, err := r.prepare(ctx, provider, opts)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}

	if !providers.IsProxy(provider) {
		if opts.AcceptEULA || (r.config != nil && r.config.Defaults.AcceptEULA) {
			if err := config.CreateEULA(opts.Directory); err != nil {
				return fmt.Errorf("failed to create EULA: %w", err)
			}
		}

		if err := config.CreateDefaultServerProperties(opts.Directory); err != nil {
			return fmt.Errorf("failed to create server.properties: %w", err)
		}

		if err := r.updateServerProperties(opts); err != nil {
			return fmt.Errorf("failed to update server.properties: %w", err)
		}
	}

	args := r.buildArgs(provider, opts, inst, stdinIsTerminal())

	fmt.Printf("\nStarting Minecraft server %s (%s)...\n", inst.version, opts.Mod)
	fmt.Printf("Java: %s (version %d)\n", javaInst.Path, javaInst.Version)
	fmt.Printf("Launch: %s\n\n", describeLaunch(inst.launch))

	return r.start(ctx, javaInst.Path, opts.Directory, args)
}

// prepare makes sure the requested server is installed and picks its Java
func (r *Runner) prepare(
	ctx context.Context, provider providers.Provider, opts RunOptions,
) (*installation, *java.Installation, error) {
	jar, resolveErr := r.downloader.Resolve(ctx, DownloadOptions{
		Version:    opts.Version,
		Mod:        opts.Mod,
		ModVersion: opts.ModVersion,
		Directory:  opts.Directory,
	})
	if resolveErr != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}

		inst, err := r.installedFallback(opts, resolveErr)
		if err != nil {
			return nil, nil, err
		}

		javaInst, err := r.findJava(ctx, opts, provider, inst.version)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find Java: %w", err)
		}

		return inst, javaInst, nil
	}

	javaInst, err := r.findJava(ctx, opts, provider, jar.Version)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find Java: %w", err)
	}

	launch, err := r.install(ctx, provider, jar, javaInst.Path, opts.Directory)
	if err != nil {
		return nil, nil, err
	}

	inst := &installation{
		version:    jar.Version,
		launch:     *launch,
		serverArgs: jar.ServerArgs,
	}

	r.recordInstall(opts, jar, inst)

	return inst, javaInst, nil
}

// install downloads and installs the server unless that exact version is already in place
func (r *Runner) install(
	ctx context.Context, provider providers.Provider, jar *providers.ServerJar, javaPath, directory string,
) (*providers.LaunchTarget, error) {
	if target := r.resolveLaunch(provider, directory, jar); target != nil {
		return target, nil
	}

	if err := r.downloader.Fetch(ctx, directory, jar, false); err != nil {
		return nil, fmt.Errorf("failed to download server: %w", err)
	}

	if jar.RequiresInstall {
		if err := provider.PostDownload(ctx, directory, jar, javaPath); err != nil {
			return nil, fmt.Errorf("post-download failed: %w", err)
		}
	}

	target := r.resolveLaunch(provider, directory, jar)
	if target == nil {
		return nil, fmt.Errorf("%s %s was installed, but nothing to launch was found in %s",
			provider.Name(), jar.Version, directory)
	}

	return target, nil
}

func (r *Runner) resolveLaunch(
	provider providers.Provider, directory string, jar *providers.ServerJar,
) *providers.LaunchTarget {
	if resolver, ok := provider.(providers.LaunchResolver); ok {
		return resolver.ResolveLaunch(directory, jar)
	}

	if jar.RequiresInstall || !r.downloader.IsDownloaded(directory, jar) {
		return nil
	}

	return &providers.LaunchTarget{Jar: jar.Filename}
}

// installedFallback starts the server a previous run installed when the
// provider's API cannot tell which file serves the request. Servers that are
// already on disk must not go down together with a download API.
func (r *Runner) installedFallback(opts RunOptions, cause error) (*installation, error) {
	failure := fmt.Errorf("failed to resolve server: %w", cause)

	state, err := LoadState(opts.Directory)
	if err != nil || state == nil {
		return nil, failure
	}

	if !state.Matches(opts.Mod, opts.Version, opts.ModVersion) || !launchFilesExist(opts.Directory, state.Launch) {
		return nil, failure
	}

	fmt.Printf("[WARNING] Could not resolve the %s server to run: %v\n", opts.Mod, cause)
	fmt.Printf("  Starting the installed %s %s instead\n\n", state.Mod, state.Version)

	return &installation{
		version:    state.Version,
		launch:     state.Launch,
		serverArgs: state.ServerArgs,
	}, nil
}

// recordInstall remembers what was installed and drops the JAR a newer build replaced
func (r *Runner) recordInstall(opts RunOptions, jar *providers.ServerJar, inst *installation) {
	previous, _ := LoadState(opts.Directory)

	state := &State{
		Mod:         opts.Mod,
		Version:     jar.Version,
		ModVersion:  jar.ModVersion,
		File:        jar.Filename,
		Launch:      inst.launch,
		ServerArgs:  inst.serverArgs,
		InstalledAt: time.Now().UTC(),
	}

	if previous != nil && previous.Matches(state.Mod, state.Version, state.ModVersion) &&
		reflect.DeepEqual(previous.Launch, state.Launch) {
		return
	}

	if err := state.Save(opts.Directory); err != nil {
		fmt.Printf("[WARNING] %v\n", err)
		return
	}

	removeSuperseded(opts.Directory, previous, state)
}

// removeSuperseded deletes the plain server JAR of the previous build of the
// same server. Installer-based servers share their libraries between versions
// and are left alone.
func removeSuperseded(directory string, previous, current *State) {
	if previous == nil || previous.Mod != current.Mod {
		return
	}
	if previous.File == "" || previous.Launch.Jar != previous.File || current.Launch.Jar == "" {
		return
	}
	if previous.File == current.File || previous.File == current.Launch.Jar {
		return
	}

	if err := os.Remove(filepath.Join(directory, previous.File)); err == nil {
		fmt.Printf("Removed superseded %s\n", previous.File)
	}
}

// findJava finds an appropriate Java installation
func (r *Runner) findJava(
	ctx context.Context, opts RunOptions, provider providers.Provider, mcVersion string,
) (*java.Installation, error) {
	if opts.JavaPath != "" {
		// The server runs from its own directory, which a relative path would then be resolved against
		javaPath, err := filepath.Abs(opts.JavaPath)
		if err != nil {
			return nil, fmt.Errorf("invalid Java path %q: %w", opts.JavaPath, err)
		}
		return r.javaManager.GetByPath(javaPath)
	}

	if opts.JavaVersion != 0 {
		return r.javaManager.GetByVersion(ctx, opts.JavaVersion, true)
	}

	recommended := provider.GetRecommendedJavaVersion(ctx, mcVersion)
	return r.javaManager.GetByVersion(ctx, recommended, true)
}

// buildArgs constructs the java command line. Every path in it is relative to
// the server directory, which is the working directory of the process.
func (r *Runner) buildArgs(
	provider providers.Provider, opts RunOptions, inst *installation, interactive bool,
) []string {
	var args []string

	usesArgFiles := len(inst.launch.ArgFiles) > 0

	// Listed first so that the memory settings below win over the ones a server pack ships
	if usesArgFiles {
		if _, err := os.Stat(filepath.Join(opts.Directory, userJVMArgsFile)); err == nil {
			args = append(args, "@"+userJVMArgsFile)
		}
	}

	memory, minMemory := r.memorySettings(opts)
	args = append(args, "-Xmx"+memory, "-Xms"+minMemory)

	// Without a terminal the JLine console of Paper, Forge and the like has
	// nothing to drive; this makes them read plain lines from stdin, which is
	// what a control panel writes. A user argument below can still override it.
	if !interactive {
		args = append(args, "-Dterminal.jline=false")
	}

	if r.config != nil {
		args = append(args, r.config.Server.JVMArgs...)
	}
	args = append(args, opts.JVMArgs...)

	if usesArgFiles {
		for _, argFile := range inst.launch.ArgFiles {
			args = append(args, "@"+argFile)
		}
	} else {
		args = append(args, "-jar", inst.launch.Jar)
	}

	return append(args, serverArgs(provider, opts, inst)...)
}

// serverArgs returns the program arguments. Proxies have no GUI switch, and
// Velocity rejects options it does not know.
func serverArgs(provider providers.Provider, opts RunOptions, inst *installation) []string {
	if providers.IsProxy(provider) {
		if listen, ok := provider.(providers.ListenArgsProvider); ok {
			return listen.ListenArgs(opts.Port)
		}
		return nil
	}

	if len(inst.serverArgs) > 0 {
		return inst.serverArgs
	}

	return []string{"--nogui"}
}

func (r *Runner) memorySettings(opts RunOptions) (string, string) {
	memory := opts.Memory
	if memory == "" && r.config != nil {
		memory = r.config.Defaults.Memory
	}
	if memory == "" {
		memory = defaultMemory
	}

	minMemory := opts.MinMemory
	if minMemory == "" && r.config != nil {
		minMemory = r.config.Defaults.MinMemory
	}
	if minMemory == "" {
		minMemory = defaultMemory
	}

	// The JVM refuses to start with an initial heap above the maximum
	maxBytes, maxOK := parseMemory(memory)
	minBytes, minOK := parseMemory(minMemory)
	if maxOK && minOK && minBytes > maxBytes {
		minMemory = memory
	}

	return memory, minMemory
}

// parseMemory parses a JVM heap size such as "512M" or "4g" into bytes
func parseMemory(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	multiplier := int64(1)
	switch value[len(value)-1] {
	case 'k', 'K':
		multiplier = 1 << 10
	case 'm', 'M':
		multiplier = 1 << 20
	case 'g', 'G':
		multiplier = 1 << 30
	case 't', 'T':
		multiplier = 1 << 40
	}
	if multiplier > 1 {
		value = value[:len(value)-1]
	}

	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}

	return n * multiplier, true
}

// start runs the server until it exits or a stop is requested through ctx
func (r *Runner) start(ctx context.Context, javaPath, directory string, args []string) error {
	cmd := exec.CommandContext(ctx, javaPath, args...)
	cmd.Dir = directory
	cmd.Env = utils.JavaEnvironment(javaPath)

	// The descriptors are handed over as they are instead of being copied
	// through pipes: the server reads the console input of mcrun directly, and
	// a terminal stays a terminal for its line editor.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Cancel = func() error {
		fmt.Println("\nStop requested, waiting for the server to shut down...")
		return terminateProcess(cmd.Process)
	}
	cmd.WaitDelay = stopTimeout

	err := cmd.Run()

	if ctx.Err() != nil {
		if state := cmd.ProcessState; state != nil && !state.Exited() {
			fmt.Printf("[WARNING] The server did not stop within %s and was killed\n", stopTimeout)
		}
		return nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &ExitError{Code: exitErr.ExitCode()}
	}

	return err
}

func describeLaunch(target providers.LaunchTarget) string {
	if len(target.ArgFiles) > 0 {
		return "@" + strings.Join(target.ArgFiles, " @")
	}
	return target.Jar
}

// stdinIsTerminal reports whether console input comes from a terminal rather
// than from a pipe or FIFO, as it does under a control panel
func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// updateServerProperties updates server.properties with provided options
func (r *Runner) updateServerProperties(opts RunOptions) error {
	props, err := config.LoadServerProperties(opts.Directory)
	if err != nil {
		return err
	}

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

	if r.config != nil {
		for key, value := range r.config.Server.Properties {
			if props.Get(key) == "" {
				props.Set(key, value)
			}
		}
	}

	if _, err := os.Stat(filepath.Join(opts.Directory, "server.properties")); err == nil {
		return props.Save()
	}

	return nil
}
