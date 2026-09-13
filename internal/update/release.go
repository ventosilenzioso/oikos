package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

type Release struct {
	Version   string
	BinaryURL string
	SHA256    string
	Signature []byte
	GOOS      string
	GOARCH    string
}

type ReleaseVerifier interface {
	Verify(Release, []byte) error
}

func verifyRelease(release Release, body []byte, verifier ReleaseVerifier) error {
	if release.Version == "" {
		return errors.New("release version is required")
	}
	if release.BinaryURL == "" {
		return errors.New("release binary URL is required")
	}
	want := strings.ToLower(strings.TrimSpace(release.SHA256))
	if len(want) != sha256.Size*2 {
		return errors.New("release SHA-256 is invalid")
	}
	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("release checksum mismatch: got %s, want %s", got, want)
	}
	if verifier != nil {
		if err := verifier.Verify(release, body); err != nil {
			return fmt.Errorf("release signature verification failed: %w", err)
		}
	}
	return nil
}
