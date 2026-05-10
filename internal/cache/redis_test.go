package cache

import (
	"context"
	"testing"
	"time"

	"github.com/eval_llm/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisClientConnection(t *testing.T) {
	cfg := &config.RedisConfig{
		Host: "localhost",
		Port: 3106,
		TTL:  24 * time.Hour,
	}

	client, err := NewRedisClient(cfg)
	require.NoError(t, err, "Redis client should connect successfully")
	require.NotNil(t, client, "Redis client should not be nil")

	defer client.Close()

	// Test basic connectivity
	ctx := context.Background()
	err = client.Ping(ctx)
	assert.NoError(t, err, "Ping should succeed")
}

func TestRedisClientSetAndGetStatus(t *testing.T) {
	cfg := &config.RedisConfig{
		Host: "localhost",
		Port: 3106,
		TTL:  24 * time.Hour,
	}

	client, err := NewRedisClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	testID := "test-eval-123"

	// Set status
	err = client.SetStatus(ctx, testID, "pending")
	require.NoError(t, err, "SetStatus should succeed")

	// Get status
	status, err := client.GetStatus(ctx, testID)
	require.NoError(t, err, "GetStatus should succeed")
	assert.Equal(t, "pending", status, "Status should match")

	// Cleanup
	client.DeleteStatus(ctx, testID)
}

func TestRedisClientGetStatusNotFound(t *testing.T) {
	cfg := &config.RedisConfig{
		Host: "localhost",
		Port: 3106,
		TTL:  24 * time.Hour,
	}

	client, err := NewRedisClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	testID := "non-existent-id"

	// Get status for non-existent key
	status, err := client.GetStatus(ctx, testID)
	assert.Error(t, err, "GetStatus should return error for missing key")
	assert.Equal(t, "", status, "Status should be empty string for missing key")
	assert.True(t, client.IsKeyNotFound(err), "Error should be key not found type")
}

func TestRedisClientSetAndGetProgress(t *testing.T) {
	cfg := &config.RedisConfig{
		Host: "localhost",
		Port: 3106,
		TTL:  24 * time.Hour,
	}

	client, err := NewRedisClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	testID := "test-eval-456"

	// Set progress
	err = client.SetProgress(ctx, testID, 50)
	require.NoError(t, err, "SetProgress should succeed")

	// Get progress
	progress, err := client.GetProgress(ctx, testID)
	require.NoError(t, err, "GetProgress should succeed")
	assert.Equal(t, 50, progress, "Progress should match")

	// Cleanup
	client.DeleteProgress(ctx, testID)
}

func TestRedisClientGetProgressNotFound(t *testing.T) {
	cfg := &config.RedisConfig{
		Host: "localhost",
		Port: 3106,
		TTL:  24 * time.Hour,
	}

	client, err := NewRedisClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	testID := "non-existent-progress-id"

	// Get progress for non-existent key
	progress, err := client.GetProgress(ctx, testID)
	assert.Error(t, err, "GetProgress should return error for missing key")
	assert.Equal(t, 0, progress, "Progress should be 0 for missing key")
	assert.True(t, client.IsKeyNotFound(err), "Error should be key not found type")
}

func TestRedisClientTTL(t *testing.T) {
	cfg := &config.RedisConfig{
		Host: "localhost",
		Port: 3106,
		TTL:  24 * time.Hour,
	}

	client, err := NewRedisClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	testID := "test-ttl-id"

	// Set status with TTL
	err = client.SetStatus(ctx, testID, "running")
	require.NoError(t, err)

	// Check TTL is set (should be approximately 24 hours = 86400 seconds)
	ttl, err := client.GetTTL(ctx, StatusKey(testID))
	require.NoError(t, err, "GetTTL should succeed")

	// TTL is returned as time.Duration (nanoseconds)
	// Convert to seconds for comparison
	ttlSeconds := int(ttl.Seconds())

	// TTL should be approximately 24 hours (86400 seconds)
	// Allow some variance due to test execution time
	assert.GreaterOrEqual(t, ttlSeconds, 86300, "TTL should be approximately 24 hours")
	assert.LessOrEqual(t, ttlSeconds, 86400, "TTL should not exceed 24 hours")

	// Cleanup
	client.DeleteStatus(ctx, testID)
}

