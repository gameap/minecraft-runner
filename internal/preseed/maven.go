package preseed

import (
	"fmt"
	"strings"
)

// MavenPath maps a Maven coordinate, group:artifact:version[:classifier][@ext],
// onto its path in a repository
func MavenPath(coord string) (string, error) {
	ext := "jar"
	if at := strings.LastIndexByte(coord, '@'); at >= 0 {
		coord, ext = coord[:at], coord[at+1:]
	}

	parts := strings.Split(coord, ":")
	if len(parts) < 3 || len(parts) > 4 || ext == "" {
		return "", fmt.Errorf("malformed Maven coordinate %q", coord)
	}
	for _, part := range append(parts, ext) {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, `/\`) {
			return "", fmt.Errorf("malformed Maven coordinate %q", coord)
		}
	}

	group, artifact, version := parts[0], parts[1], parts[2]

	file := artifact + "-" + version
	if len(parts) == 4 {
		file += "-" + parts[3]
	}

	return strings.ReplaceAll(group, ".", "/") + "/" + artifact + "/" + version + "/" + file + "." + ext, nil
}

// repositoryURLs joins path to each repository
func repositoryURLs(repositories []string, path string) []string {
	urls := make([]string, 0, len(repositories))
	for _, repo := range repositories {
		urls = append(urls, strings.TrimRight(repo, "/")+"/"+path)
	}
	return urls
}
