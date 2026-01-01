package object_storage

import (
	"context"
	"fmt"
	"io"
	"sync"
)

var (
	defaultStorage ObjectStorage
	storageMutex   sync.RWMutex
)

// BatchUploadInput represents a single file in batch upload
type BatchUploadInput struct {
	Bucket      string
	Key         string
	Data        io.Reader
	ContentType string
}

// BatchUploadResult represents the result of a batch upload operation
type BatchUploadResult struct {
	Successful int
	Failed     int
	Errors     []error
}

// ObjectMetadata represents metadata for an object in storage
type ObjectMetadata struct {
	Size        int64
	ContentType string
	ETag        string
}

// ObjectStorage defines the interface for object storage operations
type ObjectStorage interface {
	// Upload uploads data to the specified key
	Upload(ctx context.Context, bucket, key string, data io.Reader, contentType string) error

	BatchUpload(ctx context.Context, uploads []BatchUploadInput) *BatchUploadResult

	UploadWithValidation(ctx context.Context, bucket, key string, data io.Reader, contentType string) error

	// Download retrieves data from the specified key
	Download(ctx context.Context, bucket, key string) (io.ReadCloser, error)

	// Delete removes the object at the specified key
	Delete(ctx context.Context, bucket, key string) error

	// Update replaces the object at the specified key
	Update(ctx context.Context, bucket, key string, data io.Reader, contentType string) error

	// Exists checks if an object exists at the specified key
	Exists(ctx context.Context, bucket, key string) (bool, error)

	// GetURL returns a URL for accessing the object
	GetURL(bucket, key string) string
}

// SetDefaultStorage sets the default object storage instance
func SetDefaultStorage(storage ObjectStorage) {
	storageMutex.Lock()
	defer storageMutex.Unlock()
	defaultStorage = storage
}

// GetDefaultStorage returns the default object storage instance
func GetDefaultStorage() ObjectStorage {
	storageMutex.RLock()
	defer storageMutex.RUnlock()
	return defaultStorage
}

// Ping checks if the object storage is accessible
func Ping(ctx context.Context, storage ObjectStorage) error {
	if storage == nil {
		return fmt.Errorf("object storage is nil")
	}
	testBucket := "health-check"
	testKey := "ping-test"

	_, err := storage.Exists(ctx, testBucket, testKey)
	if err != nil {
		errStr := err.Error()
		if contains(errStr, "NoSuchBucket") || contains(errStr, "not found") {
			return nil
		}
		return err
	}

	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					len(substr) < len(s) && findSubstr(s, substr)))
}

func findSubstr(s, substr string) bool {
	for i := 1; i < len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
