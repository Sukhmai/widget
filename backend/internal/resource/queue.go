package resource

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const fetchQueueKey = "resource:fetch"

// FetchJob is the message pushed onto the Redis queue when a resource
// row is created and its metadata has not yet been fetched.
type FetchJob struct {
	ResourceID string `json:"resource_id"`
}

// Enqueue pushes a fetch job onto the Redis queue.
func Enqueue(ctx context.Context, rdb *redis.Client, resourceID string) error {
	job := FetchJob{ResourceID: resourceID}
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("resource: marshal fetch job: %w", err)
	}
	return rdb.LPush(ctx, fetchQueueKey, payload).Err()
}
