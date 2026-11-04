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

	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/ManuelDavid69/pumpcast/api/graph"
	"github.com/ManuelDavid69/pumpcast/api/internal/cache"
	"github.com/ManuelDavid69/pumpcast/api/internal/config"
	"github.com/ManuelDavid69/pumpcast/api/internal/dataloader"
	"github.com/ManuelDavid69/pumpcast/api/internal/db"
	"github.com/ManuelDavid69/pumpcast/api/internal/health"
	"github.com/ManuelDavid69/pumpcast/api/internal/predictions"
	"github.com/ManuelDavid69/pumpcast/api/internal/prices"
	"github.com/ManuelDavid69/pumpcast/api/internal/ratelimit"
	"github.com/ManuelDavid69/pumpcast/api/internal/stations"
	"github.com/ManuelDavid69/pumpcast/api/internal/stats"
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

	database := mongoClient.Database(cfg.MongoDB)
	if err := db.EnsureSchema(ctx, database); err != nil {
		return err
	}

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
	// Pas de middleware.RealIP : il croit l'en-tête X-Forwarded-For envoyé par
	// le client, ce qui permettrait de contourner le rate limiting
	r.Use(middleware.RequestID, middleware.Logger, middleware.Recoverer)
	// Le front tourne sur une autre origine (localhost:3000, Vercel) : sans
	// CORS le navigateur bloquerait les réponses de l'API
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: cfg.CORSOrigins,
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         600,
	}))
	r.Get("/health", health.Handler(map[string]health.Check{
		"mongo": func(ctx context.Context) error { return mongoClient.Ping(ctx, readpref.Primary()) },
		"redis": func(ctx context.Context) error { return redisClient.Ping(ctx).Err() },
	}))

	current := prices.NewCurrentCache(redisClient)
	preds := predictions.NewRepository(database)
	resolver := &graph.Resolver{
		Stations:    stations.NewRepository(database),
		Current:     current,
		Stats:       stats.NewRepository(database),
		Predictions: preds,
		Redis:       redisClient,
		Now:         time.Now,
	}
	limiter := ratelimit.New(redisClient, cfg.RateLimitPerMinute)
	r.With(limiter.Middleware(cfg.TrustProxy)).Handle("/graphql", graph.NewHandler(resolver, dataloader.Sources{
		Current:     current,
		History:     prices.NewHistoryRepository(database),
		Predictions: preds,
	}))
	// Page web pour tester les requêtes à la main, désactivée en production
	if cfg.EnablePlayground {
		r.Handle("/playground", playground.Handler("PumpCast", "/graphql"))
	}

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
