package mcrun

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gameap/minecraft-runner/internal/config"
	"github.com/gameap/minecraft-runner/internal/server"
	"github.com/gameap/minecraft-runner/internal/utils"
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

Supported server types:
` + supportedModsHelp(),
	// A server that exits with an error is not a usage mistake, and Execute
	// reports the error itself
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		utils.SetVersion(Version)

		var err error
		cfg, err = config.Load(cfgFile, serverDir)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		applyConfigDefaults(cmd)

		return nil
	},
}

// applyConfigDefaults lets the config files stand in for flags that were not
// given. The mod flag has a default of its own, so only Changed can tell
// whether the user asked for vanilla.
func applyConfigDefaults(cmd *cobra.Command) {
	flags := cmd.Flags()

	if !flags.Changed("version") && cfg.Defaults.Version != "" {
		mcVersion = cfg.Defaults.Version
	}
	if !flags.Changed("mod") && cfg.Defaults.Mod != "" {
		mod = cfg.Defaults.Mod
	}
	if !flags.Changed("mod-version") && cfg.Defaults.ModVersion != "" {
		modVersion = cfg.Defaults.ModVersion
	}
	if !flags.Changed("java") && cfg.Java.Version != 0 {
		javaVersion = cfg.Java.Version
	}
	if !flags.Changed("java-path") && cfg.Java.Path != "" {
		javaPath = cfg.Java.Path
	}
}

func supportedModsHelp() string {
	return "  " + strings.Join(createRegistry().List(), ", ")
}

func Execute() {
	err := rootCmd.Execute()
	if err == nil {
		return
	}

	var exitErr *server.ExitError
	if errors.As(err, &exitErr) {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		if exitErr.Code > 0 {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: ~/.mcrun/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "V", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVar(&mcVersion, "version", "", "Minecraft version (e.g., 1.20.4)")
	rootCmd.PersistentFlags().StringVarP(&mod, "mod", "m", "vanilla", "server mod type (see the list of supported server types)")
	rootCmd.PersistentFlags().StringVar(&modVersion, "mod-version", "", "mod-specific version")
	rootCmd.PersistentFlags().IntVar(&javaVersion, "java", 0, "Java version override (8, 11, 17, 21, 25)")
	rootCmd.PersistentFlags().StringVar(&javaPath, "java-path", "", "custom Java binary path")
	rootCmd.PersistentFlags().StringVarP(&serverDir, "dir", "d", ".", "server working directory")
}
