// Vérifie les alertes des utilisateurs et envoie les emails.
//
// À lancer après les prévisions du jour (et après les ingestions pour les
// alertes de seuil) : go run ./cmd/alerts
package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/ManuelDavid69/pumpcast/api/internal/alerts"
	"github.com/ManuelDavid69/pumpcast/api/internal/cache"
	"github.com/ManuelDavid69/pumpcast/api/internal/config"
	"github.com/ManuelDavid69/pumpcast/api/internal/db"
	"github.com/ManuelDavid69/pumpcast/api/internal/predictions"
	"github.com/ManuelDavid69/pumpcast/api/internal/prices"
	"github.com/ManuelDavid69/pumpcast/api/internal/stations"
	"github.com/ManuelDavid69/pumpcast/api/internal/users"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("vérification des alertes échouée : %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	sender, err := newSender(cfg)
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

	redisClient, err := cache.Connect(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("fermeture redis : %v", err)
		}
	}()

	checker := &alerts.Checker{
		Alerts:      alerts.NewRepository(database),
		Users:       users.NewRepository(database),
		Stations:    stations.NewRepository(database),
		Current:     prices.NewCurrentCache(redisClient),
		Predictions: predictions.NewRepository(database),
		Sender:      sender,
		SiteURL:     cfg.SiteURL,
		Now:         time.Now,
	}
	sum, err := checker.Run(ctx)
	if err != nil {
		return err
	}
	log.Print(sum)
	return nil
}

func newSender(cfg config.Config) (alerts.Sender, error) {
	switch cfg.EmailProvider {
	case "log":
		return alerts.LogSender{}, nil
	case "brevo":
		if cfg.BrevoAPIKey == "" || cfg.AlertFromEmail == "" {
			return nil, errors.New("EMAIL_PROVIDER=brevo demande BREVO_API_KEY et ALERT_FROM_EMAIL")
		}
		return alerts.BrevoSender{APIKey: cfg.BrevoAPIKey, FromEmail: cfg.AlertFromEmail, FromName: "PumpCast"}, nil
	}
	return nil, errors.New("EMAIL_PROVIDER doit valoir log ou brevo")
}
