package mojang

import "testing"

func TestNewClientManifestURL(t *testing.T) {
	if got := NewClient().manifestURL; got != versionManifestURL {
		t.Errorf("NewClient() manifestURL = %q, want %q", got, versionManifestURL)
	}
	if got := NewClientWithManifestURL("").manifestURL; got != versionManifestURL {
		t.Errorf("NewClientWithManifestURL(\"\") manifestURL = %q, want %q", got, versionManifestURL)
	}
	if got := NewClientWithManifestURL("http://example.com/m.json").manifestURL; got != "http://example.com/m.json" {
		t.Errorf("NewClientWithManifestURL(custom) manifestURL = %q", got)
	}
}
