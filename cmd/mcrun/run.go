package mcrun

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/gameap/minecraft-runner/internal/java"
	"github.com/gameap/minecraft-runner/internal/providers"
	"github.com/gameap/minecraft-runner/internal/server"
)

var (
	runIP           string
	runPort         int
	runQueryPort    int
	runRconPort     int
	runRconPassword string
	runMemory       string
	runMinMemory    string
	runJVMArgs      []string
	runAcceptEULA   bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Download (if needed) and run a Minecraft server",
	Long: `Downloads the specified Minecraft server (if not already present) and runs it.

Examples:
  mcrun run                                    # Run latest vanilla server
  mcrun run --version=1.20.4                   # Run specific vanilla version
  mcrun run --mod=paper --version=1.20.4       # Run Paper server
  mcrun run --mod=forge --version=1.20.4       # Run Forge server
  mcrun run --version=1.20.4 --memory=4G       # Run with 4GB memory`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create context that cancels on interrupt
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle interrupt signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			cancel()
		}()

		// Create provider registry
		registry := createRegistry()

		// Create Java manager
		javaManager := java.NewManager(cfg)

		// Create runner
		runner := server.NewRunner(registry, javaManager, cfg)

		// Run server
		return runner.Run(ctx, server.RunOptions{
			Version:      mcVersion,
			Mod:          mod,
			ModVersion:   modVersion,
			Directory:    serverDir,
			IP:           runIP,
			Port:         runPort,
			QueryPort:    runQueryPort,
			RconPort:     runRconPort,
			RconPassword: runRconPassword,
			Memory:       runMemory,
			MinMemory:    runMinMemory,
			JVMArgs:      runJVMArgs,
			AcceptEULA:   runAcceptEULA,
			JavaPath:     javaPath,
			JavaVersion:  javaVersion,
		})
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&runIP, "ip", "", "server bind IP")
	runCmd.Flags().IntVar(&runPort, "port", 0, "server port (default: 25565)")
	runCmd.Flags().IntVar(&runQueryPort, "query-port", 0, "query port (enables query)")
	runCmd.Flags().IntVar(&runRconPort, "rcon-port", 0, "RCON port")
	runCmd.Flags().StringVar(&runRconPassword, "rcon-password", "", "RCON password (enables RCON)")
	runCmd.Flags().StringVar(&runMemory, "memory", "", "max heap size (e.g., '4G')")
	runCmd.Flags().StringVar(&runMinMemory, "min-memory", "", "initial heap size (e.g., '1G')")
	runCmd.Flags().StringSliceVar(&runJVMArgs, "jvm-args", nil, "additional JVM arguments")
	runCmd.Flags().BoolVar(&runAcceptEULA, "accept-eula", false, "automatically accept Minecraft EULA")
}

// createRegistry creates a provider registry with all providers
func createRegistry() *providers.Registry {
	registry := providers.NewRegistry()

	registry.Register(providers.NewVanillaProvider())
	registry.Register(providers.NewPaperProvider())
	registry.Register(providers.NewFabricProvider())
	registry.Register(providers.NewForgeProvider())
	registry.Register(providers.NewSpigotProvider())
	registry.Register(providers.NewCraftBukkitProvider())
	registry.Register(providers.NewCauldronProvider())

	return registry
}
