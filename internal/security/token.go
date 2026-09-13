package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
)

// GenerateToken creates a one-time pairing token from 32 random bytes encoded as hex.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ValidateFormat memastikan token berbentuk hex 64 karakter.
func ValidateFormat(token string) error {
	if token == "" {
		return errors.New("token kosong")
	}
	if len(token) != 64 {
		return errors.New("panjang token harus 64 karakter hex")
	}
	if _, err := hex.DecodeString(token); err != nil {
		return errors.New("token bukan hex valid")
	}
	return nil
}
