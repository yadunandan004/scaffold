package store

import (
	"context"
	"fmt"
	"github.com/yadunandan004/scaffold/logger"
	"github.com/yadunandan004/scaffold/store/object_storage"
	"github.com/yadunandan004/scaffold/store/object_storage/s3"
	"os"
	"strings"

	"github.com/yadunandan004/scaffold/config"
)

// InitObjectStorage initializes the object storage system
func InitObjectStorage(cfg *config.S3Config) error {
	if cfg == nil || !cfg.Enabled {
		logger.LogInfo(nil, "Object storage is disabled")
		return nil
	}

	logger.LogInfo(nil, "Initializing object storage...")

	// Create S3 client configuration
	s3Config := &s3.Config{
		Region:          cfg.Region,
		Endpoint:        cfg.Endpoint,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
		UsePathStyle:    cfg.UsePathStyle,
	}

	// Create S3 client
	client, err := s3.NewClient(s3Config)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	object_storage.SetDefaultStorage(client)
	ctx := context.Background()
	if err := object_storage.Ping(ctx, client); err != nil {
		logger.LogInfo(nil, "Object storage ping failed: %v", err)
		return err
	}
	// Create default buckets if enabled
	if shouldAutoCreateBuckets() {
		ctx := context.Background()
		buckets := getRequiredBuckets(cfg)
		for _, bucket := range buckets {
			logger.LogInfo(nil, "Ensuring bucket exists: %s", bucket)
			if err := client.CreateBucket(ctx, bucket); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}
	}
	logger.LogInfo(nil, "Object storage initialized successfully")
	return nil
}

// shouldAutoCreateBuckets determines if buckets should be auto-created
func shouldAutoCreateBuckets() bool {
	// Check environment variable
	autoCreate := os.Getenv("S3_AUTO_CREATE_BUCKETS")
	if autoCreate == "" {
		// Default to true in development, false in production
		env := os.Getenv("ENV")
		return env == "" || env == "development" || env == "dev" || env == "local"
	}
	return strings.ToLower(autoCreate) == "true"
}

// getRequiredBuckets returns list of buckets that should exist
func getRequiredBuckets(cfg *config.S3Config) []string {
	buckets := []string{}

	// Add default bucket from config
	if cfg.Bucket != "" {
		buckets = append(buckets, cfg.Bucket)
	}

	// Add any other application-specific buckets
	// These could be environment-specific
	buckets = append(buckets,
		"user-images",  // Profile pictures, badges
		"app-assets",   // Icons, static assets
		"temp-uploads", // Temporary upload storage
	)

	return buckets
}
