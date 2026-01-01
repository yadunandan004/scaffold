package s3

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	testBucket = "test-bucket"
	testKey    = "test-key"
)

var tc testcontainers.Container

func NewMockObjectStorage(t *testing.T) *Client {
	ctx := context.Background()

	// Create MinIO container
	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin",
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	// Get container host and port
	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "9000")
	require.NoError(t, err)

	// Create S3 client
	cfg := &Config{
		Region:          "us-east-1",
		Endpoint:        "http://" + host + ":" + port.Port(),
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		UsePathStyle:    true,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Create test bucket
	err = client.CreateBucket(ctx, testBucket)
	require.NoError(t, err)
	tc = container
	return client
}

func StopMockObjectStorage() {
	_ = tc.Terminate(context.Background())
}
