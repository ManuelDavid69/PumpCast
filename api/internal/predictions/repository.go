// Package predictions lit les prédictions et les scores du modèle.
//
// Ces collections sont écrites par les scripts Python (phase 5), l'API ne
// fait que les lire. Les structs ci-dessous fixent donc le format attendu.
package predictions

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Point struct {
	Date  time.Time `bson:"date"`
	Price float64   `bson:"price"`
	Low   float64   `bson:"low"`
	High  float64   `bson:"high"`
}

// Prediction est la prédiction d'un carburant pour une station (J+1 à J+7).
type Prediction struct {
	StationID      string    `bson:"station_id"`
	Fuel           string    `bson:"fuel"`
	GeneratedAt    time.Time `bson:"generated_at"`
	ModelVersion   string    `bson:"model_version"`
	Horizon        []Point   `bson:"horizon"`
	Recommendation string    `bson:"recommendation"` // fill_now, wait ou stable
	Confidence     float64   `bson:"confidence"`
	Reasons        []string  `bson:"reasons"`
}

// HorizonScore est le score du modèle pour un horizon (J+h).
type HorizonScore struct {
	Horizon                int      `bson:"horizon"`
	MAEModel               float64  `bson:"mae_model"`
	MAEBaseline            float64  `bson:"mae_baseline"`
	RecommendationAccuracy *float64 `bson:"recommendation_accuracy"`
}

// ModelRun est un entraînement du modèle et ses scores sur la période de test.
type ModelRun struct {
	ModelVersion string    `bson:"model_version"`
	TrainedAt    time.Time `bson:"trained_at"`
	Metrics      struct {
		Horizons []HorizonScore `bson:"horizons"`
	} `bson:"metrics"`
}

type Repository struct {
	predictions *mongo.Collection
	runs        *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{predictions: db.Collection("predictions"), runs: db.Collection("model_runs")}
}

// LatestForStations renvoie la prédiction la plus récente de chaque station
// pour un carburant, en une seule requête (utilisé par le dataloader).
func (r *Repository) LatestForStations(ctx context.Context, ids []string, fuel string) (map[string]*Prediction, error) {
	out := make(map[string]*Prediction, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	// On trie par date de génération puis on garde la première de chaque station
	pipeline := bson.A{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "station_id", Value: bson.D{{Key: "$in", Value: ids}}},
			{Key: "fuel", Value: fuel},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "generated_at", Value: -1}}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$station_id"},
			{Key: "doc", Value: bson.D{{Key: "$first", Value: "$$ROOT"}}},
		}}},
		bson.D{{Key: "$replaceRoot", Value: bson.D{{Key: "newRoot", Value: "$doc"}}}},
	}
	cur, err := r.predictions.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("lecture des prédictions : %w", err)
	}
	var list []Prediction
	if err := cur.All(ctx, &list); err != nil {
		return nil, fmt.Errorf("lecture des prédictions : %w", err)
	}
	for i := range list {
		out[list[i].StationID] = &list[i]
	}
	return out, nil
}

// Runs renvoie les entraînements du plus récent au plus ancien.
func (r *Repository) Runs(ctx context.Context, limit int) ([]ModelRun, error) {
	cur, err := r.runs.Find(ctx, bson.D{},
		options.Find().SetSort(bson.D{{Key: "trained_at", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, fmt.Errorf("lecture des scores du modèle : %w", err)
	}
	var list []ModelRun
	if err := cur.All(ctx, &list); err != nil {
		return nil, fmt.Errorf("lecture des scores du modèle : %w", err)
	}
	return list, nil
}
