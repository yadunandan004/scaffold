package cache

import (
	"context"
	"time"
)

// CacheService defines the interface for cache operations
type CacheService interface {
	// Get retrieves a value by key
	Get(ctx context.Context, key string) (interface{}, error)

	// Set stores a key-value pair with optional expiration
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error

	// MGet retrieves multiple values by keys
	MGet(ctx context.Context, keys ...string) ([]interface{}, error)

	// MSet stores multiple key-value pairs
	MSet(ctx context.Context, pairs map[string]interface{}, expiration time.Duration) error

	// HGet retrieves a hash field value
	HGet(ctx context.Context, key, field string) (interface{}, error)

	// HSet stores a hash field value
	HSet(ctx context.Context, key, field string, value interface{}) error

	// HMGet retrieves multiple hash field values
	HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error)

	// HMSet stores multiple hash field values
	HMSet(ctx context.Context, key string, values map[string]interface{}) error

	// Delete removes one or more keys
	Delete(ctx context.Context, keys ...string) error

	// Exists checks if a key exists
	Exists(ctx context.Context, key string) (bool, error)

	// Expire sets expiration on a key
	Expire(ctx context.Context, key string, expiration time.Duration) error

	// TTL returns time to live for a key
	TTL(ctx context.Context, key string) (time.Duration, error)
}

// CacheOptions contains options for cache operations
type CacheOptions struct {
	// DefaultExpiration is the default expiration time for cache entries
	DefaultExpiration time.Duration

	// CleanupInterval is the interval for cleaning up expired entries (local cache only)
	CleanupInterval time.Duration
}

// DefaultCacheOptions returns default cache options
func DefaultCacheOptions() *CacheOptions {
	return &CacheOptions{
		DefaultExpiration: 5 * time.Minute,
		CleanupInterval:   10 * time.Minute,
	}
}
