package mcrun

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available Minecraft server versions",
	Long: `Lists available versions for the specified server type.

Examples:
  mcrun list                           # List vanilla versions
  mcrun list --mod=paper               # List Paper versions
  mcrun list --mod=forge               # List Forge versions
  mcrun list --mod=paper --version=1.20.4  # List Paper builds for 1.20.4`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		registry := createRegistry()

		provider, err := registry.Get(mod)
		if err != nil {
			return err
		}

		// If version specified, list mod versions for that MC version
		if mcVersion != "" {
			versions, err := provider.ListModVersions(ctx, mcVersion)
			if err != nil {
				return fmt.Errorf("failed to list mod versions: %w", err)
			}

			if len(versions) == 0 {
				fmt.Printf("No %s versions found for Minecraft %s\n", mod, mcVersion)
				return nil
			}

			fmt.Printf("\n%s versions for Minecraft %s:\n", mod, mcVersion)
			fmt.Println("------------------------------------------")

			for _, v := range versions {
				status := ""
				if v.IsStable {
					status = " (stable)"
				}
				fmt.Printf("  %s%s\n", v.ModVersion, status)
			}

			return nil
		}

		// List Minecraft versions
		versions, err := provider.ListVersions(ctx)
		if err != nil {
			return fmt.Errorf("failed to list versions: %w", err)
		}

		fmt.Printf("\nAvailable %s versions:\n", mod)
		fmt.Println("------------------------------------------")
		fmt.Println("Version          | Type      | Status")
		fmt.Println("------------------------------------------")

		// Limit output for readability
		limit := 30
		if verbose {
			limit = len(versions)
		}

		count := 0
		for _, v := range versions {
			if count >= limit {
				fmt.Printf("\n... and %d more versions (use --verbose to see all)\n", len(versions)-limit)
				break
			}

			vType := v.Type
			if vType == "" {
				vType = "release"
			}

			status := ""
			if v.IsStable {
				status = "stable"
			} else if vType == "snapshot" {
				status = "snapshot"
			}

			modVer := ""
			if v.ModVersion != "" {
				modVer = fmt.Sprintf(" (%s)", v.ModVersion)
			}

			fmt.Printf("%-16s | %-9s | %s%s\n", v.MinecraftVersion, vType, status, modVer)
			count++
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
