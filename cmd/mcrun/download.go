package mcrun

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/gameap/minecraft-runner/internal/server"
)

var downloadForce bool

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download a Minecraft server JAR without running",
	Long: `Downloads the specified Minecraft server JAR file.
The server will not be started after download.

Examples:
  mcrun download --version=1.20.4                 # Download vanilla 1.20.4
  mcrun download --mod=paper --version=1.20.4     # Download Paper 1.20.4
  mcrun download --mod=forge --version=1.20.4     # Download Forge 1.20.4
  mcrun download --version=1.20.4 --force         # Re-download even if exists`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		registry := createRegistry()

		downloader := server.NewDownloader(registry)

		jar, err := downloader.Download(ctx, server.DownloadOptions{
			Version:    mcVersion,
			Mod:        mod,
			ModVersion: modVersion,
			Directory:  serverDir,
			Force:      downloadForce,
		})
		if err != nil {
			return err
		}

		color.Green("\n✓ Download complete!")
		fmt.Printf("  Server: %s\n", mod)
		fmt.Printf("  Version: %s\n", jar.Version)
		if jar.ModVersion != "" {
			fmt.Printf("  Mod Version: %s\n", jar.ModVersion)
		}
		fmt.Printf("  File: %s\n", jar.Filename)

		if jar.RequiresInstall {
			color.Yellow("\n⚠ This server requires installation.")
			fmt.Println("  The installer will run automatically when you use 'mcrun run'")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().BoolVar(&downloadForce, "force", false, "force re-download even if file exists")
}
