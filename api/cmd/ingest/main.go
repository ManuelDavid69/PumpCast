// Programme d'ingestion : télécharge le flux officiel et met à jour
// les stations, l'historique des prix et le cache Redis.
//
// Usage :
//
//	go run ./cmd/ingest              # télécharge le flux
//	go run ./cmd/ingest -file x.zip  # lit un fichier local (zip ou xml)
package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/ManuelDavid69/pumpcast/api/internal/cache"
	"github.com/ManuelDavid69/pumpcast/api/internal/config"
	"github.com/ManuelDavid69/pumpcast/api/internal/db"
	"github.com/ManuelDavid69/pumpcast/api/internal/ingest"
	"github.com/ManuelDavid69/pumpcast/api/internal/prices"
	"github.com/ManuelDavid69/pumpcast/api/internal/stations"
)

func main() {
	file := flag.String("file", "", "fichier local à lire au lieu de télécharger le flux")
	flag.Parse()

	if err := run(*file); err != nil {
		log.Fatalf("ingestion échouée : %v", err)
	}
}

func run(file string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	start := time.Now()
	var xmlData []byte
	if file != "" {
		xmlData, err = ingest.ReadFile(file)
	} else {
		xmlData, err = ingest.Download(ctx, cfg.FeedURL)
	}
	if err != nil {
		return err
	}
	log.Printf("flux récupéré (%d Ko)", len(xmlData)/1024)

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

	redisClient, err := cache.Connect(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("fermeture redis : %v", err)
		}
	}()

	sum, err := ingest.Run(ctx, xmlData, ingest.Deps{
		Stations:    stations.NewRepository(database),
		History:     prices.NewHistoryRepository(database),
		Current:     prices.NewCurrentCache(redisClient),
		KeepHistory: cfg.KeepHistory,
		Now:         time.Now,
	})
	if err != nil {
		return err
	}
	log.Printf("%s (en %s)", sum, time.Since(start).Round(time.Millisecond))
	return nil
}
