package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// JavaEnvironment returns the environment for a process started with the given
// Java binary: that Java comes first on PATH and is JAVA_HOME. Servers spawn
// "java" themselves - hybrid servers restart that way after installing their
// libraries - and mcrun's own Java is normally not on PATH at all, so without
// this they fail or end up on whichever other Java the system has.
// A bare command name leaves the environment untouched.
func JavaEnvironment(javaBinary string) []string {
	return javaEnvironment(os.Environ(), javaBinary)
}

func javaEnvironment(environ []string, javaBinary string) []string {
	if !strings.ContainsRune(javaBinary, filepath.Separator) {
		return environ
	}

	binDir := filepath.Dir(javaBinary)

	env := make([]string, 0, len(environ)+2)
	path := ""

	for _, entry := range environ {
		key, value, _ := strings.Cut(entry, "=")

		switch {
		case envKeyEquals(key, "PATH"):
			path = value
		case envKeyEquals(key, "JAVA_HOME"):
		default:
			env = append(env, entry)
		}
	}

	if path == "" {
		path = binDir
	} else {
		path = binDir + string(os.PathListSeparator) + path
	}
	env = append(env, "PATH="+path)

	if filepath.Base(binDir) == "bin" {
		env = append(env, "JAVA_HOME="+filepath.Dir(binDir))
	}

	return env
}

// envKeyEquals compares environment variable names, which Windows treats case-insensitively
func envKeyEquals(key, name string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(key, name)
	}
	return key == name
}
