package storage

import (
	"context"
	"errors"
	"io"

	gcs "cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

type GCS struct {
	client *gcs.Client
	bucket string
}

func NewGCS(ctx context.Context, bucket, credentialsFile string) (*GCS, error) {
	var opts []option.ClientOption
	if credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}
	cl, err := gcs.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &GCS{client: cl, bucket: bucket}, nil
}

func (g *GCS) obj(path string) *gcs.ObjectHandle { return g.client.Bucket(g.bucket).Object(path) }

// Put mengalirkan data ke GCS dengan penyangga 2 MB (memori terbatas walau unggahan paralel).
func (g *GCS) Put(ctx context.Context, path string, r io.Reader, contentType string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	w := g.obj(path).NewWriter(ctx)
	w.ContentType = contentType
	w.ChunkSize = 2 << 20
	if _, err := io.Copy(w, r); err != nil {
		cancel() // batalkan unggahan agar tidak ada objek parsial
		_ = w.Close()
		return err
	}
	return w.Close()
}

func (g *GCS) Open(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	r, err := g.obj(path).NewReader(ctx)
	if errors.Is(err, gcs.ErrObjectNotExist) {
		return nil, 0, ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	return r, r.Attrs.Size, nil
}

func (g *GCS) Delete(ctx context.Context, path string) error {
	err := g.obj(path).Delete(ctx)
	if errors.Is(err, gcs.ErrObjectNotExist) {
		return nil
	}
	return err
}

func (g *GCS) Ping(ctx context.Context) error {
	_, err := g.client.Bucket(g.bucket).Attrs(ctx)
	return err
}

