package mcrun

import "github.com/gameap/minecraft-runner/internal/providers"

// createRegistry creates a provider registry with all providers
func createRegistry() *providers.Registry {
	registry := providers.NewRegistry()

	registry.Register(providers.NewVanillaProvider())
	registry.Register(providers.NewPaperProvider())
	registry.Register(providers.NewFoliaProvider())
	registry.Register(providers.NewPurpurProvider())
	registry.Register(providers.NewLeafProvider())
	registry.Register(providers.NewPufferfishProvider())
	registry.Register(providers.NewFabricProvider())
	registry.Register(providers.NewQuiltProvider())
	registry.Register(providers.NewForgeProvider())
	registry.Register(providers.NewNeoForgeProvider())
	registry.Register(providers.NewMohistProvider())
	registry.Register(providers.NewBannerProvider())
	registry.Register(providers.NewSpongeVanillaProvider())
	registry.Register(providers.NewSpigotProvider())
	registry.Register(providers.NewCraftBukkitProvider())
	registry.Register(providers.NewCauldronProvider())
	registry.Register(providers.NewWaterfallProvider())
	registry.Register(providers.NewVelocityProvider())
	registry.Register(providers.NewBungeecordProvider())

	return registry
}
