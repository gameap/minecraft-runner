package preseed

import (
	"context"
	"fmt"

	"github.com/gameap/minecraft-runner/pkg/api/mojang"
)

// Versions looks up the files of a Minecraft version
type Versions interface {
	GetVersion(ctx context.Context, id string) (*mojang.VersionDetail, error)
}

// VanillaServer describes the vanilla server of version, to be stored at path
func VanillaServer(ctx context.Context, versions Versions, version, path string) (File, error) {
	detail, err := versions.GetVersion(ctx, version)
	if err != nil {
		return File{}, fmt.Errorf("failed to look up the vanilla server %s: %w", version, err)
	}

	server := detail.Downloads.Server
	if server.URL == "" {
		return File{}, fmt.Errorf("minecraft %s has no server download", version)
	}

	return File{Path: path, URLs: mojangURLs(server.URL), SHA1: server.SHA1}, nil
}
