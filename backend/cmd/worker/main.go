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

	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		log.Fatal("SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET are required")
	}

	tmdbToken := os.Getenv("TMDB_ACCESS_TOKEN")
	if tmdbToken == "" {
		log.Fatal("TMDB_ACCESS_TOKEN is required")
	}


	// appleMusicKeyID := os.Getenv("APPLE_MUSIC_KEY_ID")
	// appleMusicTeamID := os.Getenv("APPLE_MUSIC_TEAM_ID")
	// appleMusicPrivateKey := os.Getenv("APPLE_MUSIC_PRIVATE_KEY")
	// if appleMusicKeyID == "" || appleMusicTeamID == "" || appleMusicPrivateKey == "" {
	// 	log.Fatal("APPLE_MUSIC_KEY_ID, APPLE_MUSIC_TEAM_ID, and APPLE_MUSIC_PRIVATE_KEY are required")
	// }
	// appleMusicFetcher, err := resource.NewAppleMusicFetcher(appleMusicTeamID, appleMusicKeyID, appleMusicPrivateKey)
	// if err != nil {
	// 	log.Fatalf("apple music: %v", err)
	// }

	worker := resource.NewWorker(rdb, db.New(pool)).
		WithFetcher(resource.SourceSpotify, resource.NewSpotifyFetcher(clientID, clientSecret)).
		WithFetcher(resource.SourceLetterboxd, resource.NewLetterboxdFetcher(tmdbToken)).
		WithFetcher(resource.SourceIMDB, resource.NewIMDBFetcher(tmdbToken)).
		WithFetcher(resource.SourceGoodreads, resource.NewGoodreadsFetcher()).
		WithFetcher(resource.SourceX, resource.NewXFetcher()).
		WithFetcher(resource.SourceYouTube, resource.NewYouTubeFetcher()).
		WithFetcher(resource.SourceWikipedia, resource.NewWikipediaFetcher()).
		WithFetcher(resource.SourceSteam, resource.NewSteamFetcher()).
		WithFetcher(resource.SourceSubstack, resource.NewSubstackFetcher()).
		// WithFetcher(resource.SourceAppleMusic, appleMusicFetcher).
		WithFetcher(resource.SourceReddit, resource.NewRedditFetcher()).
		WithFetcher(resource.SourceTikTok, resource.NewTikTokFetcher()).
		WithFetcher(resource.SourceMyAnimeList, resource.NewMyAnimeListFetcher()).
		WithFetcher(resource.SourceAniList, resource.NewAniListFetcher()).
		WithFetcher(resource.SourceBluesky, resource.NewBlueskyFetcher()).
		// WithFetcher(resource.SourceInstagram, resource.NewInstagramFetcher()).

		WithFetcher(resource.SourceGeneric, resource.NewGenericFetcher())

	slog.Info("resource worker starting")
	if err := worker.Run(ctx); err != nil {
		log.Fatalf("resource worker: %v", err)
	}
}
