package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func createArchive(ctx context.Context, root, dest, id string) (BackupResult, error) {
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return BackupResult{}, err
	}
	defer f.Close()
	hash := sha256.New()
	writer := io.MultiWriter(f, hash)
	gz := gzip.NewWriter(writer)
	tw := tar.NewWriter(gz)
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." || rel == ".." || filepath.IsAbs(rel) || rel == ".."+string(os.PathSeparator) {
			return fmt.Errorf("archive path invalid")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink archive ditolak: %s", rel)
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			in, err := os.Open(path)
			if err != nil {
				return err
			}
			_, err = io.Copy(tw, in)
			_ = in.Close()
			return err
		}
		return nil
	})
	if err != nil {
		tw.Close()
		gz.Close()
		return BackupResult{}, err
	}
	if err := tw.Close(); err != nil {
		return BackupResult{}, err
	}
	if err := gz.Close(); err != nil {
		return BackupResult{}, err
	}
	if err := f.Sync(); err != nil {
		return BackupResult{}, err
	}
	info, err := f.Stat()
	if err != nil {
		return BackupResult{}, err
	}
	return BackupResult{BackupID: id, SizeBytes: info.Size(), ChecksumSHA: hex.EncodeToString(hash.Sum(nil))}, nil
}
