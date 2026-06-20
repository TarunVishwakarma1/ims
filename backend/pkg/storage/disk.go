package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type DiskStorage struct {
	Root         string
	PublicPrefix string // e.g. "/uploads"
}

func NewDiskStorage(root, publicPrefix string) *DiskStorage {
	return &DiskStorage{Root: root, PublicPrefix: strings.TrimRight(publicPrefix, "/")}
}

func (d *DiskStorage) Save(ctx context.Context, key string, r io.Reader, _ string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	target := filepath.Join(d.Root, key)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return d.PublicPrefix + "/" + key, nil
}

func (d *DiskStorage) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	target := filepath.Join(d.Root, key)
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func validateKey(key string) error {
	if key == "" {
		return ErrInvalidKey
	}
	clean := filepath.Clean(key)
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, "/../") || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, `\`) {
		return ErrInvalidKey
	}
	return nil
}
