package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// defaultMirrors are GameAP's mirrors of everything mcrun downloads. They are
// asked only when the upstream itself cannot be reached.
var defaultMirrors = []string{"https://mirror.gameap.ru"}

// mirrorCacheHeader is set by the mirror on every answer it got from or for
// the upstream, which tells an upstream 404 from a path it does not mirror
const mirrorCacheHeader = "X-Mirror-Cache"

// mirrorAttempts is how often each mirror is tried
const mirrorAttempts = 2

// upstreamSource is the source of an answer that came from the upstream
// itself rather than from a mirror
const upstreamSource = -1

var (
	mirrorsMu sync.RWMutex
	mirrors   = defaultMirrors

	// downHosts remembers the upstreams that could not be reached during this
	// run, so later requests to them go to the mirrors without waiting again
	downHosts sync.Map

	// warnedHosts makes the fallback warning appear once per host
	warnedHosts sync.Map
)

// SetMirrors replaces the mirror list; an empty list disables the fallback
func SetMirrors(list []string) {
	cleaned := make([]string, 0, len(list))
	for _, m := range list {
		m = strings.TrimRight(strings.TrimSpace(m), "/")
		if m == "" {
			continue
		}
		if u, err := url.Parse(m); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			fmt.Printf("[WARNING] Ignoring mirror %q: not an http(s) URL\n", m)
			continue
		}
		cleaned = append(cleaned, m)
	}

	mirrorsMu.Lock()
	mirrors = cleaned
	mirrorsMu.Unlock()
}

// ConfigureMirrors applies the MCRUN_MIRRORS environment variable: a comma
// separated list of mirror URLs, or "off". Unset keeps GameAP's mirrors.
func ConfigureMirrors(env string, set bool) {
	if !set {
		return
	}

	if strings.EqualFold(strings.TrimSpace(env), "off") {
		SetMirrors(nil)
		return
	}

	SetMirrors(strings.Split(env, ","))
}

func currentMirrors() []string {
	mirrorsMu.RLock()
	defer mirrorsMu.RUnlock()

	return mirrors
}

// canMirror tells whether raw can be asked from a mirror at all
func canMirror(raw string) bool {
	if len(currentMirrors()) == 0 {
		return false
	}
	_, ok := mirrorURL("", raw)
	return ok
}

// mirrorURL maps an upstream URL onto a mirror: https://host/path?q becomes
// <mirror>/host/path?q. It reports false for URLs that cannot be mirrored.
func mirrorURL(base, raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", false
	}

	for _, m := range currentMirrors() {
		if strings.HasPrefix(raw, m+"/") {
			return "", false
		}
	}

	mirrored := base + "/" + u.Host + u.EscapedPath()
	if u.RawQuery != "" {
		mirrored += "?" + u.RawQuery
	}

	return mirrored, true
}

// isConnectError reports a failure to reach the host at all: name
// resolution or connecting, as opposed to a broken exchange with a host that
// answered
func isConnectError(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	var opErr *net.OpError

	return errors.As(err, &opErr) && opErr.Op == "dial"
}

// blockedStatus is an answer of a host that refuses to serve this client
// rather than one that lacks the file
func blockedStatus(code int) bool {
	return code == http.StatusForbidden || code == http.StatusUnavailableForLegalReasons
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		return u.Host
	}
	return ""
}

func isHostDown(raw string) bool {
	_, down := downHosts.Load(hostOf(raw))
	return down
}

func markHostDown(raw string) {
	if host := hostOf(raw); host != "" {
		downHosts.Store(host, struct{}{})
	}
}

// warnOnce prints a warning about host once per run
func warnOnce(host, format string, args ...any) {
	if _, warned := warnedHosts.LoadOrStore(host, struct{}{}); warned {
		return
	}

	fmt.Printf("[WARNING] "+format+"\n", args...)
}

// retryAfterBreak tells where to ask again after err: an answer that broke off
// midway is asked from the mirror after the source that sent it, while there
// is one. An upstream that broke off gets the rest of this run's requests
// sent to the mirrors.
func retryAfterBreak(ctx context.Context, raw string, source int, err error) (int, bool) {
	if err == nil || ctx.Err() != nil || !isBrokenBody(err) || !canMirror(raw) {
		return 0, false
	}

	mirrorList := currentMirrors()
	next := source + 1
	if next >= len(mirrorList) {
		return 0, false
	}

	if source == upstreamSource {
		host := hostOf(raw)
		warnOnce(host, "Download from %s broke off (%v), retrying through a mirror", host, err)
		markHostDown(raw)
	} else {
		fmt.Printf("[WARNING] Download of %s through %s broke off (%v), trying the next mirror\n",
			raw, mirrorList[source], err)
	}

	return next, true
}

func warnFallback(raw string, cause error, mirror string) {
	host := hostOf(raw)
	warnOnce(host, "%s is unreachable (%v), downloading through %s", host, cause, mirror)
}

