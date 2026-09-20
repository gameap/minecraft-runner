package mcrun

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/gameap/minecraft-runner/internal/java"
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

The server is stopped gracefully on SIGINT, SIGTERM and SIGHUP: it gets time to
save the world before mcrun gives up and kills it.

Examples:
  mcrun run                                    # Run latest vanilla server
  mcrun run --version=1.20.4                   # Run specific vanilla version
  mcrun run --mod=paper --version=1.20.4       # Run Paper server
  mcrun run --mod=forge --version=1.20.1       # Run Forge server
  mcrun run --mod=neoforge --version=1.21.1    # Run NeoForge server
  mcrun run --version=1.20.4 --memory=4G       # Run with 4GB memory`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		defer stop()

		applyNetworkDefaults(cmd)

		runner := server.NewRunner(createRegistry(), java.NewManager(cfg), cfg)

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

// applyNetworkDefaults fills the network flags that were not given from the
// per-server config
func applyNetworkDefaults(cmd *cobra.Command) {
	flags := cmd.Flags()
	network := cfg.Server.Network

	if !flags.Changed("ip") && network.IP != "" {
		runIP = network.IP
	}
	if !flags.Changed("port") && network.Port != 0 {
		runPort = network.Port
	}
	if !flags.Changed("query-port") && network.QueryPort != 0 {
		runQueryPort = network.QueryPort
	}
	if !flags.Changed("rcon-port") && network.RconPort != 0 {
		runRconPort = network.RconPort
	}
	if !flags.Changed("rcon-password") && network.RconPassword != "" {
		runRconPassword = network.RconPassword
	}
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
