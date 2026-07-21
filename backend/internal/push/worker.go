package push

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

// Worker pops fan-out jobs from Redis and sends a silent push to every
// subscriber of the poster who has a registered push token.
type Worker struct {
	rdb     *redis.Client
	queries *db.Queries
	client  *Client
}

// NewWorker creates a push fan-out Worker.
func NewWorker(rdb *redis.Client, queries *db.Queries, client *Client) *Worker {
	return &Worker{rdb: rdb, queries: queries, client: client}
}

// Run blocks, processing fan-out jobs until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		results, err := w.rdb.BRPop(ctx, 5*time.Second, fanoutQueueKey).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("push worker: brpop: %w", err)
		}

		// BRPop returns [key, value]
		if err := w.process(ctx, results[1]); err != nil {
			slog.Error("push worker: failed to process job", "err", err)
		}
	}
}

func (w *Worker) process(ctx context.Context, payload string) error {
	var job FanoutJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return fmt.Errorf("unmarshal job: %w", err)
	}

	var posterID pgtype.UUID
	if err := posterID.Scan(job.PosterID); err != nil {
		return fmt.Errorf("parse poster id %q: %w", job.PosterID, err)
	}

	tokens, err := w.queries.ListSubscriberPushTokens(ctx, posterID)
	if err != nil {
		return fmt.Errorf("list subscribers of %q: %w", job.PosterID, err)
	}

	sent := 0
	for _, t := range tokens {
		if !t.Valid {
			continue
		}
		if err := w.client.SendSilent(t.String); err != nil {
			slog.Error("push worker: failed to send", "err", err)
			continue
		}
		sent++
	}

	slog.Info("push worker: fanout complete", "poster_id", job.PosterID, "subscribers", len(tokens), "sent", sent)
	return nil
}
