// Package storage provides file storage backend implementations.
package storage

import (
	"fmt"
	"strings"
)

// MinIOBackend reuses the S3 backend implementation.
type MinIOBackend = S3Backend

// NewMinIOBackend creates a new MinIOBackend.
func NewMinIOBackend(options MinIOOptions) (*MinIOBackend, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}

	return NewS3Backend(S3Options{
		Endpoint:  options.Endpoint,
		AccessKey: options.AccessKey,
		SecretKey: options.SecretKey,
		Bucket:    options.Bucket,
		Region:    options.Region,
		UseSSL:    options.UseSSL,
		Prefix:    options.Prefix,
	})
}

func newMinIOBackendWithStore(store s3ObjectStore, prefix string) *MinIOBackend {
	return newS3BackendWithStore(store, prefix)
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
