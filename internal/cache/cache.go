package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache interfaces will be implemented in future features
// This placeholder imports go-redis to ensure dependency is tracked

// StatusCache interface for status caching
type StatusCache interface {
	SetStatus(ctx context.Context, key string, status string, ttl time.Duration) error
	GetStatus(ctx context.Context, key string) (string, error)
}

// RedisCache placeholder implementation
type RedisCache struct {
	client *redis.Client
}

// Ensure redis types are referenced for go mod
var _ redis.Cmdable
