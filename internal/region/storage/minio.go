// Package storage provides file storage backend implementations.
package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	commonerrors "asisaid.cn/JzSE/internal/common/errors"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type minioObjectStore interface {
	Put(ctx context.Context, key string, reader io.Reader, size int64) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Stat(ctx context.Context, key string) (*FileInfo, error)
	List(ctx context.Context, prefix string) ([]*FileInfo, error)
	Close() error
}

// MinIOBackend implements Backend using MinIO object storage.
type MinIOBackend struct {
	store  minioObjectStore
	prefix string
}

// NewMinIOBackend creates a new MinIOBackend.
func NewMinIOBackend(options MinIOOptions) (*MinIOBackend, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}

	client, err := minio.New(options.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(options.AccessKey, options.SecretKey, ""),
		Secure: options.UseSSL,
		Region: options.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize minio client: %w", err)
	}

	store := &minioStore{
		client: client,
		bucket: options.Bucket,
		region: options.Region,
	}

	if err := store.ensureBucket(context.Background()); err != nil {
		return nil, err
	}

	return newMinIOBackendWithStore(store, options.Prefix), nil
}

func newMinIOBackendWithStore(store minioObjectStore, prefix string) *MinIOBackend {
	return &MinIOBackend{
		store:  store,
		prefix: normalizePrefix(prefix),
	}
}

// Put stores a file.
func (b *MinIOBackend) Put(ctx context.Context, key string, reader io.Reader, size int64) error {
	return b.store.Put(ctx, b.toObjectKey(key), reader, size)
}

// Get retrieves a file.
func (b *MinIOBackend) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return b.store.Get(ctx, b.toObjectKey(key))
}

// Delete removes a file.
func (b *MinIOBackend) Delete(ctx context.Context, key string) error {
	return b.store.Delete(ctx, b.toObjectKey(key))
}

// Exists checks if a file exists.
func (b *MinIOBackend) Exists(ctx context.Context, key string) (bool, error) {
	_, err := b.store.Stat(ctx, b.toObjectKey(key))
	if err != nil {
		if commonerrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Stat returns file information.
func (b *MinIOBackend) Stat(ctx context.Context, key string) (*FileInfo, error) {
	info, err := b.store.Stat(ctx, b.toObjectKey(key))
	if err != nil {
		return nil, err
	}

	info.Key = b.fromObjectKey(info.Key)
	return info, nil
}

// List lists files with the given prefix.
func (b *MinIOBackend) List(ctx context.Context, prefix string) ([]*FileInfo, error) {
	list, err := b.store.List(ctx, b.toObjectPrefix(prefix))
	if err != nil {
		return nil, err
	}

	result := make([]*FileInfo, 0, len(list))
	for _, item := range list {
		itemCopy := *item
		itemCopy.Key = b.fromObjectKey(item.Key)
		result = append(result, &itemCopy)
	}

	return result, nil
}

// Close closes the backend.
func (b *MinIOBackend) Close() error {
	return b.store.Close()
}

func (b *MinIOBackend) toObjectKey(key string) string {
	cleanKey := strings.TrimLeft(key, "/")
	if b.prefix == "" {
		return cleanKey
	}
	if cleanKey == "" {
		return b.prefix + "/"
	}
	return b.prefix + "/" + cleanKey
}

func (b *MinIOBackend) toObjectPrefix(prefix string) string {
	cleanPrefix := strings.TrimLeft(prefix, "/")
	if b.prefix == "" {
		return cleanPrefix
	}
	if cleanPrefix == "" {
		return b.prefix + "/"
	}
	return b.prefix + "/" + cleanPrefix
}

func (b *MinIOBackend) fromObjectKey(objectKey string) string {
	if b.prefix == "" {
		return objectKey
	}

	prefixWithSlash := b.prefix + "/"
	if strings.HasPrefix(objectKey, prefixWithSlash) {
		return strings.TrimPrefix(objectKey, prefixWithSlash)
	}

	return objectKey
}

func normalizePrefix(prefix string) string {
	return strings.Trim(strings.TrimSpace(prefix), "/")
}

func (o MinIOOptions) validate() error {
	switch {
	case strings.TrimSpace(o.Endpoint) == "":
		return fmt.Errorf("minio endpoint is required")
	case strings.TrimSpace(o.AccessKey) == "":
		return fmt.Errorf("minio access key is required")
	case strings.TrimSpace(o.SecretKey) == "":
		return fmt.Errorf("minio secret key is required")
	case strings.TrimSpace(o.Bucket) == "":
		return fmt.Errorf("minio bucket is required")
	default:
		return nil
	}
}

type minioStore struct {
	client *minio.Client
	bucket string
	region string
}

func (s *minioStore) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return mapMinIOError(err)
	}
	if exists {
		return nil
	}

	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{Region: s.region}); err != nil {
		resp := minio.ToErrorResponse(err)
		if resp.Code == "BucketAlreadyOwnedByYou" || resp.Code == "BucketAlreadyExists" {
			return nil
		}
		return mapMinIOError(err)
	}

	return nil
}

func (s *minioStore) Put(ctx context.Context, key string, reader io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{})
	return mapMinIOError(err)
}

func (s *minioStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, mapMinIOError(err)
	}

	if _, err := object.Stat(); err != nil {
		_ = object.Close()
		return nil, mapMinIOError(err)
	}

	return object, nil
}

func (s *minioStore) Delete(ctx context.Context, key string) error {
	return mapMinIOError(s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}))
}

func (s *minioStore) Stat(ctx context.Context, key string) (*FileInfo, error) {
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return nil, mapMinIOError(err)
	}

	return &FileInfo{
		Key:     info.Key,
		Size:    info.Size,
		ModTime: info.LastModified,
		IsDir:   false,
	}, nil
}

func (s *minioStore) List(ctx context.Context, prefix string) ([]*FileInfo, error) {
	result := make([]*FileInfo, 0)
	for object := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if object.Err != nil {
			return nil, mapMinIOError(object.Err)
		}

		result = append(result, &FileInfo{
			Key:     object.Key,
			Size:    object.Size,
			ModTime: object.LastModified,
			IsDir:   false,
		})
	}

	return result, nil
}

func (s *minioStore) Close() error {
	return nil
}

func mapMinIOError(err error) error {
	if err == nil {
		return nil
	}
	if isMinIONotFound(err) {
		return commonerrors.ErrNotFound
	}
	return err
}

func isMinIONotFound(err error) bool {
	resp := minio.ToErrorResponse(err)
	if resp.StatusCode == 404 {
		return true
	}

	switch resp.Code {
	case "NoSuchKey", "NoSuchBucket", "NoSuchObject", "NotFound":
		return true
	default:
		return false
	}
}
