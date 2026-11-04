package storage_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"fite-arsip-api/internal/storage"
)

// Pastikan LocalStorage memenuhi kontrak interface storage.Storage.
var _ storage.Storage = (*storage.LocalStorage)(nil)

func TestLocalStorage(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "local_storage_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	store, err := storage.NewLocalStorage(tempDir)
	if err != nil {
		t.Fatalf("NewLocalStorage failed: %v", err)
	}

	// 1. Ping
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	// 2. Open non-existent file returns ErrNotFound
	_, _, err = store.Open(ctx, "documents/nonexistent.pdf")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// 3. Put nested file
	content := []byte("%PDF-1.4 test document content\n%%EOF")
	relPath := "documents/IF/2026/05/test_doc.pdf"
	if err := store.Put(ctx, relPath, bytes.NewReader(content), "application/pdf"); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// 4. Verify file exists on local filesystem
	expectedDiskPath := filepath.Join(tempDir, filepath.FromSlash(relPath))
	if _, err := os.Stat(expectedDiskPath); err != nil {
		t.Fatalf("file does not exist on disk at %s: %v", expectedDiskPath, err)
	}

	// 5. Open and verify content & size
	rc, size, err := store.Open(ctx, relPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), size)
	}

	readBytes, err := io.ReadAll(rc)
	_ = rc.Close() // Pastikan ditutup sebelum overwrite pada OS Windows
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(readBytes, content) {
		t.Errorf("content read does not match content written")
	}

	// 6. Overwrite file (atomicity & replace)
	newContent := []byte("%PDF-1.4 updated version content\n%%EOF")
	if err := store.Put(ctx, relPath, bytes.NewReader(newContent), "application/pdf"); err != nil {
		t.Fatalf("Put overwrite failed: %v", err)
	}

	rc2, size2, err := store.Open(ctx, relPath)
	if err != nil {
		t.Fatalf("Open after overwrite failed: %v", err)
	}

	if size2 != int64(len(newContent)) {
		t.Errorf("expected updated size %d, got %d", len(newContent), size2)
	}
	readBytes2, _ := io.ReadAll(rc2)
	_ = rc2.Close() // Pastikan ditutup sebelum Delete pada OS Windows
	if !bytes.Equal(readBytes2, newContent) {
		t.Errorf("content after overwrite does not match new content")
	}

	// 7. Path traversal defense
	badPaths := []string{
		"../escape.txt",
		"../../etc/passwd",
		"documents/../../../outside.txt",
	}
	for _, bp := range badPaths {
		if err := store.Put(ctx, bp, bytes.NewReader([]byte("hacked")), "text/plain"); err == nil {
			t.Errorf("expected error for path traversal '%s', got nil", bp)
		}
		if _, _, err := store.Open(ctx, bp); err == nil {
			t.Errorf("expected error for path traversal Open '%s', got nil", bp)
		}
	}

	// 8. Delete (idempotency)
	if err := store.Delete(ctx, relPath); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := os.Stat(expectedDiskPath); !os.IsNotExist(err) {
		t.Errorf("file should not exist on disk after Delete")
	}

	// Second delete on deleted file should return nil (idempotent)
	if err := store.Delete(ctx, relPath); err != nil {
		t.Fatalf("Second delete should be idempotent, got: %v", err)
	}

	// Open after delete returns ErrNotFound
	_, _, err = store.Open(ctx, relPath)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after Delete, got %v", err)
	}
}
