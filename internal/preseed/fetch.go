// Package preseed puts in place the files a server or its installer would
// otherwise download on its own: the vanilla server a patcher works on,
// libraries, modules. mcrun downloads them through its own client, which falls
// back to GameAP's mirrors, and checks every checksum, so the server process
// needs no network.
package preseed

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gameap/minecraft-runner/internal/utils"
)

// parallel is how many files are downloaded at once
const parallel = 6

// File is one file a server would download itself
type File struct {
	// Path is where the server looks for the file: relative to the server
	// directory, with forward slashes
	Path string
	// URLs are tried in order
	URLs []string

	SHA512 string
	SHA256 string
	SHA1   string
	MD5    string
	// AnySHA1 accepts a file whose SHA-1 is any of these, as legacy Forge
	// installers do
	AnySHA1 []string

	// Sidecar names the checksum file published next to the download
	// ("sha1", "sha256") for files whose checksum is not known in advance. A
	// repository without one is trusted.
	Sidecar string
}

type checksum struct {
	name   string
	value  string
	verify func(path, value string) error
}

// checksum is the strongest checksum known for the file
func (f File) checksum() (checksum, bool) {
	switch {
	case f.SHA512 != "":
		return checksum{"sha512", f.SHA512, utils.VerifySHA512}, true
	case f.SHA256 != "":
		return checksum{"sha256", f.SHA256, utils.VerifySHA256}, true
	case f.SHA1 != "":
		return checksum{"sha1", f.SHA1, utils.VerifySHA1}, true
	case f.MD5 != "":
		return checksum{"md5", f.MD5, utils.VerifyMD5}, true
	case len(f.AnySHA1) > 0:
		return checksum{"sha1", strings.Join(f.AnySHA1, " or "), verifyAnySHA1(f.AnySHA1)}, true
	}
	return checksum{}, false
}

func verifyAnySHA1(accepted []string) func(path, value string) error {
	return func(path, _ string) error {
		actual, err := utils.CalculateSHA1(path)
		if err != nil {
			return err
		}
		for _, sum := range accepted {
			if strings.EqualFold(strings.TrimSpace(sum), actual) {
				return nil
			}
		}
		return fmt.Errorf("SHA1 mismatch: expected one of %s, got %s", strings.Join(accepted, ", "), actual)
	}
}

// Ensure puts files into dir. A file already there with the right checksum is
// kept, so a prepared server costs no requests; what is a name for the
// messages.
func Ensure(ctx context.Context, client *utils.HTTPClient, dir, what string, files []File) error {
	missing := make([]File, 0, len(files))
	seen := make(map[string]bool, len(files))

	for _, f := range files {
		dest, err := Resolve(dir, f.Path)
		if err != nil {
			return err
		}
		if seen[dest] {
			continue
		}
		seen[dest] = true

		if !present(dest, f) {
			missing = append(missing, f)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	if len(missing) == 1 {
		fmt.Printf("Downloading %s for %s...\n", missing[0].Path, what)
	} else {
		fmt.Printf("Downloading %d files for %s...\n", len(missing), what)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	queue := make(chan File)
	for range min(parallel, len(missing)) {
		wg.Go(func() {
			for f := range queue {
				if err := fetch(ctx, client, dir, f); err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
					cancel()
				}
			}
		})
	}

	for _, f := range missing {
		select {
		case queue <- f:
		case <-ctx.Done():
		}
	}
	close(queue)
	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("failed to prepare %s: %w", what, errors.Join(errs...))
	}
	// Cancelled before the workers got every file
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("failed to prepare %s: %w", what, err)
	}

	return nil
}

// Resolve maps a server-relative path from metadata onto dir and refuses
// paths that leave it
func Resolve(dir, rel string) (string, error) {
	clean := path.Clean(rel)
	if rel == "" || strings.ContainsAny(rel, `\:`) || path.IsAbs(rel) || filepath.IsAbs(rel) ||
		clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("refusing to write %q outside the server directory", rel)
	}

	return filepath.Join(dir, filepath.FromSlash(clean)), nil
}

// pathExists tells whether anything is at the server-relative path rel
func pathExists(dir, rel string) (bool, error) {
	dest, err := Resolve(dir, rel)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(dest)
	return err == nil, nil
}

// present tells whether dest holds the file. Without a known checksum any file
// counts, so it is not downloaded on every start.
func present(dest string, f File) bool {
	info, err := os.Stat(dest)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}

	sum, ok := f.checksum()
	if !ok {
		return true
	}

	return sum.verify(dest, sum.value) == nil
}

func fetch(ctx context.Context, client *utils.HTTPClient, dir string, f File) error {
	dest, err := Resolve(dir, f.Path)
	if err != nil {
		return err
	}

	part := dest + ".part"
	failures := make([]string, 0, len(f.URLs))

	for _, url := range f.URLs {
		sum, known := f.checksum()
		if !known && f.Sidecar != "" {
			sum, known, err = sidecar(ctx, client, url, f.Sidecar)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				failures = append(failures, err.Error())
				continue
			}
		}

		if err := client.DownloadFile(ctx, url, part); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			failures = append(failures, err.Error())
			continue
		}

		if known {
			if err := sum.verify(part, sum.value); err != nil {
				os.Remove(part)
				failures = append(failures, fmt.Sprintf("%s: %v", url, err))
				continue
			}
		}

		if err := os.Rename(part, dest); err != nil {
			os.Remove(part)
			return fmt.Errorf("%s: %w", f.Path, err)
		}

		return nil
	}

	if len(failures) == 0 {
		return fmt.Errorf("%s: nowhere to download it from", f.Path)
	}

	return fmt.Errorf("%s: %s", f.Path, strings.Join(failures, "; "))
}

// sidecar reads the checksum published as <url>.<algorithm>
func sidecar(ctx context.Context, client *utils.HTTPClient, url, algorithm string) (checksum, bool, error) {
	data, err := client.Get(ctx, url+"."+algorithm)
	if err != nil {
		var statusErr *utils.StatusError
		if errors.As(err, &statusErr) && statusErr.StatusCode == 404 {
			return checksum{}, false, nil
		}
		return checksum{}, false, err
	}

	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return checksum{}, false, fmt.Errorf("%s.%s is empty", url, algorithm)
	}

	f := File{}
	switch algorithm {
	case "sha512":
		f.SHA512 = fields[0]
	case "sha256":
		f.SHA256 = fields[0]
	case "sha1":
		f.SHA1 = fields[0]
	case "md5":
		f.MD5 = fields[0]
	default:
		return checksum{}, false, fmt.Errorf("unknown checksum %q", algorithm)
	}

	sum, _ := f.checksum()

	return sum, true, nil
}
