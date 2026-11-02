package storage

import (
	"context"
	"errors"
	"io"
)

var ErrNotFound = errors.New("storage: objek tidak ditemukan")

type Storage interface {
	Put(ctx context.Context, path string, r io.Reader, contentType string) error
	Open(ctx context.Context, path string) (io.ReadCloser, int64, error)
	Delete(ctx context.Context, path string) error // idempoten: objek tidak ada = sukses
	Ping(ctx context.Context) error
}

