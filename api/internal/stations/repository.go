// Package stations gère la collection des stations.
package stations

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const CollectionName = "stations"

// GeoPoint est un point GeoJSON. MongoDB attend [longitude, latitude],
// dans cet ordre, pour que l'index 2dsphere fonctionne.
type GeoPoint struct {
	Type        string    `bson:"type"`
	Coordinates []float64 `bson:"coordinates"`
}

func NewPoint(lat, lng float64) *GeoPoint {
	return &GeoPoint{Type: "Point", Coordinates: []float64{lng, lat}}
}

// OpeningHours reprend les horaires du flux, remis au propre.
type OpeningHours struct {
	Automate24h bool  `bson:"automate_24_24"`
	Days        []Day `bson:"days"`
}

type Day struct {
	Day    int    `bson:"day"` // 1 = lundi
	Name   string `bson:"name"`
	Closed bool   `bson:"closed"`
	Slots  []Slot `bson:"slots,omitempty"`
}

type Slot struct {
	Open  string `bson:"open"` // "07:00"
	Close string `bson:"close"`
}

// Station est un document de la collection stations.
// Le flux ne donne pas l'enseigne (Total, Leclerc...), on ne l'invente pas.
type Station struct {
	ID           string       `bson:"_id"`
	Address      string       `bson:"address"`
	City         string       `bson:"city"`
	PostalCode   string       `bson:"postal_code"`
	Department   string       `bson:"department"`
	Location     *GeoPoint    `bson:"location,omitempty"`
	RoadType     string       `bson:"road_type"`
	Services     []string     `bson:"services"`
	OpeningHours OpeningHours `bson:"opening_hours"`
	UpdatedAt    time.Time    `bson:"updated_at"`
}

type Repository struct {
	coll *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection(CollectionName)}
}

// UpsertMany crée ou remplace les stations en une seule requête groupée
// (bulk write), au lieu de 10 000 allers-retours vers la base.
func (r *Repository) UpsertMany(ctx context.Context, list []Station) (int64, error) {
	if len(list) == 0 {
		return 0, nil
	}
	models := make([]mongo.WriteModel, 0, len(list))
	for _, s := range list {
		models = append(models, mongo.NewReplaceOneModel().
			SetFilter(bson.D{{Key: "_id", Value: s.ID}}).
			SetReplacement(s).
			SetUpsert(true))
	}

	// ordered=false : une erreur sur une station n'arrête pas les autres
	res, err := r.coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return 0, fmt.Errorf("upsert des stations : %w", err)
	}
	return res.UpsertedCount + res.ModifiedCount, nil
}
