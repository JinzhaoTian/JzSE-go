// Package storage provides file storage backend implementations.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	commonerrors "asisaid.cn/JzSE/internal/common/errors"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	awstypes "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type objectStore interface {
	Put(ctx context.Context, key string, reader io.Reader, size int64) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Stat(ctx context.Context, key string) (*FileInfo, error)
	List(ctx context.Context, prefix string) ([]*FileInfo, error)
	Close() error
}

// S3Backend implements Backend using S3-compatible object storage.
type S3Backend struct {
	store  objectStore
	prefix string
}

func newS3BackendWithStore(store objectStore, prefix string) *S3Backend {
	return &S3Backend{
		store:  store,
		prefix: normalizePrefix(prefix),
	}
}

func (b *S3Backend) Put(ctx context.Context, key string, reader io.Reader, size int64) error {
	return b.store.Put(ctx, b.toObjectKey(key), reader, size)
}

func (b *S3Backend) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return b.store.Get(ctx, b.toObjectKey(key))
}

func (b *S3Backend) Delete(ctx context.Context, key string) error {
	return b.store.Delete(ctx, b.toObjectKey(key))
}

func (b *S3Backend) Exists(ctx context.Context, key string) (bool, error) {
	_, err := b.store.Stat(ctx, b.toObjectKey(key))
	if err != nil {
		if commonerrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (b *S3Backend) Stat(ctx context.Context, key string) (*FileInfo, error) {
	info, err := b.store.Stat(ctx, b.toObjectKey(key))
	if err != nil {
		return nil, err
	}

	info.Key = b.fromObjectKey(info.Key)
	return info, nil
}

func (b *S3Backend) List(ctx context.Context, prefix string) ([]*FileInfo, error) {
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

func (b *S3Backend) Close() error {
	return b.store.Close()
}

func (b *S3Backend) toObjectKey(key string) string {
	cleanKey := strings.TrimLeft(key, "/")
	if b.prefix == "" {
		return cleanKey
	}
	if cleanKey == "" {
		return b.prefix + "/"
	}
	return b.prefix + "/" + cleanKey
}

func (b *S3Backend) toObjectPrefix(prefix string) string {
	cleanPrefix := strings.TrimLeft(prefix, "/")
	if b.prefix == "" {
		return cleanPrefix
	}
	if cleanPrefix == "" {
		return b.prefix + "/"
	}
	return b.prefix + "/" + cleanPrefix
}

func (b *S3Backend) fromObjectKey(objectKey string) string {
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

func validateObjectStorageOptions(options ObjectStorageOptions, backendName string) error {
	switch {
	case strings.TrimSpace(options.Endpoint) == "":
		return fmt.Errorf("%s endpoint is required", backendName)
	case strings.TrimSpace(options.AccessKey) == "":
		return fmt.Errorf("%s access key is required", backendName)
	case strings.TrimSpace(options.SecretKey) == "":
		return fmt.Errorf("%s secret key is required", backendName)
	case strings.TrimSpace(options.Bucket) == "":
		return fmt.Errorf("%s bucket is required", backendName)
	default:
		return nil
	}
}

func normalizeS3CompatibleEndpoint(endpoint string, useSSL bool) string {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "://") {
		return trimmed
	}
	if useSSL {
		return "https://" + trimmed
	}
	return "http://" + trimmed
}

type s3CompatibleStore struct {
	client *s3.Client
	bucket string
	region string
}

func newS3CompatibleStoreFromOptions(options ObjectStorageOptions, backendName string) (objectStore, error) {
	cfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(options.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			options.AccessKey,
			options.SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize %s aws config: %w", backendName, err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		endpoint := normalizeS3CompatibleEndpoint(options.Endpoint, options.UseSSL)
		if endpoint != "" {
			o.BaseEndpoint = &endpoint
		}
	})

	store := &s3CompatibleStore{
		client: client,
		bucket: options.Bucket,
		region: options.Region,
	}

	if err := store.ensureBucket(context.Background()); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *s3CompatibleStore) ensureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
	if err == nil {
		return nil
	}
	if !isS3CompatibleNotFound(err) {
		return mapS3CompatibleError(err)
	}

	input := &s3.CreateBucketInput{Bucket: &s.bucket}
	if s.region != "" && s.region != "us-east-1" {
		input.CreateBucketConfiguration = &awstypes.CreateBucketConfiguration{
			LocationConstraint: awstypes.BucketLocationConstraint(s.region),
		}
	}

	_, err = s.client.CreateBucket(ctx, input)
	if err != nil {
		if isS3CompatibleBucketAlreadyExists(err) {
			return nil
		}
		return mapS3CompatibleError(err)
	}

	return nil
}

func (s *s3CompatibleStore) Put(ctx context.Context, key string, reader io.Reader, size int64) error {
	input := &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
		Body:   reader,
	}
	if size >= 0 {
		input.ContentLength = &size
	}

	_, err := s.client.PutObject(ctx, input)
	return mapS3CompatibleError(err)
}

func (s *s3CompatibleStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return nil, mapS3CompatibleError(err)
	}
	return out.Body, nil
}

func (s *s3CompatibleStore) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	return mapS3CompatibleError(err)
}

func (s *s3CompatibleStore) Stat(ctx context.Context, key string) (*FileInfo, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return nil, mapS3CompatibleError(err)
	}

	modTime := time.Time{}
	if out.LastModified != nil {
		modTime = *out.LastModified
	}
	size := int64(0)
	if out.ContentLength != nil {
		size = *out.ContentLength
	}

	return &FileInfo{
		Key:     key,
		Size:    size,
		ModTime: modTime,
		IsDir:   false,
	}, nil
}

func (s *s3CompatibleStore) List(ctx context.Context, prefix string) ([]*FileInfo, error) {
	result := make([]*FileInfo, 0)
	p := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: &s.bucket,
		Prefix: &prefix,
	})

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, mapS3CompatibleError(err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			mod := time.Time{}
			if obj.LastModified != nil {
				mod = *obj.LastModified
			}
			size := int64(0)
			if obj.Size != nil {
				size = *obj.Size
			}
			result = append(result, &FileInfo{
				Key:     *obj.Key,
				Size:    size,
				ModTime: mod,
				IsDir:   false,
			})
		}
	}

	return result, nil
}

func (s *s3CompatibleStore) Close() error {
	return nil
}

func mapS3CompatibleError(err error) error {
	if err == nil {
		return nil
	}
	if isS3CompatibleNotFound(err) {
		return commonerrors.ErrNotFound
	}
	return err
}

func isS3CompatibleNotFound(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.ErrorCode() {
	case "NotFound", "NoSuchKey", "NoSuchBucket", "NoSuchObject", "404":
		return true
	default:
		return false
	}
}

func isS3CompatibleBucketAlreadyExists(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.ErrorCode() {
	case "BucketAlreadyOwnedByYou", "BucketAlreadyExists":
		return true
	default:
		return false
	}
}

// NewS3Backend creates a new S3-compatible backend.
func NewS3Backend(options S3Options) (*S3Backend, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}

	store, err := newS3CompatibleStoreFromOptions(options.toObjectStorageOptions(), "s3")
	if err != nil {
		return nil, err
	}

	return newS3BackendWithStore(store, options.Prefix), nil
}

func (o S3Options) validate() error {
	return validateObjectStorageOptions(o.toObjectStorageOptions(), "s3")
}

func (o S3Options) toObjectStorageOptions() ObjectStorageOptions {
	return ObjectStorageOptions(o)
}
