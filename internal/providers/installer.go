package providers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/gameap/minecraft-runner/internal/utils"
)

// runInstaller runs a downloaded installer JAR inside the server directory.
// The installer gets no stdin of its own: the console input of the future
// server must not be consumed by it.
func runInstaller(ctx context.Context, javaPath, dir, installerFile string, args ...string) error {
	if javaPath == "" {
		javaPath = "java"
	}

	cmdArgs := append([]string{"-jar", installerFile}, args...)

	cmd := exec.CommandContext(ctx, javaPath, cmdArgs...)
	cmd.Dir = dir
	cmd.Env = utils.JavaEnvironment(javaPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// removeInstaller deletes an installer JAR and the logs it leaves behind
func removeInstaller(dir, installerFile string) {
	os.Remove(filepath.Join(dir, installerFile))
	os.Remove(filepath.Join(dir, installerFile+".log"))
	os.Remove(filepath.Join(dir, "installer.log"))
}

// argFileName is the JVM argument file modular Forge and NeoForge ship per platform
func argFileName() string {
	if runtime.GOOS == "windows" {
		return "win_args.txt"
	}
	return "unix_args.txt"
}

// resolveArgFileLaunch returns the @argfile launch of a modular Forge-like
// install, or nil when libraries/<group path>/<version> holds none
func resolveArgFileLaunch(dir string, librarySubPath ...string) *LaunchTarget {
	argFile := filepath.Join(append(append([]string{"libraries"}, librarySubPath...), argFileName())...)

	if _, err := os.Stat(filepath.Join(dir, argFile)); err != nil {
		return nil
	}

	return &LaunchTarget{ArgFiles: []string{filepath.ToSlash(argFile)}}
}

// resolveLegacyForgeJar finds the runnable JAR a pre-1.17 Forge installer
// leaves in the server directory. Old releases append a Minecraft suffix or
// "-universal" to the name, hence the glob.
func resolveLegacyForgeJar(dir, prefix string) *LaunchTarget {
	matches, err := filepath.Glob(filepath.Join(dir, prefix+"*.jar"))
	if err != nil {
		return nil
	}

	sort.Strings(matches)

	for _, match := range matches {
		name := filepath.Base(match)
		if strings.HasSuffix(name, "-installer.jar") || strings.HasSuffix(name, "-shim.jar") {
			continue
		}
		return &LaunchTarget{Jar: name}
	}

	return nil
}

func installerError(name string, err error) error {
	return fmt.Errorf("%s installer failed (its log is kept in the server directory): %w", name, err)
}
