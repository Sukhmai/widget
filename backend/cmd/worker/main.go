package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/sukhmai/widget/backend/internal/db"
	"github.com/sukhmai/widget/backend/internal/postgres"
	"github.com/sukhmai/widget/backend/internal/push"
	"github.com/sukhmai/widget/backend/internal/resource"
)

func main() {
	_ = godotenv.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	pool, err := postgres.New(ctx, dsn)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()

	// Push is a best-effort optimization on top of the widget's own poll
	// fallback, so missing APNs credentials must not be fatal to the worker —
	// just run without it.
	var pushWorker *push.Worker
	apnsKeyPath := os.Getenv("APNS_KEY_PATH")
	apnsKeyID := os.Getenv("APNS_KEY_ID")
	apnsTeamID := os.Getenv("APNS_TEAM_ID")
	apnsTopic := os.Getenv("APNS_TOPIC")
	if apnsKeyPath == "" || apnsKeyID == "" || apnsTeamID == "" || apnsTopic == "" {
		slog.Warn("push worker disabled: APNS_KEY_PATH, APNS_KEY_ID, APNS_TEAM_ID, and APNS_TOPIC are not all set; widgets will rely on their poll fallback only")
	} else {
		apnsProduction := os.Getenv("APNS_PRODUCTION") == "true"
		apnsClient, err := push.NewClient(apnsKeyPath, apnsKeyID, apnsTeamID, apnsTopic, apnsProduction)
		if err != nil {
			slog.Warn("push worker disabled: failed to build APNs client", "err", err)
		} else {
			pushWorker = push.NewWorker(rdb, db.New(pool), apnsClient)
		}
	}

	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		log.Fatal("SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET are required")
	}

	tmdbToken := os.Getenv("TMDB_ACCESS_TOKEN")
	if tmdbToken == "" {
		log.Fatal("TMDB_ACCESS_TOKEN is required")
	}


	worker := resource.NewWorker(rdb, db.New(pool))
	for source, fetcher := range resource.BuildFetchers(resource.FetcherConfig{
		SpotifyClientID:     clientID,
		SpotifyClientSecret: clientSecret,
		TMDBAccessToken:     tmdbToken,
	}) {
		worker = worker.WithFetcher(source, fetcher)
	}

	if pushWorker != nil {
		go func() {
			slog.Info("push worker starting")
			if err := pushWorker.Run(ctx); err != nil {
				slog.Error("push worker stopped", "err", err)
			}
		}()
	}

	slog.Info("resource worker starting")
	if err := worker.Run(ctx); err != nil {
		log.Fatalf("resource worker: %v", err)
	}
}
