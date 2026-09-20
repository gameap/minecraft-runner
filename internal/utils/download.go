package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	projectURL = "https://github.com/gameap/minecraft-runner"

	// requestTimeout bounds a whole API request, body included
	requestTimeout = 30 * time.Second

	// headerTimeout bounds the wait for response headers; downloads have no
	// overall deadline, because a large file on a slow link takes as long as it takes
	headerTimeout = 30 * time.Second

	// stallTimeout aborts a download that stopped delivering data
	stallTimeout = 60 * time.Second
)

// userAgent identifies mcrun to the download APIs. PaperMC asks for a
// descriptive agent with a contact URL and may block generic ones.
var userAgent = fmt.Sprintf("mcrun/dev (+%s)", projectURL)

// SetVersion puts the mcrun version into the User-Agent of every request
func SetVersion(version string) {
	if version == "" {
		version = "dev"
	}
	userAgent = fmt.Sprintf("mcrun/%s (+%s)", version, projectURL)
}

// HTTPClient is a wrapper around http.Client with retry and progress support
type HTTPClient struct {
	client       *http.Client
	showProgress bool
	retryMax     int
	retryWaitMin time.Duration
	retryWaitMax time.Duration
}

// NewHTTPClient creates a new HTTP client with retry support
func NewHTTPClient(showProgress bool) *HTTPClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = headerTimeout

	return &HTTPClient{
		client:       &http.Client{Transport: transport},
		showProgress: showProgress,
		retryMax:     3,
		retryWaitMin: 1 * time.Second,
		retryWaitMax: 30 * time.Second,
	}
}

// StatusError is returned when a server answers with an unexpected HTTP status
type StatusError struct {
	StatusCode int
	URL        string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected status code %d for %s", e.StatusCode, e.URL)
}

// doWithRetry performs a GET request, retrying network errors, 5xx and 429
func (c *HTTPClient) doWithRetry(ctx context.Context, url string) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.retryMax; attempt++ {
		if attempt > 0 {
			backoff := c.retryWaitMin * time.Duration(1<<uint(attempt-1))
			if backoff > c.retryWaitMax {
				backoff = c.retryWaitMax
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := c.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
			continue
		}

		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			lastErr = &StatusError{StatusCode: resp.StatusCode, URL: url}
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.retryMax, lastErr)
}

// Get performs a GET request and returns the response body
func (c *HTTPClient) Get(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.doWithRetry(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{StatusCode: resp.StatusCode, URL: url}
	}

	return io.ReadAll(resp.Body)
}

// DownloadFile downloads a file to the specified path with progress bar
func (c *HTTPClient) DownloadFile(ctx context.Context, url string, destPath string) (err error) {
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	tmpPath := destPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		file.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	resp, err := c.doWithRetry(ctx, url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &StatusError{StatusCode: resp.StatusCode, URL: url}
	}

	var writer io.Writer = file

	if c.showProgress && resp.ContentLength > 0 {
		writer = &progressWriter{
			total:    resp.ContentLength,
			filename: filepath.Base(destPath),
			writer:   file,
		}
	}

	body := newStallReader(resp.Body, stallTimeout, func() {
		cancel(fmt.Errorf("no data received for %s", stallTimeout))
	})
	defer body.stop()

	written, err := io.Copy(writer, body)
	if c.showProgress {
		fmt.Println()
	}
	if err != nil {
		if cause := context.Cause(ctx); cause != nil && !errors.Is(cause, context.Canceled) {
			err = cause
		}
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Not every provider publishes a checksum, and a broken mirror may well
	// answer 200 with an empty or cut-off body
	if written == 0 {
		return fmt.Errorf("the server sent an empty file for %s", url)
	}
	if resp.ContentLength > 0 && written != resp.ContentLength {
		return fmt.Errorf("incomplete download of %s: got %d of %d bytes", url, written, resp.ContentLength)
	}

	if err = file.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	if err = os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// FileExists checks if a file exists at the given path
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// stallReader calls onStall when the wrapped reader delivers nothing for the
// given timeout, which is how a dead connection is told from a slow one
type stallReader struct {
	reader  io.Reader
	timeout time.Duration
	timer   *time.Timer
}

func newStallReader(reader io.Reader, timeout time.Duration, onStall func()) *stallReader {
	return &stallReader{
		reader:  reader,
		timeout: timeout,
		timer:   time.AfterFunc(timeout, onStall),
	}
}

func (r *stallReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.timer.Reset(r.timeout)
	}
	return n, err
}

func (r *stallReader) stop() {
	r.timer.Stop()
}

// progressWriter is a simple progress indicator for downloads
type progressWriter struct {
	total    int64
	written  int64
	filename string
	writer   io.Writer
	lastPct  int
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	pw.written += int64(n)

	pct := int(float64(pw.written) / float64(pw.total) * 100)
	if pct != pw.lastPct && pct%10 == 0 {
		fmt.Printf("\r%s: %d%%", pw.filename, pct)
		pw.lastPct = pct
	}

	return n, err
}
