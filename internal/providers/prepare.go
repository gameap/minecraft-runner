package providers

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gameap/minecraft-runner/internal/preseed"
	"github.com/gameap/minecraft-runner/internal/utils"
	"github.com/gameap/minecraft-runner/pkg/api/mojang"
)

// prepareModules puts the modules of BungeeCord and Waterfall in place
func prepareModules(ctx context.Context, dir string, jar *ServerJar) error {
	if len(jar.Extras) == 0 {
		return nil
	}
	return preseed.ProxyModules(ctx, utils.NewHTTPClient(false), dir, jar.Extras)
}

// preparePaperclip prepares a server started through Paperclip or QuantumLeaper
func preparePaperclip(ctx context.Context, dir string, jar *ServerJar) error {
	return preseed.Paperclip(ctx, utils.NewHTTPClient(false), dir, filepath.Join(dir, jar.Filename))
}

// prepareMohist prepares a Mohist, Banner or Cauldron server
func prepareMohist(ctx context.Context, dir string, jar *ServerJar) error {
	return preseed.Mohist(ctx, utils.NewHTTPClient(false), mojang.NewClient(), dir, filepath.Join(dir, jar.Filename))
}

// prepareSponge prepares a SpongeVanilla server
func prepareSponge(ctx context.Context, dir string, jar *ServerJar) error {
	return preseed.Sponge(ctx, utils.NewHTTPClient(false), mojang.NewClient(), dir, filepath.Join(dir, jar.Filename))
}

// installForge runs a Forge or NeoForge installer. What it would download is
// put in place first, so it runs offline. When that is not possible, or the
// offline run fails, the original installer runs and downloads on its own.
func installForge(ctx context.Context, javaPath, dir, installer, name string) error {
	install, err := preseed.ForgeInstaller(ctx, utils.NewHTTPClient(false), mojang.NewClient(), dir, installer, name)
	if err != nil {
		fmt.Printf("[WARNING] Could not prepare the %s installer, it downloads its files itself: %v\n", name, err)
	}

	if install != nil && install.Offline {
		err := runInstaller(ctx, javaPath, dir, install.Installer, "--offline", "--installServer")
		install.Remove()
		if err == nil || ctx.Err() != nil {
			return err
		}
		fmt.Printf("[WARNING] The offline %s install failed (%v), running the installer online\n", name, err)
	}

	return runInstaller(ctx, javaPath, dir, installer, "--installServer")
}
