package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yadunandan004/scaffold/singleton"
	"github.com/yadunandan004/scaffold/store/cache"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	globalClient *redis.Client
	clientMutex  sync.RWMutex
	configured   bool
)

// SetGlobalClient sets the global Redis client
func SetGlobalClient(client *redis.Client) {
	clientMutex.Lock()
	defer clientMutex.Unlock()
	globalClient = client
	configured = client != nil
}

// GetGlobalClient returns the global Redis client
func GetGlobalClient() *redis.Client {
	clientMutex.RLock()
	defer clientMutex.RUnlock()
	return globalClient
}

// IsConfigured returns whether Redis is configured
func IsConfigured() bool {
	clientMutex.RLock()
	defer clientMutex.RUnlock()
	return configured
}

// Ping checks if the Redis connection is alive
func Ping(ctx context.Context) error {
	client := GetGlobalClient()
	if client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	_, err := client.Ping(ctx).Result()
	return err
}

var ErrKeyNotFound = errors.New("key not found")

// RedisCache implements CacheService using Redis
type RedisCache struct {
	client  *redis.Client
	options *cache.CacheOptions
}

// RedisCacheBuilder implements the builder pattern for dependency injection
type RedisCacheBuilder struct {
	client  *redis.Client
	options *cache.CacheOptions
}

func (b RedisCacheBuilder) Build() cache.CacheService {
	if b.options == nil {
		b.options = cache.DefaultCacheOptions()
	}
	return NewRedisCache(b.client, b.options)
}

// RedisConfig contains Redis connection configuration
type RedisConfig struct {
	Host         string
	Port         int
	Password     string
	DB           int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolSize     int
}

// DefaultRedisConfig returns default Redis configuration
func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		Host:         "localhost",
		Port:         6379,
		Password:     "",
		DB:           0,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	}
}

// NewRedisClient creates a new Redis client
func NewRedisClient(config *RedisConfig) *redis.Client {
	if config == nil {
		config = DefaultRedisConfig()
	}

	return redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password:     config.Password,
		DB:           config.DB,
		MaxRetries:   config.MaxRetries,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		PoolSize:     config.PoolSize,
	})
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(client *redis.Client, options *cache.CacheOptions) *RedisCache {
	if options == nil {
		options = cache.DefaultCacheOptions()
	}

	return &RedisCache{
		client:  client,
		options: options,
	}
}

// Get retrieves a value by key
func (c *RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}

	// Try to unmarshal as JSON first
	var result interface{}
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		// If not JSON, return as string
		return val, nil
	}

	return result, nil
}

// Set stores a key-value pair with optional expiration
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	// Marshal value to JSON
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if expiration == 0 && c.options.DefaultExpiration > 0 {
		expiration = c.options.DefaultExpiration
	}

	return c.client.Set(ctx, key, data, expiration).Err()
}

// MGet retrieves multiple values by keys
func (c *RedisCache) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	vals, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	results := make([]interface{}, len(vals))
	for i, val := range vals {
		if val == nil {
			results[i] = nil
			continue
		}

		// Try to unmarshal as JSON
		var result interface{}
		if err := json.Unmarshal([]byte(val.(string)), &result); err != nil {
			// If not JSON, return as string
			results[i] = val
		} else {
			results[i] = result
		}
	}

	return results, nil
}

// MSet stores multiple key-value pairs
func (c *RedisCache) MSet(ctx context.Context, pairs map[string]interface{}, expiration time.Duration) error {
	// Redis MSET doesn't support expiration, so we need to use pipeline
	pipe := c.client.Pipeline()

	if expiration == 0 && c.options.DefaultExpiration > 0 {
		expiration = c.options.DefaultExpiration
	}

	for key, value := range pairs {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		pipe.Set(ctx, key, data, expiration)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// HGet retrieves a hash field value
func (c *RedisCache) HGet(ctx context.Context, key, field string) (interface{}, error) {
	val, err := c.client.HGet(ctx, key, field).Result()
	if err == redis.Nil {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}

	// Try to unmarshal as JSON
	var result interface{}
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		// If not JSON, return as string
		return val, nil
	}

	return result, nil
}

// HSet stores a hash field value
func (c *RedisCache) HSet(ctx context.Context, key, field string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.client.HSet(ctx, key, field, data).Err()
}

// HMGet retrieves multiple hash field values
func (c *RedisCache) HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error) {
	vals, err := c.client.HMGet(ctx, key, fields...).Result()
	if err != nil {
		return nil, err
	}

	results := make([]interface{}, len(vals))
	for i, val := range vals {
		if val == nil {
			results[i] = nil
			continue
		}

		// Try to unmarshal as JSON
		var result interface{}
		if err := json.Unmarshal([]byte(val.(string)), &result); err != nil {
			// If not JSON, return as string
			results[i] = val
		} else {
			results[i] = result
		}
	}

	return results, nil
}

// HMSet stores multiple hash field values
func (c *RedisCache) HMSet(ctx context.Context, key string, values map[string]interface{}) error {
	data := make(map[string]interface{})
	for field, value := range values {
		marshaled, err := json.Marshal(value)
		if err != nil {
			return err
		}
		data[field] = marshaled
	}

	return c.client.HMSet(ctx, key, data).Err()
}

// Delete removes one or more keys
func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Expire sets expiration on a key
func (c *RedisCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.client.Expire(ctx, key, expiration).Err()
}

// TTL returns time to live for a key
func (c *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	if ttl == -2 {
		return 0, ErrKeyNotFound
	}

	if ttl == -1 {
		return -1, nil // No expiration
	}

	return ttl, nil
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// Register the builder with the injector
func init() {
	singleton.Inject[RedisCacheBuilder, cache.CacheService]()
}
