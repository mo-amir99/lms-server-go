package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client defines the interface for cache operations.
type Client interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
	Increment(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
	Close() error
}

// RedisClient is a wrapper around the Redis client.
type RedisClient struct {
	client  *redis.Client
	enabled bool
}

// NewRedisClient creates a new Redis cache client.
func NewRedisClient(addr, password string, db int) (*RedisClient, error) {
	if addr == "" {
		// Return disabled client if no address provided
		return &RedisClient{enabled: false}, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisClient{
		client:  client,
		enabled: true,
	}, nil
}

// Get retrieves a value from cache.
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	if !r.enabled {
		return "", fmt.Errorf("cache not enabled")
	}

	return r.client.Get(ctx, key).Result()
}

// Set stores a value in cache with expiration.
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if !r.enabled {
		return nil // Silently skip if cache is not enabled
	}

	return r.client.Set(ctx, key, value, expiration).Err()
}

// Delete removes keys from cache.
func (r *RedisClient) Delete(ctx context.Context, keys ...string) error {
	if !r.enabled {
		return nil
	}

	return r.client.Del(ctx, keys...).Err()
}

// Exists checks if keys exist in cache.
func (r *RedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	if !r.enabled {
		return 0, nil
	}

	return r.client.Exists(ctx, keys...).Result()
}

// Increment increments a counter in cache.
func (r *RedisClient) Increment(ctx context.Context, key string) (int64, error) {
	if !r.enabled {
		return 0, nil
	}

	return r.client.Incr(ctx, key).Result()
}

// Expire sets an expiration on a key.
func (r *RedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	if !r.enabled {
		return nil
	}

	return r.client.Expire(ctx, key, expiration).Err()
}

// Close closes the Redis connection.
func (r *RedisClient) Close() error {
	if !r.enabled {
		return nil
	}

	return r.client.Close()
}

// SetJSON stores a JSON-serialized value in cache.
func (r *RedisClient) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	return r.Set(ctx, key, string(data), expiration)
}

// GetJSON retrieves and deserializes a JSON value from cache.
func (r *RedisClient) GetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := r.Get(ctx, key)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return fmt.Errorf("failed to unmarshal value: %w", err)
	}

	return nil
}

// MemoryCache is an in-memory cache implementation for development/testing.
type MemoryCache struct {
	store map[string]cacheItem
}

type cacheItem struct {
	value      string
	expiration time.Time
}

// NewMemoryCache creates a new in-memory cache (for development/testing).
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		store: make(map[string]cacheItem),
	}
}

// Get retrieves a value from memory cache.
func (m *MemoryCache) Get(ctx context.Context, key string) (string, error) {
	item, exists := m.store[key]
	if !exists {
		return "", fmt.Errorf("key not found")
	}

	if time.Now().After(item.expiration) {
		delete(m.store, key)
		return "", fmt.Errorf("key expired")
	}

	return item.value, nil
}

// Set stores a value in memory cache.
func (m *MemoryCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	case []byte:
		strValue = string(v)
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
		strValue = string(data)
	}

	exp := time.Now().Add(expiration)
	if expiration == 0 {
		exp = time.Now().Add(24 * time.Hour) // Default 24 hour expiration
	}

	m.store[key] = cacheItem{
		value:      strValue,
		expiration: exp,
	}

	return nil
}

// Delete removes keys from memory cache.
func (m *MemoryCache) Delete(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		delete(m.store, key)
	}
	return nil
}

// Exists checks if keys exist in memory cache.
func (m *MemoryCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	count := int64(0)
	for _, key := range keys {
		if _, exists := m.store[key]; exists {
			count++
		}
	}
	return count, nil
}

// Increment increments a counter in memory cache.
func (m *MemoryCache) Increment(ctx context.Context, key string) (int64, error) {
	item, exists := m.store[key]
	if !exists {
		m.Set(ctx, key, "1", 24*time.Hour)
		return 1, nil
	}

	// Try to parse as int64
	var current int64
	fmt.Sscanf(item.value, "%d", &current)
	current++

	m.Set(ctx, key, fmt.Sprintf("%d", current), 24*time.Hour)
	return current, nil
}

// Expire sets an expiration on a key in memory cache.
func (m *MemoryCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	item, exists := m.store[key]
	if !exists {
		return fmt.Errorf("key not found")
	}

	item.expiration = time.Now().Add(expiration)
	m.store[key] = item
	return nil
}

// Close is a no-op for memory cache.
func (m *MemoryCache) Close() error {
	m.store = make(map[string]cacheItem)
	return nil
}
