// Import des archives annuelles des prix, pour avoir de l'historique
// dès le départ sans attendre des mois de collecte.
//
// À lancer une fois (ou pour rattraper un trou). Les années sont traitées
// dans l'ordre pour reporter le dernier prix de décembre sur janvier.
//
// Usage :
//
//	go run ./cmd/importhistory -from 2023 -to 2026
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/ManuelDavid69/pumpcast/api/internal/archive"
	"github.com/ManuelDavid69/pumpcast/api/internal/config"
	"github.com/ManuelDavid69/pumpcast/api/internal/db"
	"github.com/ManuelDavid69/pumpcast/api/internal/ingest"
	"github.com/ManuelDavid69/pumpcast/api/internal/prices"
	"github.com/ManuelDavid69/pumpcast/api/internal/stations"
)

func main() {
	thisYear := time.Now().In(ingest.Paris).Year()
	// 2022 est évitée par défaut : la remise carburant de l'État (avril à décembre)
	// crée des sauts de prix artificiels qui gêneraient le modèle
	from := flag.Int("from", thisYear-3, "première année à importer")
	to := flag.Int("to", thisYear, "dernière année à importer")
	dir := flag.String("dir", "../data/archives", "dossier où garder les zips téléchargés")
	flag.Parse()

	if err := run(*from, *to, *dir, thisYear); err != nil {
		log.Fatalf("import échoué : %v", err)
	}
}

func run(from, to int, dir string, thisYear int) error {
	if from > to || from < 2007 || to > thisYear {
		return fmt.Errorf("années invalides : de %d à %d (archives disponibles de 2007 à %d)", from, to, thisYear)
	}

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

	im := &archive.Importer{
		History:     prices.NewHistoryRepository(database),
		Stations:    stations.NewRepository(database),
		KeepHistory: cfg.KeepHistory,
	}
	// Aujourd'hui est géré par l'ingestion du flux instantané
	yesterday := prices.Day(time.Now(), ingest.Paris).AddDate(0, 0, -1)
	carry := archive.Carry{}

	for year := from; year <= to; year++ {
		start := time.Now()
		path, err := archive.Fetch(ctx, dir, year, year == thisYear)
		if err != nil {
			return err
		}
		r, closeXML, err := archive.OpenXML(path)
		if err != nil {
			return err
		}
		sum, err := im.ImportYear(ctx, r, year, carry, yesterday)
		if closeErr := closeXML(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		log.Printf("%s (en %s)", sum, time.Since(start).Round(time.Second))
	}
	return nil
}