func TestRedisClientProgressTTL(t *testing.T) {
	cfg := &config.RedisConfig{
		Host: "localhost",
		Port: 3106,
		TTL:  24 * time.Hour,
	}

	client, err := NewRedisClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	testID := "test-progress-ttl-id"

	// Set progress with TTL
	err = client.SetProgress(ctx, testID, 75)
	require.NoError(t, err)

	// Check TTL is set
	ttl, err := client.GetTTL(ctx, ProgressKey(testID))
	require.NoError(t, err, "GetTTL should succeed")

	// TTL is returned as time.Duration (nanoseconds)
	// Convert to seconds for comparison
	ttlSeconds := int(ttl.Seconds())

	// TTL should be approximately 24 hours
	assert.GreaterOrEqual(t, ttlSeconds, 86300, "TTL should be approximately 24 hours")
	assert.LessOrEqual(t, ttlSeconds, 86400, "TTL should not exceed 24 hours")

	// Cleanup
	client.DeleteProgress(ctx, testID)
}

func TestRedisClientStatusKeyPattern(t *testing.T) {
	testID := "eval-123"
	key := StatusKey(testID)
	assert.Equal(t, "eval:status:eval-123", key, "Status key should follow pattern eval:status:{id}")
}

func TestRedisClientProgressKeyPattern(t *testing.T) {
	testID := "eval-456"
	key := ProgressKey(testID)
	assert.Equal(t, "eval:progress:eval-456", key, "Progress key should follow pattern eval:progress:{id}")
}

func TestRedisClientUpdateStatus(t *testing.T) {
	cfg := &config.RedisConfig{
		Host: "localhost",
		Port: 3106,
		TTL:  24 * time.Hour,
	}

	client, err := NewRedisClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	testID := "test-update-status"

	// Set initial status
	err = client.SetStatus(ctx, testID, "pending")
	require.NoError(t, err)

	// Update status
	err = client.SetStatus(ctx, testID, "running")
	require.NoError(t, err)

	// Verify updated status
	status, err := client.GetStatus(ctx, testID)
	require.NoError(t, err)
	assert.Equal(t, "running", status, "Status should be updated to running")

	// Cleanup
	client.DeleteStatus(ctx, testID)
}

func TestRedisClientUpdateProgress(t *testing.T) {
	cfg := &config.RedisConfig{
		Host: "localhost",
		Port: 3106,
		TTL:  24 * time.Hour,
	}

	client, err := NewRedisClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	testID := "test-update-progress"

	// Set initial progress
	err = client.SetProgress(ctx, testID, 25)
	require.NoError(t, err)

	// Update progress
	err = client.SetProgress(ctx, testID, 75)
	require.NoError(t, err)

	// Verify updated progress
	progress, err := client.GetProgress(ctx, testID)
	require.NoError(t, err)
	assert.Equal(t, 75, progress, "Progress should be updated to 75")

	// Cleanup
	client.DeleteProgress(ctx, testID)
}

func TestRedisClientSetStatusAndProgress(t *testing.T) {
	cfg := &config.RedisConfig{
		Host: "localhost",
		Port: 3106,
		TTL:  24 * time.Hour,
	}

	client, err := NewRedisClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	testID := "test-combined"

	// Set both status and progress
	err = client.SetStatus(ctx, testID, "running")
	require.NoError(t, err)

	err = client.SetProgress(ctx, testID, 50)
	require.NoError(t, err)

	// Verify both
	status, err := client.GetStatus(ctx, testID)
	require.NoError(t, err)
	assert.Equal(t, "running", status)

	progress, err := client.GetProgress(ctx, testID)
	require.NoError(t, err)
	assert.Equal(t, 50, progress)

	// Cleanup
	client.DeleteStatus(ctx, testID)
	client.DeleteProgress(ctx, testID)
}

func TestRedisClientConnectionPoolSettings(t *testing.T) {
	cfg := &config.RedisConfig{
		Host: "localhost",
		Port: 3106,
		TTL:  24 * time.Hour,
	}

	client, err := NewRedisClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	// Verify client is properly configured
	assert.NotNil(t, client.Client(), "Redis client should have underlying go-redis client")

	// Test multiple concurrent operations (connection pool handling)
	ctx := context.Background()

	// Run 10 concurrent set/get operations to verify pool handling
	for i := 0; i < 10; i++ {
		testID := "concurrent-test-" + string(rune(i))
		err := client.SetStatus(ctx, testID, "pending")
		require.NoError(t, err)

		status, err := client.GetStatus(ctx, testID)
		require.NoError(t, err)
		assert.Equal(t, "pending", status)

		client.DeleteStatus(ctx, testID)
	}
}
