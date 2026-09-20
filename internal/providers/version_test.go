package providers

import (
	"reflect"
	"testing"
)

func TestCompareMCVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.21.11", "1.21.4", 1},
		{"1.21.4", "1.21.11", -1},
		{"1.9", "1.10", -1},
		{"26.2", "1.21.11", 1},
		{"26.1.2", "26.1", 1},
		{"26.2", "26.1.2", 1},
		{"1.21", "1.21.0", 0},
		{"1.20.4", "1.20.4", 0},
		{"1.20.4-pre1", "1.20.4", -1},
		{"26.3", "26.3-rc-3", 1},
		{"26.3-rc-3", "26.3-rc-2", 1},
		{"1.20.4-pre1", "1.20.3", 1},
	}

	for _, tt := range tests {
		got := compareMCVersions(tt.a, tt.b)
		if sign(got) != tt.want {
			t.Errorf("compareMCVersions(%q, %q) = %d, want sign %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSortMCVersionsDesc(t *testing.T) {
	versions := []string{"1.21.11", "1.21.4", "1.21.5", "26.1.2", "1.21.8", "26.2", "1.9", "1.10"}
	want := []string{"26.2", "26.1.2", "1.21.11", "1.21.8", "1.21.5", "1.21.4", "1.10", "1.9"}

	sortMCVersionsDesc(versions)

	if !reflect.DeepEqual(versions, want) {
		t.Errorf("sortMCVersionsDesc() = %v, want %v", versions, want)
	}
}

func TestIsPreRelease(t *testing.T) {
	tests := map[string]bool{
		"1.21.4":          false,
		"26.3":            false,
		"26.3-rc-3":       true,
		"1.20.4-pre1":     true,
		"26.3-snapshot-8": true,
		"24w14a":          true,
	}

	for version, want := range tests {
		if got := isPreRelease(version); got != want {
			t.Errorf("isPreRelease(%q) = %v, want %v", version, got, want)
		}
	}
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}
