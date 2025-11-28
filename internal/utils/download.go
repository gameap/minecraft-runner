package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

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
	return &HTTPClient{
		client:       &http.Client{Timeout: 30 * time.Second},
		showProgress: showProgress,
		retryMax:     3,
		retryWaitMin: 1 * time.Second,
		retryWaitMax: 30 * time.Second,
	}
}

// doWithRetry performs an HTTP request with retry logic
func (c *HTTPClient) doWithRetry(req *http.Request) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retryMax; attempt++ {
		if attempt > 0 {
			// Calculate backoff with exponential increase
			backoff := c.retryWaitMin * time.Duration(1<<uint(attempt-1))
			if backoff > c.retryWaitMax {
				backoff = c.retryWaitMax
			}
			time.Sleep(backoff)
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Retry on 5xx errors
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}
	return nil, fmt.Errorf("request failed after %d retries: %w", c.retryMax, lastErr)
}

// Get performs a GET request and returns the response body
func (c *HTTPClient) Get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "mcrun/1.0")

	resp, err := c.doWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// DownloadFile downloads a file to the specified path with progress bar
func (c *HTTPClient) DownloadFile(ctx context.Context, url string, destPath string) error {
	// Ensure destination directory exists
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create temporary file for download
	tmpPath := destPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		file.Close()
		os.Remove(tmpPath) // Clean up on error
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "mcrun/1.0")

	resp, err := c.doWithRetry(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var writer io.Writer = file

	if c.showProgress && resp.ContentLength > 0 {
		pw := &progressWriter{
			total:    resp.ContentLength,
			filename: filepath.Base(destPath),
			writer:   file,
		}
		writer = pw
	}

	_, err = io.Copy(writer, resp.Body)
	if c.showProgress {
		fmt.Println() // newline after progress
	}
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Close file before renaming
	file.Close()

	// Rename temp file to final destination
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// FileExists checks if a file exists at the given path
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
