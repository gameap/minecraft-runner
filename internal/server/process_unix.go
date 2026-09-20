//go:build !windows

package server

import (
	"os"
	"syscall"
)

// terminateProcess asks the server to stop. The JVM runs the shutdown hook of
// the Minecraft server on SIGTERM, which saves the world before exiting.
func terminateProcess(process *os.Process) error {
	return process.Signal(syscall.SIGTERM)
}
