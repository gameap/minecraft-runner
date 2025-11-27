package utils

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// VerifySHA256 verifies that a file matches the expected SHA256 hash
func VerifySHA256(filePath string, expectedHash string) error {
	hash, err := CalculateSHA256(filePath)
	if err != nil {
		return err
	}

	expectedHash = strings.ToLower(expectedHash)
	if hash != expectedHash {
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", expectedHash, hash)
	}

	return nil
}

// CalculateSHA256 calculates the SHA256 hash of a file
func CalculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// VerifySHA1 verifies that a file matches the expected SHA1 hash
func VerifySHA1(filePath string, expectedHash string) error {
	hash, err := CalculateSHA1(filePath)
	if err != nil {
		return err
	}

	expectedHash = strings.ToLower(expectedHash)
	if hash != expectedHash {
		return fmt.Errorf("SHA1 mismatch: expected %s, got %s", expectedHash, hash)
	}

	return nil
}

// CalculateSHA1 calculates the SHA1 hash of a file
func CalculateSHA1(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
