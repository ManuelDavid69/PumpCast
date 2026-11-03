// Point d'entrée de l'API PumpCast.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/ManuelDavid69/pumpcast/api/internal/cache"
	"github.com/ManuelDavid69/pumpcast/api/internal/config"
	"github.com/ManuelDavid69/pumpcast/api/internal/db"
	"github.com/ManuelDavid69/pumpcast/api/internal/health"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// ctx est annulé quand on reçoit Ctrl+C ou le SIGTERM de Docker
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	mongoClient, err := db.Connect(ctx, cfg.MongoURI)
	if err != nil {
		return err
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			log.Printf("fermeture mongo : %v", err)
		}
	}()

	redisClient, err := cache.Connect(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("fermeture redis : %v", err)
		}
	}()

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	r.Get("/health", health.Handler(map[string]health.Check{
		"mongo": func(ctx context.Context) error { return mongoClient.Ping(ctx, readpref.Primary()) },
		"redis": func(ctx context.Context) error { return redisClient.Ping(ctx).Err() },
	}))

	srv := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("API démarrée sur le port %s", cfg.APIPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	// On laisse 10 s aux requêtes en cours pour se terminer
	log.Println("arrêt de l'API...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