// fetch performs a GET: first from the upstream, then, when the upstream
// cannot be reached, from each mirror. A host already found unreachable in
// this run goes to the mirrors first and gets its own chance only when they
// fail. From the index of a mirror on, only that mirror and the ones after it
// are asked. budget bounds each request phase (0: no bound); the response body
// keeps its phase's deadline until it is closed. source is upstreamSource or
// the index of the mirror that answered.
func (c *HTTPClient) fetch(
	ctx context.Context, raw string, budget time.Duration, from int,
) (resp *http.Response, source int, err error) {
	mirrorOnly := from != upstreamSource

	var mirrorList []string
	if canMirror(raw) {
		mirrorList = currentMirrors()
	}

	if mirrorOnly && from >= len(mirrorList) {
		return nil, upstreamSource, errors.New("no mirror to retry through")
	}
	if len(mirrorList) == 0 {
		resp, err = c.phase(ctx, budget, func(ctx context.Context) (*http.Response, error) {
			return c.doWithRetry(ctx, raw, c.retryMax, false)
		})
		return resp, upstreamSource, err
	}

	skipDirect := mirrorOnly || isHostDown(raw)

	var directErr error
	if skipDirect {
		directErr = fmt.Errorf("%s was unreachable earlier in this run", hostOf(raw))
	} else {
		resp, err = c.direct(ctx, raw, budget)
		if err == nil || ctx.Err() != nil || !shouldFallback(err) {
			return resp, upstreamSource, err
		}
		markHostDown(raw)
		directErr = err
	}

	first := max(from, 0)
	resp, answered, failures, err := c.viaMirrors(ctx, raw, mirrorList[first:], budget, directErr)
	if resp != nil || err != nil {
		return resp, first + answered, err
	}
	if ctx.Err() != nil {
		return nil, upstreamSource, ctx.Err()
	}

	if skipDirect && !mirrorOnly {
		resp, err = c.direct(ctx, raw, budget)
		return resp, upstreamSource, err
	}

	return nil, upstreamSource, fmt.Errorf("%w; mirrors failed too: %s", directErr, strings.Join(failures, "; "))
}

// direct asks the upstream. A host that cannot be reached at all is not
// asked again, the mirrors are; a broken exchange gets one more try.
func (c *HTTPClient) direct(ctx context.Context, raw string, budget time.Duration) (*http.Response, error) {
	resp, err := c.phase(ctx, budget, func(ctx context.Context) (*http.Response, error) {
		return c.doWithRetry(ctx, raw, c.retryMax, true)
	})
	if err != nil {
		return nil, err
	}

	if blockedStatus(resp.StatusCode) {
		resp.Body.Close()
		return nil, &StatusError{StatusCode: resp.StatusCode, URL: raw}
	}

	return resp, nil
}

// shouldFallback tells the failures a mirror can help with: the host was
// unreachable, kept failing, or refused this client
func shouldFallback(err error) bool {
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode >= 500 || statusErr.StatusCode == http.StatusTooManyRequests ||
			blockedStatus(statusErr.StatusCode)
	}
	return true
}

// viaMirrors asks each mirror in turn. It returns the first good answer, or
// a 404 error when a mirror reports that the upstream has no such file, with
// the index of that mirror, or neither, with what went wrong on each mirror.
func (c *HTTPClient) viaMirrors(
	ctx context.Context, raw string, mirrorList []string, budget time.Duration, directErr error,
) (*http.Response, int, []string, error) {
	failures := make([]string, 0, len(mirrorList))

	for i, base := range mirrorList {
		target, _ := mirrorURL(base, raw)

		resp, err := c.phase(ctx, budget, func(ctx context.Context) (*http.Response, error) {
			return c.doWithRetry(ctx, target, mirrorAttempts-1, true)
		})
		if err != nil {
			if ctx.Err() != nil {
				return nil, 0, nil, nil
			}
			failures = append(failures, fmt.Sprintf("%s: %v", base, err))
			continue
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			warnFallback(raw, directErr, base)
			return resp, i, nil, nil
		case resp.StatusCode == http.StatusNotFound && resp.Header.Get(mirrorCacheHeader) != "":
			resp.Body.Close()
			warnFallback(raw, directErr, base)
			return nil, i, nil, &StatusError{StatusCode: resp.StatusCode, URL: raw}
		default:
			resp.Body.Close()
			failures = append(failures, fmt.Sprintf("%s: status %d", base, resp.StatusCode))
		}
	}

	return nil, 0, failures, nil
}

// phase runs one request phase under its own deadline. The deadline stays
// with the response body and ends when the body is closed.
func (c *HTTPClient) phase(
	ctx context.Context, budget time.Duration, do func(context.Context) (*http.Response, error),
) (*http.Response, error) {
	if budget <= 0 {
		return do(ctx)
	}

	ctx, cancel := context.WithTimeout(ctx, budget)

	resp, err := do(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}

	return resp, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}
