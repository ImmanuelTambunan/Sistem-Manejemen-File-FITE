package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStorage mengimplementasikan Storage interface dengan menyimpan berkas fisik
// pada direktori lokal di sistem berkas (disk).
type LocalStorage struct {
	baseDir string
}

// NewLocalStorage membuat instans LocalStorage baru pada direktori dasar yang ditentukan.
func NewLocalStorage(baseDir string) (*LocalStorage, error) {
	if baseDir == "" {
		baseDir = "./uploads"
	}
	absDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("local storage path absolut: %w", err)
	}
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return nil, fmt.Errorf("local storage buat direktori: %w", err)
	}
	return &LocalStorage{baseDir: absDir}, nil
}

// resolve memvalidasi dan memetakan path relatif ke path absolut di dalam baseDir.
// Mencegah kerentanan path traversal (directory traversal).
func (s *LocalStorage) resolve(path string) (string, error) {
	cleanPath := filepath.Clean(filepath.FromSlash(path))
	if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("path berkas storage tidak valid: %s", path)
	}
	full := filepath.Join(s.baseDir, cleanPath)
	rel, err := filepath.Rel(s.baseDir, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("percobaan path traversal terdeteksi: %s", path)
	}
	return full, nil
}

// Put menyimpan isi berkas fisik ke direktori lokal secara atomik.
func (s *LocalStorage) Put(ctx context.Context, path string, r io.Reader, contentType string) error {
	full, err := s.resolve(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("local storage gagal membuat direktori induk: %w", err)
	}

	// Tulis berkas sementara lalu rename secara atomik
	tmpFile, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return fmt.Errorf("local storage gagal membuat berkas sementara: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := io.Copy(tmpFile, r); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("local storage gagal menyalin isi berkas: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("local storage gagal menutup berkas sementara: %w", err)
	}

	// Pada Windows, hapus target lama terlebih dahulu jika sudah ada sebelum rename
	_ = os.Remove(full)
	if err := os.Rename(tmpName, full); err != nil {
		return fmt.Errorf("local storage gagal me-rename berkas: %w", err)
	}
	return nil
}

// Open membuka stream pembacaan berkas fisik lokal dan mengembalikan ukuran berkas dalam byte.
func (s *LocalStorage) Open(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	full, err := s.resolve(path)
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, ErrNotFound
		}
		return nil, 0, fmt.Errorf("local storage gagal membuka berkas: %w", err)
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, fmt.Errorf("local storage gagal membaca metadata berkas: %w", err)
	}
	return f, fi.Size(), nil
}

// Delete menghapus berkas fisik lokal secara idempoten (objek tidak ada = sukses).
func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	full, err := s.resolve(path)
	if err != nil {
		return err
	}
	err = os.Remove(full)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("local storage gagal menghapus berkas: %w", err)
	}
	return nil
}

// Ping memeriksa apakah direktori penyimpanan lokal dapat diakses dan ditulis.
func (s *LocalStorage) Ping(ctx context.Context) error {
	testFile := filepath.Join(s.baseDir, ".storage_ping")
	if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
		return fmt.Errorf("local storage tidak dapat ditulis: %w", err)
	}
	_ = os.Remove(testFile)
	return nil
}
