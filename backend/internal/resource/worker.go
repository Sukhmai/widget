package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/sukhmai/widget/backend/internal/db"
)

// Fetcher fetches display metadata for a resource URL and returns it as raw
// JSON ready to be stored in the metadata column.
type Fetcher interface {
	Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error)
}

// Worker pops fetch jobs from Redis and writes metadata back to the DB.
type Worker struct {
	rdb      *redis.Client
	queries  *db.Queries
	fetchers map[Source]Fetcher
}

// NewWorker creates a Worker. Register source fetchers via WithFetcher before
// calling Run.
func NewWorker(rdb *redis.Client, queries *db.Queries) *Worker {
	return &Worker{
		rdb:      rdb,
		queries:  queries,
		fetchers: make(map[Source]Fetcher),
	}
}

// WithFetcher registers a Fetcher for the given Source.
func (w *Worker) WithFetcher(source Source, f Fetcher) *Worker {
	w.fetchers[source] = f
	return w
}

// Run blocks, processing fetch jobs until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		results, err := w.rdb.BRPop(ctx, 5*time.Second, fetchQueueKey).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("resource worker: brpop: %w", err)
		}

		// BRPop returns [key, value]
		if err := w.process(ctx, results[1]); err != nil {
			slog.Error("resource worker: failed to process job", "err", err)
		}
	}
}

func (w *Worker) process(ctx context.Context, payload string) error {
	var job FetchJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return fmt.Errorf("unmarshal job: %w", err)
	}

	var id pgtype.UUID
	if err := id.Scan(job.ResourceID); err != nil {
		return fmt.Errorf("parse resource id %q: %w", job.ResourceID, err)
	}

	resource, err := w.queries.GetResourceByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get resource %q: %w", job.ResourceID, err)
	}

	source, err := parseSource(resource.Source)
	if err != nil {
		return err
	}

	fetcher, ok := w.fetchers[source]
	if !ok {
		slog.Info("resource worker: no fetcher registered, skipping", "source", source)
		return nil
	}

	metadata, err := fetcher.Fetch(ctx, resource.Url)
	if err != nil {
		return fmt.Errorf("fetch %q: %w", resource.Url, err)
	}

	if _, err := w.queries.UpdateResourceMetadata(ctx, db.UpdateResourceMetadataParams{
		ID:       id,
		Metadata: metadata,
	}); err != nil {
		return fmt.Errorf("update metadata for %q: %w", job.ResourceID, err)
	}

	slog.Info("resource worker: metadata fetched", "resource_id", job.ResourceID, "source", source)
	return nil
}
