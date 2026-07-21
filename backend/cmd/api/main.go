package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/sukhmai/widget/backend/internal/atom"
	"github.com/sukhmai/widget/backend/internal/db"
	"github.com/sukhmai/widget/backend/internal/gen/atom/v1/atomv1connect"
	"github.com/sukhmai/widget/backend/internal/gen/user/v1/userv1connect"
	"github.com/sukhmai/widget/backend/internal/postgres"
	"github.com/sukhmai/widget/backend/internal/user"
)

func main() {
	_ = godotenv.Load() // loads .env if present; no-op in production

	ctx := context.Background()

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

	queries := db.New(pool)

	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Mount the AtomService connect handler at its RPC path prefix.
	atomPath, atomHandler := atomv1connect.NewAtomServiceHandler(atom.New(queries, rdb))
	r.Mount(atomPath, atomHandler)

	// Mount the UserService connect handler (registration + contact matching).
	userPath, userHandler := userv1connect.NewUserServiceHandler(user.New(queries))
	r.Mount(userPath, userHandler)

	log.Fatal(http.ListenAndServe(":8080", r))
}
