package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/yadunandan004/scaffold/store/object_storage"
	"io"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"golang.org/x/sync/errgroup"
)

// Client implements the ObjectStorage interface using AWS S3
type Client struct {
	client   *s3.Client
	endpoint string
}

// Config holds S3 configuration
type Config struct {
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool // For MinIO compatibility
}

const (
	MaxConcurrentUploads = 20              // Maximum concurrent uploads
	MaxFileSize          = 5 * 1024 * 1024 // 5MB max file size
)

// NewClient creates a new S3 client
func NewClient(cfg *Config) (*Client, error) {
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with custom endpoint if provided
	opts := []func(*s3.Options){
		func(o *s3.Options) {
			o.UsePathStyle = cfg.UsePathStyle
		},
	}

	if cfg.Endpoint != "" {
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	client := s3.NewFromConfig(awsCfg, opts...)

	return &Client{
		client:   client,
		endpoint: cfg.Endpoint,
	}, nil
}

// Upload uploads data to the specified key
func (c *Client) Upload(ctx context.Context, bucket, key string, data io.Reader, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        data,
		ContentType: aws.String(contentType),
	}

	_, err := c.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	return nil
}

// Download retrieves data from the specified key
func (c *Client) Download(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := c.client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to download object: %w", err)
	}

	return result.Body, nil
}

// Delete removes the object at the specified key
func (c *Client) Delete(ctx context.Context, bucket, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	_, err := c.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// Update replaces the object at the specified key
func (c *Client) Update(ctx context.Context, bucket, key string, data io.Reader, contentType string) error {
	// S3 update is the same as upload (it overwrites)
	return c.Upload(ctx, bucket, key, data, contentType)
}

// Exists checks if an object exists at the specified key
func (c *Client) Exists(ctx context.Context, bucket, key string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	_, err := c.client.HeadObject(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NotFound" {
				return false, nil
			}
		}
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}

	return true, nil
}

// GetURL returns a URL for accessing the object
func (c *Client) GetURL(bucket, key string) string {
	if c.endpoint != "" {
		return fmt.Sprintf("%s/%s/%s", c.endpoint, bucket, key)
	}
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucket, key)
}

// CreateBucket creates a new bucket if it doesn't exist
func (c *Client) CreateBucket(ctx context.Context, bucket string) error {
	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}

	_, err := c.client.CreateBucket(ctx, input)
	if err != nil {
		// Check if bucket already exists
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "BucketAlreadyExists" || apiErr.ErrorCode() == "BucketAlreadyOwnedByYou" {
				return nil
			}
		}
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	return nil
}

// BatchUpload uploads multiple files concurrently with controlled parallelism
func (c *Client) BatchUpload(ctx context.Context, uploads []object_storage.BatchUploadInput) *object_storage.BatchUploadResult {
	result := &object_storage.BatchUploadResult{
		Errors: make([]error, len(uploads)),
	}

	if len(uploads) == 0 {
		return result
	}

	// Create errgroup with limited concurrency
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(MaxConcurrentUploads)
	var mu sync.Mutex
	for i, upload := range uploads {
		idx := i
		u := upload
		g.Go(func() error {
			err := c.Upload(ctx, u.Bucket, u.Key, u.Data, u.ContentType)
			mu.Lock()
			if err != nil {
				result.Errors[idx] = fmt.Errorf("failed to upload %s: %w", u.Key, err)
				result.Failed++
			} else {
				result.Successful++
			}
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
	return result
}

// ValidateFileSize checks if the data size is within acceptable limits
func ValidateFileSize(data io.Reader) (io.Reader, error) {
	// Read all data to check size (since files are small, this is acceptable)
	buf, err := io.ReadAll(io.LimitReader(data, MaxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	if len(buf) > MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum allowed size of %d bytes", len(buf), MaxFileSize)
	}

	// Return a bytes.Reader which is seekable
	return bytes.NewReader(buf), nil
}

// UploadWithValidation uploads a file with size validation
func (c *Client) UploadWithValidation(ctx context.Context, bucket, key string, data io.Reader, contentType string) error {
	validatedData, err := ValidateFileSize(data)
	if err != nil {
		return err
	}

	return c.Upload(ctx, bucket, key, validatedData, contentType)
}

// Ensure Client implements ObjectStorage interface
var _ object_storage.ObjectStorage = (*Client)(nil)
