// Package storage provides file storage backend implementations.
package storage

// MinIOBackend reuses S3Backend for MinIO object storage.
type MinIOBackend = S3Backend

// NewMinIOBackend creates a new MinIO backend.
func NewMinIOBackend(options MinIOOptions) (*MinIOBackend, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}

	store, err := newS3CompatibleStoreFromOptions(options.toObjectStorageOptions(), "minio")
	if err != nil {
		return nil, err
	}

	return newMinIOBackendWithStore(store, options.Prefix), nil
}

func newMinIOBackendWithStore(store objectStore, prefix string) *MinIOBackend {
	return newS3BackendWithStore(store, prefix)
}

func (o MinIOOptions) validate() error {
	return validateObjectStorageOptions(o.toObjectStorageOptions(), "minio")
}

func (o MinIOOptions) toObjectStorageOptions() ObjectStorageOptions {
	return ObjectStorageOptions(o)
}
