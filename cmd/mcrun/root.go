package mcrun

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gameap/minecraft-runner/internal/config"
)

var (
	cfgFile     string
	verbose     bool
	mcVersion   string
	mod         string
	modVersion  string
	javaVersion int
	javaPath    string
	serverDir   string

	cfg *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "mcrun",
	Short: "Minecraft Server Runner - Download and run Minecraft servers",
	Long: `
                  ###           ####
               #######################
               #######################
              #######` + "`````````" + `#######
            ######   ############   ######
          #####  ####             ##  ########
      ########  ###               ####  ########
     ######## ###                 ###### ########
      ###### ####     ################### ######
       ####  ####     ################### #####
       #### #####     ###           #####  ####
       #### #####     ###           #####  ####
       ####  ####     #########     ##### #####
      ###### ####     #########     ##### ######
     ######## ###       ######      #### ########
      ########  ##                 ###  ########
       ########   ###            ###   #####
            #####    ############   ######
              #######` + "`````````" + `#######
                #######################
                #######################
                 #####          ####

            Minecraft Server Runner

Cross-platform CLI tool to download and run Minecraft servers
with integrated Java management.

Supported server types: vanilla, paper, forge, fabric, spigot, craftbukkit, cauldron,
waterfall, velocity, bungeecord`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load(cfgFile, serverDir)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Apply config defaults if flags not set
		if mcVersion == "" && cfg.Defaults.Version != "" {
			mcVersion = cfg.Defaults.Version
		}
		if mod == "" && cfg.Defaults.Mod != "" {
			mod = cfg.Defaults.Mod
		}

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: ~/.mcrun/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "V", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVar(&mcVersion, "version", "", "Minecraft version (e.g., 1.20.4)")
	rootCmd.PersistentFlags().StringVarP(&mod, "mod", "m", "vanilla", "server mod type (vanilla, paper, forge, fabric, spigot, craftbukkit, cauldron, waterfall, velocity, bungeecord)")
	rootCmd.PersistentFlags().StringVar(&modVersion, "mod-version", "", "mod-specific version")
	rootCmd.PersistentFlags().IntVar(&javaVersion, "java", 0, "Java version override (8, 11, 17, 21, 25)")
	rootCmd.PersistentFlags().StringVar(&javaPath, "java-path", "", "custom Java binary path")
	rootCmd.PersistentFlags().StringVarP(&serverDir, "dir", "d", ".", "server working directory")
}
