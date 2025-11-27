package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/schollz/progressbar/v3"
)

// HTTPClient is a wrapper around retryablehttp.Client with progress support
type HTTPClient struct {
	client       *retryablehttp.Client
	showProgress bool
}

// NewHTTPClient creates a new HTTP client with retry support
func NewHTTPClient(showProgress bool) *HTTPClient {
	client := retryablehttp.NewClient()
	client.RetryMax = 3
	client.RetryWaitMin = 1 * time.Second
	client.RetryWaitMax = 30 * time.Second
	client.Logger = nil // Disable logging

	return &HTTPClient{
		client:       client,
		showProgress: showProgress,
	}
}

// Get performs a GET request and returns the response body
func (c *HTTPClient) Get(ctx context.Context, url string) ([]byte, error) {
	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "mcrun/1.0")

	resp, err := c.client.Do(req)
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

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "mcrun/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var writer io.Writer = file

	if c.showProgress && resp.ContentLength > 0 {
		bar := progressbar.NewOptions64(
			resp.ContentLength,
			progressbar.OptionSetDescription(filepath.Base(destPath)),
			progressbar.OptionSetWidth(40),
			progressbar.OptionShowBytes(true),
			progressbar.OptionShowCount(),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "=",
				SaucerHead:    ">",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
			progressbar.OptionOnCompletion(func() {
				fmt.Println()
			}),
		)
		writer = io.MultiWriter(file, bar)
	}

	_, err = io.Copy(writer, resp.Body)
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
