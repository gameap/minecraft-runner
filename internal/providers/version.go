package providers

import (
	"sort"
	"strconv"
	"strings"
)

// compareMCVersions compares Minecraft version strings component by component,
// so that "1.21.11" sorts above "1.21.4" and the year-based "26.2" above every
// "1.x". A pre-release ("1.20.4-pre1", "26.3-rc-3") sorts below its release.
func compareMCVersions(a, b string) int {
	aNumbers, aSuffix := splitMCVersion(a)
	bNumbers, bSuffix := splitMCVersion(b)

	for i := 0; i < len(aNumbers) || i < len(bNumbers); i++ {
		var x, y int
		if i < len(aNumbers) {
			x = aNumbers[i]
		}
		if i < len(bNumbers) {
			y = bNumbers[i]
		}
		if x != y {
			return x - y
		}
	}

	switch {
	case aSuffix == bSuffix:
		return 0
	case aSuffix == "":
		return 1
	case bSuffix == "":
		return -1
	}
	return strings.Compare(aSuffix, bSuffix)
}

func splitMCVersion(version string) ([]int, string) {
	base, suffix, _ := strings.Cut(version, "-")

	parts := strings.Split(base, ".")
	numbers := make([]int, 0, len(parts))
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			n = 0
		}
		numbers = append(numbers, n)
	}

	return numbers, suffix
}

// isPreRelease reports whether a Minecraft version is a snapshot, pre-release
// or release candidate rather than a release
func isPreRelease(version string) bool {
	if strings.Contains(version, "-") {
		return true
	}
	_, ok := parseSnapshotYear(version)
	return ok
}

// sortMCVersionsDesc sorts Minecraft versions newest first
func sortMCVersionsDesc(versions []string) {
	sort.SliceStable(versions, func(i, j int) bool {
		return compareMCVersions(versions[i], versions[j]) > 0
	})
}

// sortVersionInfosDesc sorts versions by Minecraft version, newest first
func sortVersionInfosDesc(versions []VersionInfo) {
	sort.SliceStable(versions, func(i, j int) bool {
		return compareMCVersions(versions[i].MinecraftVersion, versions[j].MinecraftVersion) > 0
	})
}
