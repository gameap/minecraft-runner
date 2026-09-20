package forge

import "testing"

func TestFindArtifactVersion(t *testing.T) {
	tests := []struct {
		name      string
		artifacts []string
		mc, forge string
		want      string
		found     bool
	}{
		{
			name:      "modern releases are just mc-forge",
			artifacts: []string{"1.20.1-47.4.9", "1.20.1-47.4.10"},
			mc:        "1.20.1", forge: "47.4.10",
			want: "1.20.1-47.4.10", found: true,
		},
		{
			name:      "old releases repeat the Minecraft version",
			artifacts: []string{"1.7.10-10.13.4.1566-1.7.10", "1.7.10-10.13.4.1614-1.7.10"},
			mc:        "1.7.10", forge: "10.13.4.1614",
			want: "1.7.10-10.13.4.1614-1.7.10", found: true,
		},
		{
			name:      "the suffix may differ from the Minecraft version",
			artifacts: []string{"1.10-12.18.0.2000-1.10.0"},
			mc:        "1.10", forge: "12.18.0.2000",
			want: "1.10-12.18.0.2000-1.10.0", found: true,
		},
		{
			name:      "a version that merely shares a prefix does not match",
			artifacts: []string{"1.20.1-47.4.100"},
			mc:        "1.20.1", forge: "47.4.10",
		},
		{
			name: "nothing published", mc: "1.20.1", forge: "47.4.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := FindArtifactVersion(tt.artifacts, tt.mc, tt.forge)
			if got != tt.want || found != tt.found {
				t.Errorf("FindArtifactVersion() = %q, %v; want %q, %v", got, found, tt.want, tt.found)
			}
		})
	}
}

func TestForgeVersionOf(t *testing.T) {
	tests := []struct{ artifact, mc, want string }{
		{"1.20.1-47.4.10", "1.20.1", "47.4.10"},
		{"1.7.10-10.13.4.1614-1.7.10", "1.7.10", "10.13.4.1614"},
		{"26.2-65.1.3", "26.2", "65.1.3"},
	}

	for _, tt := range tests {
		if got := ForgeVersionOf(tt.artifact, tt.mc); got != tt.want {
			t.Errorf("ForgeVersionOf(%q, %q) = %q, want %q", tt.artifact, tt.mc, got, tt.want)
		}
	}
}

func TestInstallerURL(t *testing.T) {
	want := "https://maven.minecraftforge.net/net/minecraftforge/forge/" +
		"1.7.10-10.13.4.1614-1.7.10/forge-1.7.10-10.13.4.1614-1.7.10-installer.jar"

	if got := NewClient().GetInstallerURL("1.7.10-10.13.4.1614-1.7.10"); got != want {
		t.Errorf("GetInstallerURL() = %q, want %q", got, want)
	}
}
