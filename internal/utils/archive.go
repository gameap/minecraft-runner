package utils

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractTarGz extracts a .tar.gz archive to the destination directory
// Returns the path to the extracted root directory
func ExtractTarGz(archivePath string, destDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	var rootDir string

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar header: %w", err)
		}

		// Clean the path to prevent directory traversal
		cleanName := filepath.Clean(header.Name)
		if strings.Contains(cleanName, "..") {
			continue
		}

		targetPath := filepath.Join(destDir, cleanName)

		// Track the root directory
		if rootDir == "" {
			parts := strings.Split(cleanName, string(filepath.Separator))
			if len(parts) > 0 {
				rootDir = filepath.Join(destDir, parts[0])
			}
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return "", fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return "", fmt.Errorf("failed to create parent directory: %w", err)
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return "", fmt.Errorf("failed to create file: %w", err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return "", fmt.Errorf("failed to write file: %w", err)
			}
			outFile.Close()

		case tar.TypeSymlink:
			// Handle symlinks
			// Remove existing file/symlink if it exists to allow re-extraction
			os.Remove(targetPath)
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				// Ignore symlink errors on Windows
				if !isWindows() {
					return "", fmt.Errorf("failed to create symlink: %w", err)
				}
			}
		}
	}

	return rootDir, nil
}

// ExtractZip extracts a .zip archive to the destination directory
// Returns the path to the extracted root directory
func ExtractZip(archivePath string, destDir string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to open zip: %w", err)
	}
	defer reader.Close()

	var rootDir string

	for _, file := range reader.File {
		// Clean the path to prevent directory traversal
		cleanName := filepath.Clean(file.Name)
		if strings.Contains(cleanName, "..") {
			continue
		}

		targetPath := filepath.Join(destDir, cleanName)

		// Track the root directory
		if rootDir == "" {
			parts := strings.Split(cleanName, string(filepath.Separator))
			if len(parts) > 0 && parts[0] != "" {
				rootDir = filepath.Join(destDir, parts[0])
			}
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return "", fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return "", fmt.Errorf("failed to create parent directory: %w", err)
		}

		srcFile, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("failed to open file in zip: %w", err)
		}

		outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			srcFile.Close()
			return "", fmt.Errorf("failed to create file: %w", err)
		}

		_, err = io.Copy(outFile, srcFile)
		srcFile.Close()
		outFile.Close()

		if err != nil {
			return "", fmt.Errorf("failed to write file: %w", err)
		}
	}

	return rootDir, nil
}
