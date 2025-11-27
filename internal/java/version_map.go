package java

import "fmt"

// GetRequiredJavaVersion returns the minimum required Java version for a Minecraft version
func GetRequiredJavaVersion(mcVersion string) int {
	major, minor, patch := parseMinecraftVersion(mcVersion)

	// MC 1.21+ requires Java 21
	if major >= 1 && minor >= 21 {
		return 21
	}

	// MC 1.20.5+ requires Java 21
	if major >= 1 && minor == 20 && patch >= 5 {
		return 21
	}

	// MC 1.18 - 1.20.4 requires Java 17
	if major >= 1 && minor >= 18 {
		return 17
	}

	// MC 1.17 requires Java 16 (but 17 works fine)
	if major >= 1 && minor >= 17 {
		return 16
	}

	// Older versions can use Java 8
	return 8
}

// GetRecommendedJavaVersion returns the recommended Java version for a Minecraft version
// This may be higher than the minimum required for better performance
func GetRecommendedJavaVersion(mcVersion string) int {
	required := GetRequiredJavaVersion(mcVersion)

	// For Java 16 requirement, recommend 17 (LTS)
	if required == 16 {
		return 17
	}

	return required
}

// IsJavaCompatible checks if a Java version is compatible with a Minecraft version
func IsJavaCompatible(javaVersion int, mcVersion string) bool {
	required := GetRequiredJavaVersion(mcVersion)
	return javaVersion >= required
}

// parseMinecraftVersion parses a Minecraft version string into components
func parseMinecraftVersion(version string) (major, minor, patch int) {
	var m, n, p int
	fmt.Sscanf(version, "%d.%d.%d", &m, &n, &p)
	if m == 0 {
		fmt.Sscanf(version, "%d.%d", &m, &n)
	}
	return m, n, p
}
