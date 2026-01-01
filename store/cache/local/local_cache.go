package local

import (
	"context"
	"errors"
	"github.com/yadunandan004/scaffold/singleton"
	"github.com/yadunandan004/scaffold/store/cache"
	"sync"
	"time"
)

var ErrKeyNotFound = errors.New("key not found")
var ErrHashNotFound = errors.New("hash not found")

// LocalCache implements CacheService using an in-memory map
type LocalCache struct {
	mu      sync.RWMutex
	data    map[string]*cacheItem
	hashes  map[string]map[string]interface{}
	options *cache.CacheOptions
	stop    chan bool
}

type cacheItem struct {
	value      interface{}
	expiration time.Time
}

// LocalCacheBuilder implements the builder pattern for dependency injection
type LocalCacheBuilder struct {
	options *cache.CacheOptions
}

func (b LocalCacheBuilder) Build() cache.CacheService {
	if b.options == nil {
		b.options = cache.DefaultCacheOptions()
	}
	return NewLocalCache(b.options)
}

// NewLocalCache creates a new local cache instance
func NewLocalCache(options *cache.CacheOptions) *LocalCache {
	if options == nil {
		options = cache.DefaultCacheOptions()
	}

	lc := &LocalCache{
		data:    make(map[string]*cacheItem),
		hashes:  make(map[string]map[string]interface{}),
		options: options,
		stop:    make(chan bool),
	}

	// Start cleanup goroutine
	go lc.cleanupLoop()

	return lc
}

func (c *LocalCache) cleanupLoop() {
	for {
		select {
		case <-c.stop:
			return
		default:
			c.cleanup()

			// Sleep for cleanup interval
			select {
			case <-c.stop:
				return
			case <-time.After(c.options.CleanupInterval):
				// Continue to next iteration
			}
		}
	}
}

func (c *LocalCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.data {
		if !item.expiration.IsZero() && now.After(item.expiration) {
			delete(c.data, key)
		}
	}
}

// Get retrieves a value by key
func (c *LocalCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.data[key]
	if !exists {
		return nil, ErrKeyNotFound
	}

	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		return nil, ErrKeyNotFound
	}

	return item.value, nil
}

// Set stores a key-value pair with optional expiration
func (c *LocalCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var exp time.Time
	if expiration > 0 {
		exp = time.Now().Add(expiration)
	} else if c.options.DefaultExpiration > 0 {
		exp = time.Now().Add(c.options.DefaultExpiration)
	}

	c.data[key] = &cacheItem{
		value:      value,
		expiration: exp,
	}

	return nil
}

// MGet retrieves multiple values by keys
func (c *LocalCache) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	values := make([]interface{}, len(keys))
	now := time.Now()

	for i, key := range keys {
		item, exists := c.data[key]
		if !exists || (!item.expiration.IsZero() && now.After(item.expiration)) {
			values[i] = nil
			continue
		}
		values[i] = item.value
	}

	return values, nil
}

// MSet stores multiple key-value pairs
func (c *LocalCache) MSet(ctx context.Context, pairs map[string]interface{}, expiration time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var exp time.Time
	if expiration > 0 {
		exp = time.Now().Add(expiration)
	} else if c.options.DefaultExpiration > 0 {
		exp = time.Now().Add(c.options.DefaultExpiration)
	}

	for key, value := range pairs {
		c.data[key] = &cacheItem{
			value:      value,
			expiration: exp,
		}
	}

	return nil
}

// HGet retrieves a hash field value
func (c *LocalCache) HGet(ctx context.Context, key, field string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hash, exists := c.hashes[key]
	if !exists {
		return nil, ErrHashNotFound
	}

	value, exists := hash[field]
	if !exists {
		return nil, ErrKeyNotFound
	}

	return value, nil
}

// HSet stores a hash field value
func (c *LocalCache) HSet(ctx context.Context, key, field string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.hashes[key]; !exists {
		c.hashes[key] = make(map[string]interface{})
	}

	c.hashes[key][field] = value
	return nil
}

// HMGet retrieves multiple hash field values
func (c *LocalCache) HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hash, exists := c.hashes[key]
	if !exists {
		return nil, ErrHashNotFound
	}

	values := make([]interface{}, len(fields))
	for i, field := range fields {
		values[i] = hash[field]
	}

	return values, nil
}

// HMSet stores multiple hash field values
func (c *LocalCache) HMSet(ctx context.Context, key string, values map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.hashes[key]; !exists {
		c.hashes[key] = make(map[string]interface{})
	}

	for field, value := range values {
		c.hashes[key][field] = value
	}

	return nil
}

// Delete removes one or more keys
func (c *LocalCache) Delete(ctx context.Context, keys ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, key := range keys {
		delete(c.data, key)
		delete(c.hashes, key)
	}

	return nil
}

// Exists checks if a key exists
func (c *LocalCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if item, exists := c.data[key]; exists {
		if !item.expiration.IsZero() && time.Now().After(item.expiration) {
			return false, nil
		}
		return true, nil
	}

	_, exists := c.hashes[key]
	return exists, nil
}

// Expire sets expiration on a key
func (c *LocalCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.data[key]
	if !exists {
		return ErrKeyNotFound
	}

	item.expiration = time.Now().Add(expiration)
	return nil
}

// TTL returns time to live for a key
func (c *LocalCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.data[key]
	if !exists {
		return 0, ErrKeyNotFound
	}

	if item.expiration.IsZero() {
		return -1, nil // No expiration
	}

	ttl := time.Until(item.expiration)
	if ttl < 0 {
		return 0, ErrKeyNotFound
	}

	return ttl, nil
}

// Close stops the cleanup goroutine
func (c *LocalCache) Close() error {
	close(c.stop)
	return nil
}

// Register the builder with the injector
func init() {
	singleton.Inject[LocalCacheBuilder, cache.CacheService]()
}
