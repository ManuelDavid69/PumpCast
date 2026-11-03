// Package prices gère l'historique des prix (MongoDB) et les prix actuels (Redis).
package prices

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const CollectionName = "prices"

// Meta regroupe ce qui identifie une série : une station et un carburant.
// Dans une collection time series, MongoDB range ensemble les mesures
// qui ont le même meta, c'est ce qui rend le stockage compact.
type Meta struct {
	StationID string `bson:"station_id"`
	Fuel      string `bson:"fuel"`
}

// DailyPrice est le prix affiché par une station pour un carburant un jour donné.
type DailyPrice struct {
	TS    time.Time `bson:"ts"`
	Meta  Meta      `bson:"meta"`
	Price float64   `bson:"price"`
}

// Day renvoie la date du jour sous forme de minuit UTC.
// On stocke une date "calendaire" (le 3 novembre) et pas un instant précis,
// sinon le décalage Paris/UTC ferait changer de jour certains relevés.
func Day(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

type HistoryRepository struct {
	coll *mongo.Collection
}

func NewHistoryRepository(db *mongo.Database) *HistoryRepository {
	return &HistoryRepository{coll: db.Collection(CollectionName)}
}

// SaveResult résume ce qui a été écrit.
type SaveResult struct {
	Inserted  int
	Updated   int
	Unchanged int
}

// SaveDay enregistre les prix d'une journée. On garde un seul relevé par
// station, carburant et jour : l'ingestion tourne plusieurs fois par jour,
// donc le dernier passage écrase le prix s'il a changé.
//
// Les collections time series ne gèrent pas l'upsert, et MongoDB refuse
// un update dont le filtre porte sur ts (testé sur la 8.0). Par contre la
// suppression marche : un prix qui a changé est donc supprimé puis réinséré.
func (r *HistoryRepository) SaveDay(ctx context.Context, day time.Time, list []DailyPrice) (SaveResult, error) {
	var res SaveResult
	if len(list) == 0 {
		return res, nil
	}

	existing, err := r.pricesOfDay(ctx, day)
	if err != nil {
		return res, err
	}

	var deletes, inserts []mongo.WriteModel
	for _, p := range list {
		p.TS = day
		old, found := existing[p.Meta]
		switch {
		case !found:
			inserts = append(inserts, mongo.NewInsertOneModel().SetDocument(p))
			res.Inserted++
		case old != p.Price:
			deletes = append(deletes, mongo.NewDeleteManyModel().SetFilter(bson.D{
				{Key: "ts", Value: day},
				{Key: "meta.station_id", Value: p.Meta.StationID},
				{Key: "meta.fuel", Value: p.Meta.Fuel},
			}))
			inserts = append(inserts, mongo.NewInsertOneModel().SetDocument(p))
			res.Updated++
		default:
			res.Unchanged++
		}
	}

	// Deux bulk séparés pour être sûr que les suppressions passent avant les
	// insertions. Si ça plante entre les deux, le relevé manque juste jusqu'à
	// la prochaine ingestion, qui le réinsère.
	for _, models := range [][]mongo.WriteModel{deletes, inserts} {
		if len(models) == 0 {
			continue
		}
		if _, err := r.coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false)); err != nil {
			return res, fmt.Errorf("écriture des prix du %s : %w", day.Format("2006-01-02"), err)
		}
	}
	return res, nil
}

func (r *HistoryRepository) pricesOfDay(ctx context.Context, day time.Time) (map[Meta]float64, error) {
	cur, err := r.coll.Find(ctx, bson.D{{Key: "ts", Value: day}},
		options.Find().SetProjection(bson.D{{Key: "meta", Value: 1}, {Key: "price", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("lecture des prix du jour : %w", err)
	}
	var docs []DailyPrice
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("lecture des prix du jour : %w", err)
	}

	m := make(map[Meta]float64, len(docs))
	for _, d := range docs {
		m[d.Meta] = d.Price
	}
	return m, nil
}
