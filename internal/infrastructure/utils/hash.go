package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// CalculateSHA256 calculates the SHA256 hash of the data from the reader.
func CalculateSHA256(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CalculateSHA256Bytes calculates the SHA256 hash of the byte slice.
func CalculateSHA256Bytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
