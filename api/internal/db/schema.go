package db

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnsureSchema crée les collections spéciales et les index s'ils n'existent pas.
// On peut l'appeler à chaque démarrage : MongoDB ne fait rien si tout est déjà là.
func EnsureSchema(ctx context.Context, database *mongo.Database) error {
	if err := ensurePricesTimeSeries(ctx, database); err != nil {
		return err
	}
	return ensureIndexes(ctx, database)
}

// La collection des prix doit être créée explicitement en time series :
// si on insère directement, MongoDB crée une collection normale et on ne
// peut plus la convertir après.
func ensurePricesTimeSeries(ctx context.Context, database *mongo.Database) error {
	names, err := database.ListCollectionNames(ctx, bson.D{{Key: "name", Value: "prices"}})
	if err != nil {
		return fmt.Errorf("liste des collections : %w", err)
	}
	if len(names) > 0 {
		return nil
	}

	// Un relevé par jour : avec la granularité "hours", MongoDB regroupe
	// les relevés d'une même série dans des blocs de 30 jours.
	opts := options.CreateCollection().SetTimeSeriesOptions(
		options.TimeSeries().
			SetTimeField("ts").
			SetMetaField("meta").
			SetGranularity("hours"),
	)
	if err := database.CreateCollection(ctx, "prices", opts); err != nil {
		return fmt.Errorf("création de la collection prices : %w", err)
	}
	return nil
}

func ensureIndexes(ctx context.Context, database *mongo.Database) error {
	indexes := map[string][]mongo.IndexModel{
		"stations": {
			// Index géospatial : permet $near / $geoNear pour "les stations autour de moi"
			{Keys: bson.D{{Key: "location", Value: "2dsphere"}}},
			{Keys: bson.D{{Key: "department", Value: 1}}},
		},
		"prices": {
			// Historique d'une station pour un carburant, trié par date
			{Keys: bson.D{{Key: "meta.station_id", Value: 1}, {Key: "meta.fuel", Value: 1}, {Key: "ts", Value: 1}}},
		},
	}

	for coll, models := range indexes {
		if _, err := database.Collection(coll).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("index de %s : %w", coll, err)
		}
	}
	return nil
}
