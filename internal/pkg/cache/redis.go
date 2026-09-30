package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var Ctx = context.Background()

var ErrCacheMiss = errors.New("cache: key not found")

type RedisCache struct {
	client *redis.Client
}

// NewRedisCache initializes a connection to Redis using environment variables.
func NewRedisCache() (*RedisCache, error) {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "127.0.0.1" // default to 127.0.0.1 for clean IPv4 resolution on Windows
	}
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379" // default redis port
	}
	password := os.Getenv("REDIS_PASSWORD") // default empty string

	client := redis.NewClient(&redis.Options{
		Addr:       fmt.Sprintf("%s:%s", host, port),
		Password:   password,
		DB:         0, // use default DB
		MaxRetries: 1,
	})

	// Test connection with timeout
	pingCtx, cancel := context.WithTimeout(Ctx, 2*time.Second)
	defer cancel()

	_, err := client.Ping(pingCtx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisCache{
		client: client,
	}, nil
}

// Client exposes the raw redis client for more advanced operations.
func (c *RedisCache) Client() *redis.Client {
	if c == nil {
		return nil
	}
	return c.client
}

// Set sets a string or primitive value with expiration.
func (c *RedisCache) Set(key string, value interface{}, expiration time.Duration) error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Set(Ctx, key, value, expiration).Err()
}

// Get fetches a string value.
func (c *RedisCache) Get(key string) (string, error) {
	if c == nil || c.client == nil {
		return "", ErrCacheMiss
	}
	val, err := c.client.Get(Ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrCacheMiss
	}
	return val, err
}

// SetJSON marshals any data to JSON and stores it with TTL.
func (c *RedisCache) SetJSON(key string, value interface{}, expiration time.Duration) error {
	if c == nil || c.client == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal error: %w", err)
	}
	err = c.client.Set(Ctx, key, data, expiration).Err()
	if err == nil {
		log.Printf("[REDIS CACHE SET] %s (TTL: %v)", key, expiration)
	}
	return err
}

// GetJSON retrieves JSON data and unmarshals it into dest.
func (c *RedisCache) GetJSON(key string, dest interface{}) error {
	if c == nil || c.client == nil {
		return ErrCacheMiss
	}
	val, err := c.client.Get(Ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		log.Printf("[REDIS CACHE MISS] %s -> fetching from database", key)
		return ErrCacheMiss
	}
	if err != nil {
		return err
	}
	log.Printf("[REDIS CACHE HIT] %s -> served directly from Redis", key)
	return json.Unmarshal([]byte(val), dest)
}

// Delete removes one or more keys safely.
func (c *RedisCache) Delete(keys ...string) error {
	if c == nil || c.client == nil || len(keys) == 0 {
		return nil
	}
	var valid []string
	for _, k := range keys {
		if trimmed := strings.TrimSpace(k); trimmed != "" {
			valid = append(valid, trimmed)
		}
	}
	if len(valid) == 0 {
		return nil
	}
	log.Printf("[REDIS CACHE PURGE] %v", valid)
	return c.client.Del(Ctx, valid...).Err()
}

// Key formatters for strong typing and consistency
func UserDashboardKey(username string) string {
	return "user:dashboard:" + strings.TrimSpace(username)
}

func UserTransactionsKey(username string) string {
	return "user:transactions:" + strings.TrimSpace(username)
}

func MerchantDashboardKey(username string) string {
	return "merchant:dashboard:" + strings.TrimSpace(username)
}

func MerchantTransactionsKey(username string) string {
	return "merchant:transactions:" + strings.TrimSpace(username)
}

func AdminDashboardKey() string {
	return "admin:dashboard"
}

// High-level invalidation helpers

// InvalidateUser deletes all caches related to a user.
func (c *RedisCache) InvalidateUser(username string) {
	if c == nil || c.client == nil || username == "" {
		return
	}
	_ = c.Delete(UserDashboardKey(username), UserTransactionsKey(username))
}

// InvalidateMerchant deletes all caches related to a merchant.
func (c *RedisCache) InvalidateMerchant(username string) {
	if c == nil || c.client == nil || username == "" {
		return
	}
	_ = c.Delete(MerchantDashboardKey(username), MerchantTransactionsKey(username))
}

// InvalidateAdmin deletes the admin aggregate dashboard cache.
func (c *RedisCache) InvalidateAdmin() {
	if c == nil || c.client == nil {
		return
	}
	_ = c.Delete(AdminDashboardKey())
}
