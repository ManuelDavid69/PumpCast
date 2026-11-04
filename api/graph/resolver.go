// Package graph contient les resolvers GraphQL : les fonctions qui vont
// chercher les données pour chaque champ du schéma.
package graph

import (
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ManuelDavid69/pumpcast/api/graph/model"
	"github.com/ManuelDavid69/pumpcast/api/internal/ingest"
	"github.com/ManuelDavid69/pumpcast/api/internal/predictions"
	"github.com/ManuelDavid69/pumpcast/api/internal/prices"
	"github.com/ManuelDavid69/pumpcast/api/internal/stations"
	"github.com/ManuelDavid69/pumpcast/api/internal/stats"
)

// Limites des paramètres, pour qu'une seule requête ne puisse pas faire
// tomber la base (par exemple 1000 stations avec 10 ans d'historique)
const (
	maxRadiusKm     = 50.0
	maxLimit        = 100
	maxHistoryDays  = 365
	maxCompared     = 5
	nearbyOversize  = 5   // on demande plus de stations à Mongo pour filtrer ensuite
	maxNearbyFetch  = 500 // sans jamais dépasser ce nombre
	performanceRuns = 20
)

// Resolver regroupe les dépendances partagées par tous les resolvers
// (l'équivalent des services injectés dans un contrôleur Spring).
// Les champs par station (prix actuels, historique, prédiction) passent
// par les dataloaders de la requête, voir internal/dataloader.
type Resolver struct {
	Stations    *stations.Repository
	Current     *prices.CurrentCache
	Stats       *stats.Repository
	Predictions *predictions.Repository
	Redis       *redis.Client
	Now         func() time.Time
}

func (r *Resolver) today() time.Time {
	return prices.Day(r.Now(), ingest.Paris)
}

func formatDay(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func toPricePoints(list []prices.DailyPrice) []*model.PricePoint {
	out := make([]*model.PricePoint, len(list))
	for i, p := range list {
		out[i] = &model.PricePoint{Date: formatDay(p.TS), Price: p.Price}
	}
	return out
}

func toCurrentPrices(list []prices.Current) []*model.CurrentPrice {
	out := make([]*model.CurrentPrice, 0, len(list))
	for _, c := range list {
		cp := &model.CurrentPrice{Fuel: model.Fuel(c.Fuel), OutOfStock: c.OutOfStock}
		// Une rupture signalée sans prix connu : prix et date restent nuls
		if c.Price > 0 {
			price := c.Price
			updated := c.UpdatedAt.In(ingest.Paris).Format(time.RFC3339)
			cp.Price, cp.UpdatedAt = &price, &updated
		}
		out = append(out, cp)
	}
	return out
}

// priceOf renvoie le prix payable d'un carburant (pas en rupture), ou false.
func priceOf(list []prices.Current, fuel string) (float64, bool) {
	for _, c := range list {
		if c.Fuel == fuel && !c.OutOfStock && c.Price > 0 {
			return c.Price, true
		}
	}
	return 0, false
}

func hasFuel(list []prices.Current, fuel string) bool {
	for _, c := range list {
		if c.Fuel == fuel {
			return true
		}
	}
	return false
}

var recommendations = map[string]model.Recommendation{
	"fill_now": model.RecommendationFillNow,
	"wait":     model.RecommendationWait,
	"stable":   model.RecommendationStable,
}

func toPrediction(p *predictions.Prediction) (*model.Prediction, error) {
	rec, ok := recommendations[p.Recommendation]
	if !ok {
		return nil, fmt.Errorf("recommandation inconnue en base : %q", p.Recommendation)
	}
	out := &model.Prediction{
		Recommendation: rec,
		Confidence:     p.Confidence,
		Reasons:        p.Reasons,
		ModelVersion:   p.ModelVersion,
		GeneratedAt:    p.GeneratedAt.In(ingest.Paris).Format(time.RFC3339),
		Horizon:        make([]*model.PredictedPoint, len(p.Horizon)),
	}
	if out.Reasons == nil {
		out.Reasons = []string{}
	}
	for i, h := range p.Horizon {
		out.Horizon[i] = &model.PredictedPoint{Date: formatDay(h.Date), Price: h.Price, Low: h.Low, High: h.High}
	}
	return out, nil
}

// normalizeDepartment accepte "69", "2a" ou "974" et refuse le reste,
// pour ne pas envoyer n'importe quoi dans une clé de cache.
func normalizeDepartment(raw string) (string, error) {
	d := strings.ToUpper(strings.TrimSpace(raw))
	if len(d) < 2 || len(d) > 3 {
		return "", fmt.Errorf("département invalide : %q", raw)
	}
	for _, c := range d {
		if (c < '0' || c > '9') && c != 'A' && c != 'B' {
			return "", fmt.Errorf("département invalide : %q", raw)
		}
	}
	return d, nil
}

func toAverages(list []stats.DepartmentDaily) []*model.DepartmentAverage {
	out := make([]*model.DepartmentAverage, len(list))
	for i, d := range list {
		out[i] = &model.DepartmentAverage{Department: d.Meta.Department, AvgPrice: d.AvgPrice, StationCount: d.StationCount}
	}
	return out
}
