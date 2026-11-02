package storage_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"fite-arsip-api/internal/storage"
)

// Ensure both implementations satisfy the storage.Storage interface.
var _ storage.Storage = (*storage.MemoryStorage)(nil)
var _ storage.Storage = (*storage.GCS)(nil)

func TestMemoryStorage(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMemoryStorage()

	// 1. Ping
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	// 2. Open non-existent object
	_, _, err := store.Open(ctx, "nonexistent/file.pdf")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// 3. Put object
	content := []byte("%PDF-1.4 sample content")
	path := "documents/2026/test.pdf"
	contentType := "application/pdf"

	if err := store.Put(ctx, path, bytes.NewReader(content), contentType); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// 4. Open object and verify
	rc, size, err := store.Open(ctx, path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer rc.Close()

	if size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), size)
	}

	readBytes, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(readBytes, content) {
		t.Errorf("expected read data to match written data")
	}

	// 5. Delete object (idempotent)
	if err := store.Delete(ctx, path); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Delete again should succeed (idempotent)
	if err := store.Delete(ctx, path); err != nil {
		t.Fatalf("Second delete failed: %v", err)
	}

	// 6. Open deleted object returns ErrNotFound
	_, _, err = store.Open(ctx, path)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after deletion, got %v", err)
	}
}
