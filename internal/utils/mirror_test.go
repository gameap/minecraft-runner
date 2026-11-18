package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

// No test may reach GameAP's real mirrors
func TestMain(m *testing.M) {
	SetMirrors(nil)
	os.Exit(m.Run())
}

// useMirrors installs mirrors for one test and forgets the hosts earlier tests
// found unreachable
func useMirrors(t *testing.T, list ...string) {
	t.Helper()

	reset := func() {
		downHosts.Clear()
		warnedHosts.Clear()
	}

	reset()
	SetMirrors(list)
	t.Cleanup(func() {
		SetMirrors(nil)
		reset()
	})
}

// counted serves handler and counts the requests it gets
type counted struct {
	*httptest.Server
	calls atomic.Int32
}

func newCounted(t *testing.T, handler http.HandlerFunc) *counted {
	t.Helper()

	c := &counted{}
	c.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.calls.Add(1)
		handler(w, r)
	}))
	t.Cleanup(c.Close)

	return c
}

// mirrorOf serves body for every request of upstream's host, the way
// gameap-mirror does
func mirrorOf(t *testing.T, body string) *counted {
	t.Helper()

	return newCounted(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(mirrorCacheHeader, "MISS")
		fmt.Fprint(w, body)
	})
}

func status(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
	}
}

func closedPortURL(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	return "http://" + addr
}

func TestMirrorURL(t *testing.T) {
	useMirrors(t, "https://mirror.example")

	for raw, want := range map[string]string{
		"https://meta.fabricmc.net/v2/versions/game":                                                  "https://m/meta.fabricmc.net/v2/versions/game",
		"https://ci.pufferfish.host/api/json?tree=jobs[name]":                                         "https://m/ci.pufferfish.host/api/json?tree=jobs[name]",
		"https://github.com/adoptium/temurin21-binaries/releases/download/jdk-21.0.13%2B9/jre.tar.gz": "https://m/github.com/adoptium/temurin21-binaries/releases/download/jdk-21.0.13%2B9/jre.tar.gz",
	} {
		if got, ok := mirrorURL("https://m", raw); !ok || got != want {
			t.Errorf("mirrorURL(%s) = %q, %v, want %q", raw, got, ok, want)
		}
	}

	for _, raw := range []string{"ftp://host/file", "/relative/path", "https://mirror.example/meta.fabricmc.net/x"} {
		if got, ok := mirrorURL("https://m", raw); ok {
			t.Errorf("mirrorURL(%s) = %q, must not be mirrored", raw, got)
		}
	}
}

func TestConfigureMirrors(t *testing.T) {
	useMirrors(t, defaultMirrors...)

	ConfigureMirrors("", false)
	if got := currentMirrors(); !reflect.DeepEqual(got, defaultMirrors) {
		t.Errorf("unset: %v, want the defaults", got)
	}

	ConfigureMirrors(" https://a.example/ , ftp://b.example,http://c.example:8437", true)
	if got, want := currentMirrors(), []string{"https://a.example", "http://c.example:8437"}; !reflect.DeepEqual(got, want) {
		t.Errorf("list: %v, want %v", got, want)
	}

	ConfigureMirrors("OFF", true)
	if got := currentMirrors(); len(got) != 0 {
		t.Errorf("off: %v, want none", got)
	}
}

func TestGetFallsBackWhenUpstreamIsUnreachable(t *testing.T) {
	mirror := mirrorOf(t, "from-mirror")
	useMirrors(t, mirror.URL)

	upstream := closedPortURL(t)

	for range 2 {
		data, err := newFastClient().Get(context.Background(), upstream+"/v2/versions/game?x=1")
		if err != nil || string(data) != "from-mirror" {
			t.Fatalf("Get = %q, %v", data, err)
		}
	}

	if mirror.calls.Load() != 2 {
		t.Errorf("mirror got %d requests, want 2", mirror.calls.Load())
	}
}

func TestGetSendsTheUpstreamPathAndQueryToTheMirror(t *testing.T) {
	var got string
	mirror := newCounted(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RequestURI()
		fmt.Fprint(w, "ok")
	})
	useMirrors(t, mirror.URL+"/")

	upstream := closedPortURL(t)
	if _, err := newFastClient().Get(context.Background(), upstream+"/api/json?tree=jobs%5Bname%5D"); err != nil {
		t.Fatal(err)
	}

	if want := "/" + strings.TrimPrefix(upstream, "http://") + "/api/json?tree=jobs%5Bname%5D"; got != want {
		t.Errorf("mirror asked for %q, want %q", got, want)
	}
}

