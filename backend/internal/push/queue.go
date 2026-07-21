// Package push fans out a silent APNs notification to a poster's subscribers
// so their widgets can refresh immediately instead of waiting on the next
// poll, and sends it via a Redis-queued worker mirroring internal/resource.
package push

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const fanoutQueueKey = "push:fanout"

// FanoutJob is the message pushed onto the Redis queue when a user posts a
// new atom, so their subscribers can be notified.
type FanoutJob struct {
	PosterID string `json:"poster_id"`
}

// Enqueue pushes a fan-out job onto the Redis queue.
func Enqueue(ctx context.Context, rdb *redis.Client, posterID string) error {
	job := FanoutJob{PosterID: posterID}
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("push: marshal fanout job: %w", err)
	}
	return rdb.LPush(ctx, fanoutQueueKey, payload).Err()
}
