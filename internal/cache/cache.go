package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/eval_llm/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

// ErrRedisUnavailable indicates Redis is not available
var ErrRedisUnavailable = errors.New("redis unavailable")

// StatusCache interface for status caching
type StatusCache interface {
	SetStatus(ctx context.Context, evalID string, status string) error
	GetStatus(ctx context.Context, evalID string) (string, error)
	SetProgress(ctx context.Context, evalID string, progress int) error
	GetProgress(ctx context.Context, evalID string) (int, error)
	DeleteStatus(ctx context.Context, evalID string) error
	DeleteProgress(ctx context.Context, evalID string) error
	Ping(ctx context.Context) error
	Close() error
	// IsAvailable returns true if Redis is currently reachable
	IsAvailable(ctx context.Context) bool
}

// RedisClient implements StatusCache with Redis backend
type RedisClient struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisClient creates a new Redis client with connection pool
func NewRedisClient(cfg *config.RedisConfig) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		// Connection pool settings:
		// PoolSize: default is 10 connections per CPU core
		// MinIdleConns: default is 0
		// MaxRetries: default is 3
		// PoolTimeout: default is 4 seconds
	})

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{
		client: client,
		ttl:    cfg.TTL,
	}, nil
}

// StatusKey returns the Redis key for status with pattern: eval:status:{id}
func StatusKey(evalID string) string {
	return fmt.Sprintf("eval:status:%s", evalID)
}

// ProgressKey returns the Redis key for progress with pattern: eval:progress:{id}
func ProgressKey(evalID string) string {
	return fmt.Sprintf("eval:progress:%s", evalID)
}

// SetStatus stores the evaluation status with TTL
func (r *RedisClient) SetStatus(ctx context.Context, evalID string, status string) error {
	key := StatusKey(evalID)
	return r.client.Set(ctx, key, status, r.ttl).Err()
}

// GetStatus retrieves the evaluation status
func (r *RedisClient) GetStatus(ctx context.Context, evalID string) (string, error) {
	key := StatusKey(evalID)
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return val, nil
}

// SetProgress stores the evaluation progress (0-100) with TTL
func (r *RedisClient) SetProgress(ctx context.Context, evalID string, progress int) error {
	key := ProgressKey(evalID)
	return r.client.Set(ctx, key, strconv.Itoa(progress), r.ttl).Err()
}

// GetProgress retrieves the evaluation progress
func (r *RedisClient) GetProgress(ctx context.Context, evalID string) (int, error) {
	key := ProgressKey(evalID)
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	progress, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid progress value: %w", err)
	}
	return progress, nil
}

// DeleteStatus removes the status key
func (r *RedisClient) DeleteStatus(ctx context.Context, evalID string) error {
	key := StatusKey(evalID)
	return r.client.Del(ctx, key).Err()
}

// DeleteProgress removes the progress key
func (r *RedisClient) DeleteProgress(ctx context.Context, evalID string) error {
	key := ProgressKey(evalID)
	return r.client.Del(ctx, key).Err()
}

// GetTTL returns the TTL for a key in seconds
func (r *RedisClient) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// Ping verifies Redis connectivity
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// Client returns the underlying go-redis client for advanced operations
func (r *RedisClient) Client() *redis.Client {
	return r.client
}

// IsKeyNotFound checks if the error is a Redis Nil error (key not found)
func (r *RedisClient) IsKeyNotFound(err error) bool {
	return errors.Is(err, redis.Nil)
}

// IsAvailable returns true if Redis is currently reachable
// This is used to implement fallback to K8s polling when Redis is unavailable
func (r *RedisClient) IsAvailable(ctx context.Context) bool {
	err := r.client.Ping(ctx).Err()
	return err == nil
}