func TestGetFallsBackAfterServerErrors(t *testing.T) {
	upstream := newCounted(t, status(http.StatusServiceUnavailable))
	mirror := mirrorOf(t, "from-mirror")
	useMirrors(t, mirror.URL)

	client := newFastClient()

	data, err := client.Get(context.Background(), upstream.URL+"/file")
	if err != nil || string(data) != "from-mirror" {
		t.Fatalf("Get = %q, %v", data, err)
	}
	if want := int32(client.retryMax + 1); upstream.calls.Load() != want {
		t.Errorf("upstream got %d requests, want %d", upstream.calls.Load(), want)
	}

	// The host failed once in this run, the next request goes to the mirror
	if _, err := client.Get(context.Background(), upstream.URL+"/other"); err != nil {
		t.Fatal(err)
	}
	if want := int32(client.retryMax + 1); upstream.calls.Load() != want {
		t.Errorf("upstream got %d requests, want still %d", upstream.calls.Load(), want)
	}
}

func TestGetFallsBackWhenBlocked(t *testing.T) {
	upstream := newCounted(t, status(http.StatusForbidden))
	mirror := mirrorOf(t, "from-mirror")
	useMirrors(t, mirror.URL)

	data, err := newFastClient().Get(context.Background(), upstream.URL+"/file")
	if err != nil || string(data) != "from-mirror" {
		t.Fatalf("Get = %q, %v", data, err)
	}
	if upstream.calls.Load() != 1 {
		t.Errorf("upstream got %d requests, want 1", upstream.calls.Load())
	}
}

func TestGetDoesNotFallBackOnNotFound(t *testing.T) {
	upstream := newCounted(t, status(http.StatusNotFound))
	mirror := mirrorOf(t, "from-mirror")
	useMirrors(t, mirror.URL)

	_, err := newFastClient().Get(context.Background(), upstream.URL+"/file")

	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusNotFound {
		t.Errorf("err = %v, want a 404 StatusError", err)
	}
	if mirror.calls.Load() != 0 {
		t.Errorf("mirror got %d requests, want none", mirror.calls.Load())
	}
}

func TestGetTriesTheNextMirror(t *testing.T) {
	notMirrored := newCounted(t, status(http.StatusNotFound))
	failing := newCounted(t, status(http.StatusBadGateway))
	good := mirrorOf(t, "from-third")
	useMirrors(t, notMirrored.URL, failing.URL, good.URL)

	data, err := newFastClient().Get(context.Background(), closedPortURL(t)+"/file")
	if err != nil || string(data) != "from-third" {
		t.Fatalf("Get = %q, %v", data, err)
	}
	if notMirrored.calls.Load() != 1 || failing.calls.Load() != mirrorAttempts {
		t.Errorf("mirrors got %d and %d requests, want 1 and %d",
			notMirrored.calls.Load(), failing.calls.Load(), mirrorAttempts)
	}
}

func TestGetTrustsTheMirrorsNotFound(t *testing.T) {
	missing := newCounted(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(mirrorCacheHeader, "MISS")
		w.WriteHeader(http.StatusNotFound)
	})
	second := mirrorOf(t, "never")
	useMirrors(t, missing.URL, second.URL)

	_, err := newFastClient().Get(context.Background(), closedPortURL(t)+"/file")

	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusNotFound {
		t.Errorf("err = %v, want a 404 StatusError", err)
	}
	if second.calls.Load() != 0 {
		t.Error("the upstream has no such file, no other mirror is asked")
	}
}

func TestGetReportsTheUpstreamErrorWhenMirrorsFail(t *testing.T) {
	upstream := newCounted(t, status(http.StatusServiceUnavailable))
	mirror := newCounted(t, status(http.StatusNotFound))
	useMirrors(t, mirror.URL)

	_, err := newFastClient().Get(context.Background(), upstream.URL+"/file")

	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("err = %v, want the upstream's 503", err)
	}
	if err == nil || !strings.Contains(err.Error(), "mirrors failed too") {
		t.Errorf("err = %v, want the mirror failures mentioned", err)
	}
}

func TestGetRetriesADownHostWhenMirrorsFail(t *testing.T) {
	var healthy atomic.Bool
	upstream := newCounted(t, func(w http.ResponseWriter, r *http.Request) {
		if !healthy.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, "upstream-back")
	})
	mirror := newCounted(t, status(http.StatusBadGateway))
	useMirrors(t, mirror.URL)

	client := newFastClient()
	if _, err := client.Get(context.Background(), upstream.URL+"/file"); err == nil {
		t.Fatal("expected an error while everything is down")
	}

	healthy.Store(true)

	data, err := client.Get(context.Background(), upstream.URL+"/file")
	if err != nil || string(data) != "upstream-back" {
		t.Errorf("Get = %q, %v", data, err)
	}
}

func TestGetDoesNotFallBackWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	upstream := newCounted(t, func(w http.ResponseWriter, r *http.Request) {
		cancel()
		<-r.Context().Done()
	})
	mirror := mirrorOf(t, "from-mirror")
	useMirrors(t, mirror.URL)

	if _, err := newFastClient().Get(ctx, upstream.URL+"/file"); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if mirror.calls.Load() != 0 {
		t.Errorf("mirror got %d requests, want none", mirror.calls.Load())
	}
}

