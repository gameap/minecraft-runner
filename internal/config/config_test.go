package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppliesLocalConfig(t *testing.T) {
	dir := t.TempDir()

	local := `version: "1.20.1"
mod: forge
mod_version: "47.4.10"
java:
  version: 17
  memory: 6G
  min_memory: 2G
  args:
    - "-Dfoo=bar"
server:
  ip: "0.0.0.0"
  port: 25570
  query_port: 25571
  rcon_port: 25575
  rcon_password: "secret"
`
	if err := os.WriteFile(filepath.Join(dir, ".mcrun.yaml"), []byte(local), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "no-global-config.yaml"), dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Defaults.Version != "1.20.1" || cfg.Defaults.Mod != "forge" || cfg.Defaults.ModVersion != "47.4.10" {
		t.Errorf("defaults = %+v", cfg.Defaults)
	}
	if cfg.Defaults.Memory != "6G" || cfg.Defaults.MinMemory != "2G" {
		t.Errorf("memory = %s/%s", cfg.Defaults.Memory, cfg.Defaults.MinMemory)
	}
	if cfg.Java.Version != 17 || cfg.Java.Path != "" {
		t.Errorf("java = %d %q", cfg.Java.Version, cfg.Java.Path)
	}
	if _, pinned := cfg.Java.Paths[17]; pinned {
		t.Error("a Java version without a path must not pin an empty binary path")
	}
	if cfg.Server.Network.Port != 25570 || cfg.Server.Network.RconPassword != "secret" {
		t.Errorf("network = %+v", cfg.Server.Network)
	}

	last := cfg.Server.JVMArgs[len(cfg.Server.JVMArgs)-1]
	if last != "-Dfoo=bar" {
		t.Errorf("JVM args = %v", cfg.Server.JVMArgs)
	}
}

func TestLoadJavaPathVariants(t *testing.T) {
	t.Run("a path with a version pins that version", func(t *testing.T) {
		dir := t.TempDir()
		writeLocalConfig(t, dir, "java:\n  version: 21\n  path: /opt/jdk21/bin/java\n")

		cfg, err := Load(filepath.Join(dir, "none.yaml"), dir)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Java.Paths[21] != "/opt/jdk21/bin/java" || cfg.Java.Version != 21 || cfg.Java.Path != "" {
			t.Errorf("java = %+v", cfg.Java)
		}
	})

	t.Run("a path on its own is the binary to use", func(t *testing.T) {
		dir := t.TempDir()
		writeLocalConfig(t, dir, "java:\n  path: /opt/java/bin/java\n")

		cfg, err := Load(filepath.Join(dir, "none.yaml"), dir)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Java.Path != "/opt/java/bin/java" || cfg.Java.Version != 0 {
			t.Errorf("java = %+v", cfg.Java)
		}
	})
}

func TestLoadIgnoresNegativeJavaVersion(t *testing.T) {
	t.Run("on its own it counts as not set", func(t *testing.T) {
		dir := t.TempDir()
		writeLocalConfig(t, dir, "java:\n  version: -5\n")

		cfg, err := Load(filepath.Join(dir, "none.yaml"), dir)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Java.Version != 0 || len(cfg.Java.Paths) != 0 {
			t.Errorf("java = %+v, a negative version must not be applied", cfg.Java)
		}
	})

	t.Run("next to a path the path is still used", func(t *testing.T) {
		dir := t.TempDir()
		writeLocalConfig(t, dir, "java:\n  version: -5\n  path: /opt/java/bin/java\n")

		cfg, err := Load(filepath.Join(dir, "none.yaml"), dir)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Java.Path != "/opt/java/bin/java" || cfg.Java.Version != 0 || len(cfg.Java.Paths) != 0 {
			t.Errorf("java = %+v", cfg.Java)
		}
	})
}

func writeLocalConfig(t *testing.T, dir, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, ".mcrun.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write the .mcrun.yaml fixture: %v", err)
	}
}

func TestLoadRejectsBrokenLocalConfig(t *testing.T) {
	dir := t.TempDir()
	writeLocalConfig(t, dir, "java: [not, a, mapping\n")

	if _, err := Load(filepath.Join(dir, "none.yaml"), dir); err == nil {
		t.Error("a malformed .mcrun.yaml must be reported, not silently ignored")
	}
}

func TestLoadWithoutAnyConfig(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load(filepath.Join(dir, "none.yaml"), dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Defaults.Mod != "vanilla" {
		t.Errorf("mod = %q", cfg.Defaults.Mod)
	}
}
