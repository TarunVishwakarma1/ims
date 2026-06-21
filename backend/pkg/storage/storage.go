package storage

import (
	"context"
	"errors"
	"io"
)

type Storage interface {
	Save(ctx context.Context, key string, r io.Reader, contentType string) (publicURL string, err error)
	Delete(ctx context.Context, key string) error
}

var ErrInvalidKey = errors.New("invalid storage key")
