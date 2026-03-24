// Package jobs provides the Redis-backed job queue.
package jobs

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const defaultQueueKey = "synt:jobs"

// RedisQueue implements the orchestrator.Queue interface using Redis.
type RedisQueue struct {
	client *redis.Client
	key    string
}

// NewRedisQueue creates a new Redis-backed queue.
func NewRedisQueue(client *redis.Client) *RedisQueue {
	return &RedisQueue{client: client, key: defaultQueueKey}
}

// Enqueue pushes a job to the right side of the queue list.
func (q *RedisQueue) Enqueue(ctx context.Context, jobType string, payload []byte) error {
	value := fmt.Sprintf(`{"job_type":%q,"payload":%s}`, jobType, string(payload))
	return q.client.RPush(ctx, q.key, value).Err()
}

// Dequeue pops a job from the left side (FIFO).
func (q *RedisQueue) Dequeue(ctx context.Context) (string, error) {
	result, err := q.client.LPop(ctx, q.key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return result, err
}

// Len returns the number of jobs in the queue.
func (q *RedisQueue) Len(ctx context.Context) (int64, error) {
	return q.client.LLen(ctx, q.key).Result()
}

// NoOpQueue is a queue that discards all jobs (for testing).
type NoOpQueue struct{}

// NewNoOpQueue creates a new no-op queue.
func NewNoOpQueue() *NoOpQueue {
	return &NoOpQueue{}
}

// Enqueue is a no-op.
func (q *NoOpQueue) Enqueue(_ context.Context, _ string, _ []byte) error {
	return nil
}
