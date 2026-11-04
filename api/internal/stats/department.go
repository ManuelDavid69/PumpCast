// Package stats calcule et stocke les moyennes de prix par département.
package stats

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const CollectionName = "department_daily"

// Meta identifie une série : un département et un carburant.
type Meta struct {
	Department string `bson:"department"`
	Fuel       string `bson:"fuel"`
}

// DepartmentDaily résume les prix d'un département pour un carburant et un jour.
// Stocké en time series comme les prix : ~900 000 lignes pour 4 ans de toute
// la France, qui se compressent très bien.
type DepartmentDaily struct {
	TS           time.Time `bson:"ts"`
	Meta         Meta      `bson:"meta"`
	AvgPrice     float64   `bson:"avg_price"`
	MinPrice     float64   `bson:"min_price"`
	MaxPrice     float64   `bson:"max_price"`
	StationCount int       `bson:"station_count"`
}

type aggKey struct {
	day  time.Time
	meta Meta
}

type agg struct {
	sum      float64
	min, max float64
	count    int
}

// Accumulator additionne les prix au fil de la lecture, sans garder chaque
// prix en mémoire : seulement une somme, un min, un max et un compteur par
// (jour, département, carburant).
type Accumulator struct {
	m map[aggKey]*agg
}

func NewAccumulator() *Accumulator {
	return &Accumulator{m: make(map[aggKey]*agg)}
}

// Add compte le prix d'une station pour ce jour.
func (a *Accumulator) Add(day time.Time, department, fuel string, price float64) {
	k := aggKey{day: day, meta: Meta{Department: department, Fuel: fuel}}
	g, ok := a.m[k]
	if !ok {
		a.m[k] = &agg{sum: price, min: price, max: price, count: 1}
		return
	}
	g.sum += price
	g.count++
	g.min = math.Min(g.min, price)
	g.max = math.Max(g.max, price)
}

// Results renvoie les moyennes jusqu'à lastDay inclus, triées par jour,
// département puis carburant.
func (a *Accumulator) Results(lastDay time.Time) []DepartmentDaily {
	out := make([]DepartmentDaily, 0, len(a.m))
	for k, g := range a.m {
		if k.day.After(lastDay) {
			continue
		}
		out = append(out, DepartmentDaily{
			TS:   k.day,
			Meta: k.meta,
			// Arrondi au dixième de centime, plus de précision n'a pas de sens
			AvgPrice:     math.Round(g.sum/float64(g.count)*10000) / 10000,
			MinPrice:     g.min,
			MaxPrice:     g.max,
			StationCount: g.count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].TS.Equal(out[j].TS) {
			return out[i].TS.Before(out[j].TS)
		}
		if out[i].Meta.Department != out[j].Meta.Department {
			return out[i].Meta.Department < out[j].Meta.Department
		}
		return out[i].Meta.Fuel < out[j].Meta.Fuel
	})
	return out
}

type Repository struct {
	coll *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection(CollectionName)}
}

// ReplaceDays remplace toutes les moyennes entre start et end inclus.
// Comme pour les prix, une time series ne permet pas l'upsert : on supprime
// la période puis on réinsère. Les moyennes sont recalculées en entier à
// chaque fois, donc il n'y a jamais besoin de fusionner avec l'existant.
func (r *Repository) ReplaceDays(ctx context.Context, start, end time.Time, list []DepartmentDaily) error {
	if _, err := r.coll.DeleteMany(ctx, bson.D{
		{Key: "ts", Value: bson.D{{Key: "$gte", Value: start}, {Key: "$lte", Value: end}}},
	}); err != nil {
		return fmt.Errorf("suppression des moyennes : %w", err)
	}
	if len(list) == 0 {
		return nil
	}
	if _, err := r.coll.InsertMany(ctx, list, options.InsertMany().SetOrdered(false)); err != nil {
		return fmt.Errorf("insertion des moyennes : %w", err)
	}
	return nil
}

// Latest renvoie le dernier jour connu pour un département et un carburant.
// nil si le département n'existe pas.
func (r *Repository) Latest(ctx context.Context, department, fuel string) (*DepartmentDaily, error) {
	var doc DepartmentDaily
	err := r.coll.FindOne(ctx,
		bson.D{{Key: "meta.department", Value: department}, {Key: "meta.fuel", Value: fuel}},
		options.FindOne().SetSort(bson.D{{Key: "ts", Value: -1}}),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("moyenne du département : %w", err)
	}
	return &doc, nil
}

// Range renvoie les moyennes d'un département depuis from, triées par date.
func (r *Repository) Range(ctx context.Context, department, fuel string, from time.Time) ([]DepartmentDaily, error) {
	cur, err := r.coll.Find(ctx, bson.D{
		{Key: "meta.department", Value: department},
		{Key: "meta.fuel", Value: fuel},
		{Key: "ts", Value: bson.D{{Key: "$gte", Value: from}}},
	}, options.Find().SetSort(bson.D{{Key: "ts", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("évolution du département : %w", err)
	}
	var list []DepartmentDaily
	if err := cur.All(ctx, &list); err != nil {
		return nil, fmt.Errorf("évolution du département : %w", err)
	}
	return list, nil
}

// LatestAll renvoie la moyenne de chaque département pour le dernier jour connu.
func (r *Repository) LatestAll(ctx context.Context, fuel string) ([]DepartmentDaily, error) {
	var last DepartmentDaily
	err := r.coll.FindOne(ctx, bson.D{{Key: "meta.fuel", Value: fuel}},
		options.FindOne().SetSort(bson.D{{Key: "ts", Value: -1}})).Decode(&last)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return []DepartmentDaily{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("moyennes des départements : %w", err)
	}
	cur, err := r.coll.Find(ctx, bson.D{{Key: "meta.fuel", Value: fuel}, {Key: "ts", Value: last.TS}},
		options.Find().SetSort(bson.D{{Key: "meta.department", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("moyennes des départements : %w", err)
	}
	var list []DepartmentDaily
	if err := cur.All(ctx, &list); err != nil {
		return nil, fmt.Errorf("moyennes des départements : %w", err)
	}
	return list, nil
}
