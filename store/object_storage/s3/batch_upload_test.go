package s3

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yadunandan004/scaffold/store/object_storage"
)

func TestClient_BatchUpload(t *testing.T) {
	client := NewMockObjectStorage(t)
	defer StopMockObjectStorage()

	ctx := context.Background()

	// Prepare test data
	uploads := []object_storage.BatchUploadInput{
		{
			Bucket:      testBucket,
			Key:         "profile/user1/avatar.jpg",
			Data:        bytes.NewReader([]byte("user1 avatar data")),
			ContentType: "image/jpeg",
		},
		{
			Bucket:      testBucket,
			Key:         "profile/user2/avatar.png",
			Data:        bytes.NewReader([]byte("user2 avatar data")),
			ContentType: "image/png",
		},
		{
			Bucket:      testBucket,
			Key:         "badges/user1/gold.png",
			Data:        bytes.NewReader([]byte("gold badge data")),
			ContentType: "image/png",
		},
		{
			Bucket:      testBucket,
			Key:         "icons/app/logo.svg",
			Data:        bytes.NewReader([]byte("<svg>logo</svg>")),
			ContentType: "image/svg+xml",
		},
	}

	// Test batch upload
	result := client.BatchUpload(ctx, uploads)

	// Verify results
	assert.Equal(t, len(uploads), result.Successful)
	assert.Equal(t, 0, result.Failed)

	// Check for any errors
	for i, err := range result.Errors {
		if err != nil {
			t.Errorf("Upload %d failed: %v", i, err)
		}
	}

	// Verify files exist
	for _, upload := range uploads {
		exists, err := client.Exists(ctx, upload.Bucket, upload.Key)
		assert.NoError(t, err)
		assert.True(t, exists, "File %s should exist", upload.Key)
	}
}

func TestClient_BatchUpload_WithFailures(t *testing.T) {
	client := NewMockObjectStorage(t)
	defer StopMockObjectStorage()

	ctx := context.Background()

	// Include some invalid uploads
	uploads := []object_storage.BatchUploadInput{
		{
			Bucket:      testBucket,
			Key:         "valid/file.txt",
			Data:        bytes.NewReader([]byte("valid data")),
			ContentType: "text/plain",
		},
		{
			Bucket:      "non-existent-bucket", // This should fail
			Key:         "invalid/file.txt",
			Data:        bytes.NewReader([]byte("data")),
			ContentType: "text/plain",
		},
	}

	result := client.BatchUpload(ctx, uploads)

	// Should have one success and one failure
	assert.Equal(t, 1, result.Successful)
	assert.Equal(t, 1, result.Failed)

	// Check specific errors
	assert.NoError(t, result.Errors[0])
	assert.Error(t, result.Errors[1])
}

func TestValidateFileSize(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		shouldError bool
	}{
		{
			name:        "Small file",
			data:        make([]byte, 1024), // 1KB
			shouldError: false,
		},
		{
			name:        "Max size file",
			data:        make([]byte, MaxFileSize), // 5MB
			shouldError: false,
		},
		{
			name:        "Oversized file",
			data:        make([]byte, MaxFileSize+1), // 5MB + 1 byte
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.data)
			_, err := ValidateFileSize(reader)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "exceeds maximum allowed size")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_UploadWithValidation(t *testing.T) {
	client := NewMockObjectStorage(t)
	defer StopMockObjectStorage()

	ctx := context.Background()

	t.Run("Valid size file", func(t *testing.T) {
		data := bytes.NewReader([]byte("small file content"))
		err := client.UploadWithValidation(ctx, testBucket, "valid-file.txt", data, "text/plain")
		assert.NoError(t, err)

		// Verify file exists
		exists, err := client.Exists(ctx, testBucket, "valid-file.txt")
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("Oversized file", func(t *testing.T) {
		data := bytes.NewReader(make([]byte, MaxFileSize+1))
		err := client.UploadWithValidation(ctx, testBucket, "oversized-file.txt", data, "text/plain")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum allowed size")

		// Verify file was not uploaded
		exists, err := client.Exists(ctx, testBucket, "oversized-file.txt")
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestBatchUpload_Concurrency(t *testing.T) {
	client := NewMockObjectStorage(t)
	defer StopMockObjectStorage()

	ctx := context.Background()

	// Create many uploads to test concurrency limits
	numUploads := 50
	uploads := make([]object_storage.BatchUploadInput, numUploads)

	for i := 0; i < numUploads; i++ {
		uploads[i] = object_storage.BatchUploadInput{
			Bucket:      testBucket,
			Key:         fmt.Sprintf("concurrent/file-%d.txt", i),
			Data:        bytes.NewReader([]byte(fmt.Sprintf("content %d", i))),
			ContentType: "text/plain",
		}
	}

	// Upload should handle all files with controlled concurrency
	result := client.BatchUpload(ctx, uploads)

	assert.Equal(t, numUploads, result.Successful)
	assert.Equal(t, 0, result.Failed)

	// Verify all files exist
	for i := 0; i < numUploads; i++ {
		key := fmt.Sprintf("concurrent/file-%d.txt", i)
		exists, err := client.Exists(ctx, testBucket, key)
		assert.NoError(t, err)
		assert.True(t, exists, "File %s should exist", key)
	}
}
