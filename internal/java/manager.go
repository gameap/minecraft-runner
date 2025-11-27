package java

import (
	"context"
	"fmt"

	"github.com/gameap/minecraft-runner/internal/config"
)

// Manager handles Java version management
type Manager struct {
	detector  *Detector
	installer *Installer
	config    *config.Config
}

// NewManager creates a new Java manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		detector:  NewDetector(),
		installer: NewInstaller(),
		config:    cfg,
	}
}

// GetForMinecraftVersion finds or installs Java suitable for a Minecraft version
func (m *Manager) GetForMinecraftVersion(ctx context.Context, mcVersion string, autoInstall bool) (*Installation, error) {
	required := GetRequiredJavaVersion(mcVersion)

	// Check config for custom path
	if m.config != nil && m.config.Java.Paths != nil {
		if customPath, ok := m.config.Java.Paths[required]; ok && customPath != "" {
			inst, err := m.detector.getInstallationInfo(customPath)
			if err == nil && inst.Version >= required {
				return inst, nil
			}
		}
	}

	// Try to find existing installation
	inst, err := m.detector.FindForVersion(mcVersion)
	if err == nil {
		return inst, nil
	}

	// Auto-install if enabled
	if autoInstall || (m.config != nil && m.config.Java.AutoInstall) {
		fmt.Printf("Java %d not found. Installing...\n", required)
		return m.installer.Install(ctx, required, InstallOptions{})
	}

	return nil, fmt.Errorf("Java %d required for Minecraft %s but not found. Run 'mcrun install java --version=%d' to install",
		required, mcVersion, required)
}

// GetByVersion finds or installs a specific Java version
func (m *Manager) GetByVersion(ctx context.Context, version int, autoInstall bool) (*Installation, error) {
	// Check config for custom path
	if m.config != nil && m.config.Java.Paths != nil {
		if customPath, ok := m.config.Java.Paths[version]; ok && customPath != "" {
			inst, err := m.detector.getInstallationInfo(customPath)
			if err == nil && inst.Version == version {
				return inst, nil
			}
		}
	}

	// Search for installed version
	installations, err := m.detector.DetectAll()
	if err != nil {
		return nil, err
	}

	for _, inst := range installations {
		if inst.Version == version {
			return &inst, nil
		}
	}

	// Auto-install if enabled
	if autoInstall || (m.config != nil && m.config.Java.AutoInstall) {
		fmt.Printf("Java %d not found. Installing...\n", version)
		return m.installer.Install(ctx, version, InstallOptions{})
	}

	return nil, fmt.Errorf("Java %d not found. Run 'mcrun install java --version=%d' to install",
		version, version)
}

// GetByPath returns an installation for a specific Java binary path
func (m *Manager) GetByPath(path string) (*Installation, error) {
	return m.detector.getInstallationInfo(path)
}

// ListInstalled returns all installed Java versions
func (m *Manager) ListInstalled() ([]Installation, error) {
	return m.detector.DetectAll()
}

// Install installs a specific Java version
func (m *Manager) Install(ctx context.Context, version int, opts InstallOptions) (*Installation, error) {
	return m.installer.Install(ctx, version, opts)
}

// InstallBundled installs Java to a server's local directory
func (m *Manager) InstallBundled(ctx context.Context, serverDir string, version int) (*Installation, error) {
	return m.installer.InstallBundled(ctx, serverDir, version)
}
