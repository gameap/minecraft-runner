package sponge

import (
	"reflect"
	"testing"
)

func TestParseOrderedVersions(t *testing.T) {
	raw := []byte(`{
		"1.21.1-12.0.4-RC2706": {"recommended": false, "tagValues": {"api": "12.0.4"}},
		"1.21.1-12.0.3": {"recommended": true},
		"1.21.1-12.0.2": {"recommended": true},
		"1.21.1-12.0.10-RC1": {"recommended": false}
	}`)

	want := []Version{
		{Version: "1.21.1-12.0.4-RC2706"},
		{Version: "1.21.1-12.0.3", Recommended: true},
		{Version: "1.21.1-12.0.2", Recommended: true},
		{Version: "1.21.1-12.0.10-RC1"},
	}

	// Decoding into a map would shuffle the builds, so one run proves little
	for i := 0; i < 20; i++ {
		got, err := parseOrderedVersions(raw)
		if err != nil {
			t.Fatalf("parseOrderedVersions: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestParseOrderedVersionsEmpty(t *testing.T) {
	for _, raw := range []string{"", "{}"} {
		got, err := parseOrderedVersions([]byte(raw))
		if err != nil || len(got) != 0 {
			t.Errorf("parseOrderedVersions(%q) = %v, %v", raw, got, err)
		}
	}
}
