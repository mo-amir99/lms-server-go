package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Cache is a lightweight in-memory cache for frequently accessed data.
type Cache struct {
	items map[string]*item
	mu    sync.RWMutex
	ttl   time.Duration
}

type item struct {
	value      interface{}
	expiration time.Time
}

// New creates a new in-memory cache with the given TTL.
func New(ttl time.Duration) *Cache {
	cache := &Cache{
		items: make(map[string]*item),
		ttl:   ttl,
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

// Get retrieves a value from the cache.
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	itm, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(itm.expiration) {
		return nil, false
	}

	return itm.value, true
}

// Set stores a value in the cache.
func (c *Cache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &item{
		value:      value,
		expiration: time.Now().Add(c.ttl),
	}
}

// Delete removes a value from the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Clear removes all items from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*item)
}

// cleanup removes expired items periodically.
func (c *Cache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, itm := range c.items {
			if now.After(itm.expiration) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

// GetOrSet retrieves a value from cache or sets it using the provided function.
func (c *Cache) GetOrSet(key string, fn func() (interface{}, error)) (interface{}, error) {
	// Try to get from cache first
	if val, found := c.Get(key); found {
		return val, nil
	}

	// Not in cache, call function
	val, err := fn()
	if err != nil {
		return nil, err
	}

	// Store in cache
	c.Set(key, val)
	return val, nil
}

// Key generates a cache key from multiple parts.
func Key(parts ...interface{}) string {
	return fmt.Sprintf("%v", parts)
}

// KeyJSON generates a cache key from JSON-serializable data.
func KeyJSON(prefix string, data interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return prefix + ":" + string(jsonData), nil
}

// WithCache wraps a database query with caching.
func WithCache(ctx context.Context, cache *Cache, key string, fn func() (interface{}, error)) (interface{}, error) {
	// Try cache first
	if val, found := cache.Get(key); found {
		return val, nil
	}

	// Execute query
	result, err := fn()
	if err != nil {
		return nil, err
	}

	// Store in cache
	cache.Set(key, result)
	return result, nil
}
