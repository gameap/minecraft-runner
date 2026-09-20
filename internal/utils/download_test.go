package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newFastClient() *HTTPClient {
	client := NewHTTPClient(false)
	client.retryWaitMin = time.Millisecond
	client.retryWaitMax = 5 * time.Millisecond
	return client
}

func TestGetSendsDescriptiveUserAgent(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	SetVersion("1.2.3")
	defer SetVersion("")

	if _, err := newFastClient().Get(context.Background(), server.URL); err != nil {
		t.Fatalf("Get: %v", err)
	}

	if want := "mcrun/1.2.3 (+" + projectURL + ")"; got != want {
		t.Errorf("User-Agent = %q, want %q", got, want)
	}
}

func TestGetRetriesTransientStatuses(t *testing.T) {
	for _, status := range []int{http.StatusBadGateway, http.StatusTooManyRequests} {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) < 3 {
				w.WriteHeader(status)
				return
			}
			fmt.Fprint(w, "ok")
		}))

		data, err := newFastClient().Get(context.Background(), server.URL)
		server.Close()

		if err != nil || string(data) != "ok" {
			t.Errorf("status %d: Get() = %q, %v", status, data, err)
		}
		if calls.Load() != 3 {
			t.Errorf("status %d: %d calls, want 3", status, calls.Load())
		}
	}
}

func TestGetDoesNotRetryClientErrors(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusGone)
	}))
	defer server.Close()

	_, err := newFastClient().Get(context.Background(), server.URL)

	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusGone {
		t.Errorf("err = %v, want a 410 StatusError", err)
	}
	if calls.Load() != 1 {
		t.Errorf("%d calls, want 1", calls.Load())
	}
}

func TestRetryWaitHonoursContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewHTTPClient(false)
	client.retryWaitMin = time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	started := time.Now()
	_, err := client.Get(ctx, server.URL)

	if err == nil {
		t.Fatal("expected an error")
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Errorf("Get returned after %s, the backoff ignored the context", elapsed)
	}
}

func TestDownloadFile(t *testing.T) {
	t.Run("a complete file lands at its destination", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "jar-content")
		}))
		defer server.Close()

		dest := filepath.Join(t.TempDir(), "sub", "server.jar")
		if err := newFastClient().DownloadFile(context.Background(), server.URL, dest); err != nil {
			t.Fatalf("DownloadFile: %v", err)
		}

		data, err := os.ReadFile(dest)
		if err != nil || string(data) != "jar-content" {
			t.Errorf("file = %q, %v", data, err)
		}
		if _, err := os.Stat(dest + ".tmp"); !os.IsNotExist(err) {
			t.Error("the temporary file must be gone")
		}
	})

	t.Run("an empty body is an error, not a server JAR", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/java-archive")
		}))
		defer server.Close()

		dest := filepath.Join(t.TempDir(), "server.jar")
		err := newFastClient().DownloadFile(context.Background(), server.URL, dest)

		if err == nil || !strings.Contains(err.Error(), "empty file") {
			t.Errorf("err = %v, want an empty file error", err)
		}
		for _, path := range []string{dest, dest + ".tmp"} {
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Errorf("%s must not be left behind", filepath.Base(path))
			}
		}
	})

	t.Run("an error status is reported", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer server.Close()

		err := newFastClient().DownloadFile(context.Background(), server.URL, filepath.Join(t.TempDir(), "server.jar"))

		var statusErr *StatusError
		if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusNotFound {
			t.Errorf("err = %v, want a 404 StatusError", err)
		}
	})
}

func TestStallReader(t *testing.T) {
	stalled := make(chan struct{})
	reader := newStallReader(strings.NewReader("data"), 20*time.Millisecond, func() { close(stalled) })
	defer reader.stop()

	buf := make([]byte, 4)
	if n, err := reader.Read(buf); n != 4 || err != nil {
		t.Fatalf("Read = %d, %v", n, err)
	}

	select {
	case <-stalled:
	case <-time.After(2 * time.Second):
		t.Error("a reader that delivers nothing more must be reported as stalled")
	}
}

func TestVerifyMD5(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := VerifyMD5(path, "5D41402ABC4B2A76B9719D911017C592"); err != nil {
		t.Errorf("VerifyMD5: %v", err)
	}
	if err := VerifyMD5(path, "00000000000000000000000000000000"); err == nil {
		t.Error("expected a mismatch")
	}
}
