package s3

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient_Upload_Download(t *testing.T) {
	client := NewMockObjectStorage(t)
	defer StopMockObjectStorage()

	ctx := context.Background()
	testData := []byte("test data content")
	contentType := "text/plain"

	// Test upload
	err := client.Upload(ctx, testBucket, testKey, bytes.NewReader(testData), contentType)
	assert.NoError(t, err)

	// Test download
	reader, err := client.Download(ctx, testBucket, testKey)
	assert.NoError(t, err)
	defer reader.Close()

	downloadedData, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, testData, downloadedData)
}

func TestClient_Exists(t *testing.T) {
	client := NewMockObjectStorage(t)
	defer StopMockObjectStorage()

	ctx := context.Background()
	testData := []byte("test data")

	// Check non-existent key
	exists, err := client.Exists(ctx, testBucket, "non-existent")
	assert.NoError(t, err)
	assert.False(t, exists)

	// Upload data
	err = client.Upload(ctx, testBucket, testKey, bytes.NewReader(testData), "text/plain")
	assert.NoError(t, err)

	// Check existing key
	exists, err = client.Exists(ctx, testBucket, testKey)
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestClient_Update(t *testing.T) {
	client := NewMockObjectStorage(t)
	defer StopMockObjectStorage()
	ctx := context.Background()
	originalData := []byte("original data")
	updatedData := []byte("updated data")

	// Upload original data
	err := client.Upload(ctx, testBucket, testKey, bytes.NewReader(originalData), "text/plain")
	assert.NoError(t, err)

	// Update data
	err = client.Update(ctx, testBucket, testKey, bytes.NewReader(updatedData), "text/plain")
	assert.NoError(t, err)

	// Download and verify updated data
	reader, err := client.Download(ctx, testBucket, testKey)
	assert.NoError(t, err)
	defer reader.Close()

	downloadedData, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, updatedData, downloadedData)
}

func TestClient_Delete(t *testing.T) {
	client := NewMockObjectStorage(t)
	defer StopMockObjectStorage()

	ctx := context.Background()
	testData := []byte("test data")

	// Upload data
	err := client.Upload(ctx, testBucket, testKey, bytes.NewReader(testData), "text/plain")
	assert.NoError(t, err)

	// Verify it exists
	exists, err := client.Exists(ctx, testBucket, testKey)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Delete data
	err = client.Delete(ctx, testBucket, testKey)
	assert.NoError(t, err)

	// Verify it's deleted
	exists, err = client.Exists(ctx, testBucket, testKey)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestClient_GetURL(t *testing.T) {
	client := NewMockObjectStorage(t)
	defer StopMockObjectStorage()

	url := client.GetURL(testBucket, testKey)
	assert.Contains(t, url, testBucket)
	assert.Contains(t, url, testKey)
}
