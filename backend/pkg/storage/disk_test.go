package storage_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/TarunVishwakarma1/ims/backend/pkg/storage"
)

func TestDisk_SaveAndDelete(t *testing.T) {
	root := t.TempDir()
	s := storage.NewDiskStorage(root, "/uploads")
	ctx := context.Background()

	url, err := s.Save(ctx, "banners/abc.jpg", bytes.NewReader([]byte("hello")), "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if url != "/uploads/banners/abc.jpg" {
		t.Fatalf("url=%s", url)
	}

	got, err := os.ReadFile(filepath.Join(root, "banners/abc.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("content=%s", got)
	}

	if err := s.Delete(ctx, "banners/abc.jpg"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "banners/abc.jpg")); !os.IsNotExist(err) {
		t.Fatal("expected file removed")
	}
}

func TestDisk_RejectsPathTraversal(t *testing.T) {
	s := storage.NewDiskStorage(t.TempDir(), "/uploads")
	_, err := s.Save(context.Background(), "../etc/passwd", bytes.NewReader([]byte("x")), "text/plain")
	if !errors.Is(err, storage.ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}
