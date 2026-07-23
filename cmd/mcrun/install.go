package mcrun

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gameap/minecraft-runner/internal/java"
)

var (
	installJavaVersion int
	installSystemWide  bool
	installListJava    bool
	installSetDefault  bool
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install dependencies (Java)",
	Long: `Install dependencies required for running Minecraft servers.

Currently supports Java installation via Eclipse Adoptium.

Examples:
  mcrun install java                    # Install recommended Java version
  mcrun install java --version=21       # Install specific Java version
  mcrun install java --version=17 --system  # Install system-wide (requires root/admin)
  mcrun install java --list             # List installed Java versions`,
}

var installJavaCmd = &cobra.Command{
	Use:   "java",
	Short: "Install or manage Java",
	Long: `Install Java from Eclipse Adoptium or list installed Java versions.

Available LTS versions: 8, 11, 17, 21, 25

Examples:
  mcrun install java                    # Install latest LTS Java
  mcrun install java --version=21       # Install Java 21
  mcrun install java --version=17 --system  # Install system-wide
  mcrun install java --list             # List installed Java versions
  mcrun install java --version=17 --set-default  # Install and set as default`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		manager := java.NewManager(cfg)

		// List mode
		if installListJava {
			return listInstalledJava(manager)
		}

		// Install mode
		version := installJavaVersion
		if version == 0 {
			version = 25 // Default to latest LTS
		}

		fmt.Printf("Installing Java %d...\n", version)

		inst, err := manager.Install(ctx, version, java.InstallOptions{
			SystemWide: installSystemWide,
			SetDefault: installSetDefault,
		})
		if err != nil {
			return fmt.Errorf("failed to install Java: %w", err)
		}

		fmt.Printf("\n[OK] Java %d installed successfully!\n", version)
		fmt.Printf("  Path: %s\n", inst.Path)
		fmt.Printf("  Version: %s\n", inst.FullVersion)
		fmt.Printf("  Vendor: %s\n", inst.Vendor)

		if installSystemWide {
			fmt.Println("\n  Installed system-wide.")
		} else {
			fmt.Printf("\n  Installed to user directory (~/.mcrun/java)\n")
		}

		return nil
	},
}

func listInstalledJava(manager *java.Manager) error {
	installations, err := manager.ListInstalled()
	if err != nil {
		return fmt.Errorf("failed to detect Java: %w", err)
	}

	if len(installations) == 0 {
		fmt.Println("[WARNING] No Java installations found.")
		fmt.Println("\nInstall Java with:")
		fmt.Println("  mcrun install java --version=25")
		return nil
	}

	fmt.Println("\nInstalled Java versions:")
	fmt.Println("--------------------------------------------------")
	fmt.Println("Version | Full Version     | Vendor           | Path")
	fmt.Println("--------------------------------------------------")

	for _, inst := range installations {
		scope := ""
		if inst.IsSystem {
			scope = " (system)"
		}

		// Truncate path for display
		path := inst.Path
		if len(path) > 40 {
			path = "..." + path[len(path)-37:]
		}

		fmt.Printf("%-7d | %-16s | %-16s | %s%s\n",
			inst.Version,
			truncate(inst.FullVersion, 16),
			truncate(inst.Vendor, 16),
			path,
			scope,
		)
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.AddCommand(installJavaCmd)

	installJavaCmd.Flags().IntVar(&installJavaVersion, "version", 0, "Java version to install (8, 11, 17, 21, 25)")
	installJavaCmd.Flags().BoolVar(&installSystemWide, "system", false, "install system-wide (requires root/admin)")
	installJavaCmd.Flags().BoolVar(&installListJava, "list", false, "list installed Java versions")
	installJavaCmd.Flags().BoolVar(&installSetDefault, "set-default", false, "set as system default after install")
}
