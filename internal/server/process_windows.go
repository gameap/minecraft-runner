//go:build windows

package server

import "os"

// terminateProcess has nothing to deliver on Windows: a console control event
// reaches every process attached to the console, so the JVM is already
// shutting down by the time mcrun learns about the stop request. The runner
// only has to wait for it.
func terminateProcess(_ *os.Process) error {
	return nil
}
