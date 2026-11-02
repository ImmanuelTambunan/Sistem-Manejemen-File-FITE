package storage

import (
	"bytes"
	"context"
	"io"
	"sync"
)

type memoryObject struct {
	data        []byte
	contentType string
}

// MemoryStorage implements Storage in memory for tests or local dev.
type MemoryStorage struct {
	mu      sync.RWMutex
	objects map[string]memoryObject
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		objects: make(map[string]memoryObject),
	}
}

func (m *MemoryStorage) Put(ctx context.Context, path string, r io.Reader, contentType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.objects[path] = memoryObject{
		data:        data,
		contentType: contentType,
	}
	return nil
}

func (m *MemoryStorage) Open(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	obj, ok := m.objects[path]
	if !ok {
		return nil, 0, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(obj.data)), int64(len(obj.data)), nil
}

func (m *MemoryStorage) Delete(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.objects, path)
	return nil
}

func (m *MemoryStorage) Ping(ctx context.Context) error {
	return nil
}

