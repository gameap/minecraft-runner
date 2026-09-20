package neoforge

import "testing"

func TestParseRelease(t *testing.T) {
	tests := []struct {
		version   string
		minecraft string
		stable    bool
		ok        bool
	}{
		{"21.1.251", "1.21.1", true, true},
		{"21.0.167", "1.21", true, true},
		{"20.4.251", "1.20.4", true, true},
		{"21.11.45", "1.21.11", true, true},
		{"20.2.3-beta", "1.20.2", false, true},
		{"26.2.0.88", "26.2", true, true},
		{"26.1.2.109", "26.1.2", true, true},
		{"26.3.0.7-beta", "26.3", false, true},
		{"26.1.0.0-alpha.1+snapshot-1", "26.1", false, true},
		{"0.25w14craftmine.3-beta", "", false, false},
		{"47.1.82", "", false, false},
		{"garbage", "", false, false},
	}

	for _, tt := range tests {
		release, ok := ParseRelease(tt.version)
		if ok != tt.ok {
			t.Errorf("ParseRelease(%q) ok = %v, want %v", tt.version, ok, tt.ok)
			continue
		}
		if !ok {
			continue
		}
		if release.MinecraftVersion != tt.minecraft || release.Stable != tt.stable {
			t.Errorf("ParseRelease(%q) = %s stable=%v, want %s stable=%v",
				tt.version, release.MinecraftVersion, release.Stable, tt.minecraft, tt.stable)
		}
	}
}

func TestReleaseInstaller(t *testing.T) {
	release, _ := ParseRelease("21.1.251")

	wantURL := "https://maven.neoforged.net/releases/net/neoforged/neoforge/21.1.251/neoforge-21.1.251-installer.jar"
	if got := release.InstallerURL(); got != wantURL {
		t.Errorf("InstallerURL() = %q, want %q", got, wantURL)
	}

	legacy, ok := parseLegacyRelease("1.20.1-47.1.106")
	if !ok {
		t.Fatal("parseLegacyRelease failed")
	}
	if legacy.Version != "47.1.106" || legacy.MinecraftVersion != "1.20.1" || legacy.Artifact != ArtifactLegacyForge {
		t.Errorf("legacy = %+v", legacy)
	}

	wantLegacyURL := "https://maven.neoforged.net/releases/net/neoforged/forge/1.20.1-47.1.106/forge-1.20.1-47.1.106-installer.jar"
	if got := legacy.InstallerURL(); got != wantLegacyURL {
		t.Errorf("legacy InstallerURL() = %q, want %q", got, wantLegacyURL)
	}

	if _, ok := parseLegacyRelease("47.1.82"); ok {
		t.Error("an entry without the Minecraft prefix must be skipped")
	}
}
