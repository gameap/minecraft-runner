package utils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// VerifySHA256 verifies that a file matches the expected SHA256 hash
func VerifySHA256(filePath string, expectedHash string) error {
	return verifyHash("SHA256", sha256.New(), filePath, expectedHash)
}

// CalculateSHA256 calculates the SHA256 hash of a file
func CalculateSHA256(filePath string) (string, error) {
	return calculateHash(sha256.New(), filePath)
}

// VerifySHA1 verifies that a file matches the expected SHA1 hash
func VerifySHA1(filePath string, expectedHash string) error {
	return verifyHash("SHA1", sha1.New(), filePath, expectedHash)
}

// CalculateSHA1 calculates the SHA1 hash of a file
func CalculateSHA1(filePath string) (string, error) {
	return calculateHash(sha1.New(), filePath)
}

// VerifyMD5 verifies that a file matches the expected MD5 hash
func VerifyMD5(filePath string, expectedHash string) error {
	return verifyHash("MD5", md5.New(), filePath, expectedHash)
}

func verifyHash(name string, hasher hash.Hash, filePath string, expectedHash string) error {
	actual, err := calculateHash(hasher, filePath)
	if err != nil {
		return err
	}

	expectedHash = strings.ToLower(strings.TrimSpace(expectedHash))
	if actual != expectedHash {
		return fmt.Errorf("%s mismatch: expected %s, got %s", name, expectedHash, actual)
	}

	return nil
}

func calculateHash(hasher hash.Hash, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
