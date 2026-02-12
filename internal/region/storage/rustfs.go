// Package storage provides file storage backend implementations.
package storage

// RustFSBackend reuses S3Backend for RustFS object storage.
type RustFSBackend = S3Backend

// NewRustFSBackend creates a new RustFS backend.
func NewRustFSBackend(options RustFSOptions) (*RustFSBackend, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}

	store, err := newS3CompatibleStoreFromOptions(options.toObjectStorageOptions(), "rustfs")
	if err != nil {
		return nil, err
	}

	return newRustFSBackendWithStore(store, options.Prefix), nil
}

func newRustFSBackendWithStore(store objectStore, prefix string) *RustFSBackend {
	return newS3BackendWithStore(store, prefix)
}

func (o RustFSOptions) validate() error {
	return validateObjectStorageOptions(o.toObjectStorageOptions(), "rustfs")
}

func (o RustFSOptions) toObjectStorageOptions() ObjectStorageOptions {
	return ObjectStorageOptions(o)
}