// breaksOff sends the headers and part of body, then drops the connection
func breaksOff(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, body[:len(body)/2])
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	}
}

func TestGetRefetchesABrokenBodyThroughTheMirror(t *testing.T) {
	upstream := newCounted(t, breaksOff(`{"versions":[]}`))
	mirror := mirrorOf(t, `{"versions":[]}`)
	useMirrors(t, mirror.URL)

	data, err := newFastClient().Get(context.Background(), upstream.URL+"/versions")
	if err != nil || string(data) != `{"versions":[]}` {
		t.Fatalf("Get = %q, %v", data, err)
	}
}

func TestDownloadFileRestartsABrokenDownloadThroughTheMirror(t *testing.T) {
	upstream := newCounted(t, breaksOff("0123456789"))
	mirror := mirrorOf(t, "0123456789")
	useMirrors(t, mirror.URL)

	dest := filepath.Join(t.TempDir(), "server.jar")
	if err := newFastClient().DownloadFile(context.Background(), upstream.URL+"/server.jar", dest); err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil || string(data) != "0123456789" {
		t.Errorf("file = %q, %v", data, err)
	}
	if upstream.calls.Load() != 1 || mirror.calls.Load() != 1 {
		t.Errorf("upstream and mirror got %d and %d requests, want 1 and 1", upstream.calls.Load(), mirror.calls.Load())
	}
}

func TestGetMovesOnFromAMirrorThatBreaksOff(t *testing.T) {
	broken := newCounted(t, breaksOff(`{"versions":[]}`))
	mirror := mirrorOf(t, `{"versions":[]}`)
	useMirrors(t, broken.URL, mirror.URL)

	data, err := newFastClient().Get(context.Background(), closedPortURL(t)+"/versions")
	if err != nil || string(data) != `{"versions":[]}` {
		t.Fatalf("Get = %q, %v", data, err)
	}
	if broken.calls.Load() != 1 || mirror.calls.Load() != 1 {
		t.Errorf("mirrors got %d and %d requests, want 1 and 1", broken.calls.Load(), mirror.calls.Load())
	}
}

func TestDownloadFileMovesOnFromAMirrorThatBreaksOff(t *testing.T) {
	upstream := newCounted(t, breaksOff("0123456789"))
	broken := newCounted(t, breaksOff("0123456789"))
	mirror := mirrorOf(t, "0123456789")
	useMirrors(t, broken.URL, mirror.URL)

	dest := filepath.Join(t.TempDir(), "server.jar")
	if err := newFastClient().DownloadFile(context.Background(), upstream.URL+"/server.jar", dest); err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}

	if data, _ := os.ReadFile(dest); string(data) != "0123456789" {
		t.Errorf("file = %q", data)
	}
	calls := []int32{upstream.calls.Load(), broken.calls.Load(), mirror.calls.Load()}
	if calls[0] != 1 || calls[1] != 1 || calls[2] != 1 {
		t.Errorf("upstream and mirrors got %v requests, want one each", calls)
	}
}

func TestDownloadFileGivesUpWhenEveryMirrorBreaksOff(t *testing.T) {
	upstream := newCounted(t, breaksOff("0123456789"))
	first := newCounted(t, breaksOff("0123456789"))
	second := newCounted(t, breaksOff("0123456789"))
	useMirrors(t, first.URL, second.URL)

	dest := filepath.Join(t.TempDir(), "server.jar")
	if err := newFastClient().DownloadFile(context.Background(), upstream.URL+"/server.jar", dest); err == nil {
		t.Fatal("expected an error")
	}

	calls := []int32{upstream.calls.Load(), first.calls.Load(), second.calls.Load()}
	if calls[0] != 1 || calls[1] != 1 || calls[2] != 1 {
		t.Errorf("upstream and mirrors got %v requests, want one each", calls)
	}
	if _, err := os.Stat(dest + ".tmp"); !os.IsNotExist(err) {
		t.Error("the temporary file must be gone")
	}
}

func TestDownloadFileFallsBackWhenUpstreamIsUnreachable(t *testing.T) {
	mirror := mirrorOf(t, "jar-content")
	useMirrors(t, mirror.URL)

	dest := filepath.Join(t.TempDir(), "server.jar")
	if err := newFastClient().DownloadFile(context.Background(), closedPortURL(t)+"/server.jar", dest); err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}

	if data, _ := os.ReadFile(dest); string(data) != "jar-content" {
		t.Errorf("file = %q", data)
	}
}

func TestBrokenDownloadWithoutMirrorsFails(t *testing.T) {
	upstream := newCounted(t, breaksOff("0123456789"))

	dest := filepath.Join(t.TempDir(), "server.jar")
	err := newFastClient().DownloadFile(context.Background(), upstream.URL+"/server.jar", dest)
	if err == nil {
		t.Fatal("expected an error")
	}
	if _, statErr := os.Stat(dest + ".tmp"); !os.IsNotExist(statErr) {
		t.Error("the temporary file must be gone")
	}
}
