//go:build !windows

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const (
	helperModeEnv   = "MCRUN_TEST_HELPER"
	helperMarkerEnv = "MCRUN_TEST_HELPER_DIR"
)

// TestMain lets the test binary stand in for the Java server: started with
// helperModeEnv set, it behaves like a server process instead of running tests
func TestMain(m *testing.M) {
	switch os.Getenv(helperModeEnv) {
	case "":
		os.Exit(m.Run())
	case "graceful":
		runGracefulHelper()
	case "exit3":
		os.Exit(3)
	}
}

// runGracefulHelper mimics a Minecraft server: it saves its "world" when asked to stop
func runGracefulHelper() {
	dir := os.Getenv(helperMarkerEnv)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)

	os.WriteFile(filepath.Join(dir, "ready"), nil, 0644)

	<-signals

	os.WriteFile(filepath.Join(dir, "world-saved"), nil, 0644)
	os.Exit(143)
}

func TestStartStopsTheServerGracefully(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(helperModeEnv, "graceful")
	t.Setenv(helperMarkerEnv, dir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(filepath.Join(dir, "ready")); err == nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		cancel()
	}()

	runner := &Runner{}

	done := make(chan error, 1)
	go func() { done <- runner.start(ctx, os.Args[0], dir, nil) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("start() = %v, a requested stop is not an error", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the server was not stopped")
	}

	if _, err := os.Stat(filepath.Join(dir, "world-saved")); err != nil {
		t.Error("the server was killed before it could save: SIGTERM never reached it")
	}
}

func TestStartReportsTheExitCode(t *testing.T) {
	t.Setenv(helperModeEnv, "exit3")

	err := (&Runner{}).start(context.Background(), os.Args[0], t.TempDir(), nil)

	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 3 {
		t.Errorf("start() = %v, want exit code 3", err)
	}
	if want := fmt.Sprintf("server exited with code %d", 3); err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}
