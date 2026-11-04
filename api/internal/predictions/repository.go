// Package predictions lit les prédictions et les scores du modèle.
//
// Ces collections sont écrites par les scripts Python (ml/pumpcast_ml/predict.py
// et train.py), l'API ne fait que les lire. Les structs ci-dessous suivent le
// format de ces scripts.
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
	StationID    string    `bson:"station_id"`
	Fuel         string    `bson:"fuel"`
	GeneratedAt  time.Time `bson:"generated_at"`
	BaseDate     time.Time `bson:"base_date"`
	BasePrice    float64   `bson:"base_price"`
	ModelVersion string    `bson:"model_version"`
	Horizon      []Point   `bson:"horizon"`
	// Jour et prix visés par le conseil, nil s'il n'y a pas de conseil
	AdviceTarget *Point `bson:"advice_target"`
	// fill_now, wait, stable, ou nil quand le modèle n'est pas assez fiable
	Recommendation *string  `bson:"recommendation"`
	Confidence     float64  `bson:"confidence"`
	Reasons        []string `bson:"reasons"`
}

type PeriodScore struct {
	MAEBaseline float64 `bson:"mae_baseline"`
	MAEModel    float64 `bson:"mae_model"`
}

// HorizonScore est le score du modèle pour un horizon (J+h).
type HorizonScore struct {
	Horizon          int                    `bson:"horizon"`
	MAEBaseline      float64                `bson:"mae_baseline"`
	MAERidge         float64                `bson:"mae_ridge"`
	MAEModel         float64                `bson:"mae_model"`
	IntervalCoverage float64                `bson:"interval_coverage"`
	BeatsBaseline    bool                   `bson:"beats_baseline"`
	ByPeriod         map[string]PeriodScore `bson:"by_period"`
}

type RecommendationStats struct {
	Accuracy       *float64            `bson:"accuracy"`
	Counts         map[string]int      `bson:"counts"`
	AccuracyByKind map[string]*float64 `bson:"accuracy_by_kind"`
	AvgGainCents   map[string]*float64 `bson:"avg_gain_cents"`
}

type FeatureImportance struct {
	Feature     string  `bson:"feature"`
	MAEIncrease float64 `bson:"mae_increase"`
}

type Period struct {
	Start time.Time `bson:"start"`
	End   time.Time `bson:"end"`
}

// KindDecision dit si un type de conseil passe la validation, le test, et s'il est activé.
type KindDecision struct {
	Validation bool `bson:"validation"`
	Test       bool `bson:"test"`
	Enabled    bool `bson:"enabled"`
}

type Criteria struct {
	MinCountValidation    int     `bson:"min_count_validation"`
	MinAccuracyValidation float64 `bson:"min_accuracy_validation"`
	MinCountTest          int     `bson:"min_count_test"`
	MinAccuracyTest       float64 `bson:"min_accuracy_test"`
}

// RecommendationPolicy est la règle d'activation des conseils, décidée à l'entraînement.
type RecommendationPolicy struct {
	EnabledKinds []string                `bson:"enabled_kinds"`
	Decision     map[string]KindDecision `bson:"decision"`
	Criteria     Criteria                `bson:"criteria"`
}

// ModelRun est un entraînement du modèle et ses scores sur la période de test.
type ModelRun struct {
	ModelVersion    string               `bson:"model_version"`
	TrainedAt       time.Time            `bson:"trained_at"`
	TrainPeriod     Period               `bson:"train_period"`
	TestPeriod      Period               `bson:"test_period"`
	AllowedHorizons []int                `bson:"allowed_horizons"`
	Policy          RecommendationPolicy `bson:"recommendation_policy"`
	Metrics         struct {
		Horizons                  []HorizonScore      `bson:"horizons"`
		Recommendations           RecommendationStats `bson:"recommendations"`
		RecommendationsValidation RecommendationStats `bson:"recommendations_validation"`
		RecommendationsTestAll    RecommendationStats `bson:"recommendations_test_all_kinds"`
		FeatureImportance         []FeatureImportance `bson:"feature_importance"`
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
