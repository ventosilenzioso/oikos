package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
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
	if err := validateVersion(release.Version); err != nil {
		return err
	}
	if release.BinaryURL == "" {
		return errors.New("release binary URL is required")
	}
	want := release.SHA256
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(want) {
		return errors.New("release SHA-256 is invalid")
	}
	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("release checksum mismatch: got %s, want %s", got, want)
	}
	if verifier == nil {
		return errors.New("release verifier is required")
	}
	if len(release.Signature) == 0 {
		return errors.New("release signature is required")
	}
	if err := verifier.Verify(release, body); err != nil {
		return fmt.Errorf("release signature verification failed: %w", err)
	}
	return nil
}

func validateVersion(version string) error {
	if version == "" || version == "." || version == ".." || version == "current" || version == "previous" || version == "state.json" || filepath.Base(version) != version || filepath.IsAbs(version) {
		return fmt.Errorf("invalid release version %q", version)
	}
	return nil
}
