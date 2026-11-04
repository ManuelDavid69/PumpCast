// Package dataloader regroupe les requêtes pour éviter le problème N+1.
//
// Le problème : pour une liste de 20 stations, le resolver priceHistory est
// appelé 20 fois, une fois par station, ce qui ferait 20 requêtes MongoDB.
// Un dataloader attend quelques millisecondes que tous les resolvers aient
// demandé leur station, puis fait une seule requête avec les 20 identifiants
// ($in) et redistribue les résultats.
//
// Les loaders sont créés à chaque requête HTTP : leur cache ne dure que le
// temps d'une requête, il n'y a donc jamais de données périmées entre deux appels.
package dataloader

import (
	"context"
	"net/http"
	"time"

	"github.com/vikstrous/dataloadgen"

	"github.com/ManuelDavid69/pumpcast/api/internal/predictions"
	"github.com/ManuelDavid69/pumpcast/api/internal/prices"
)

// Temps d'attente pour regrouper les demandes avant de lancer la requête
const wait = 2 * time.Millisecond

// HistoryKey identifie un historique demandé. Les demandes avec le même
// carburant et la même date de début sont regroupées dans une requête.
type HistoryKey struct {
	StationID string
	Fuel      string
	Since     time.Time
}

type PredictionKey struct {
	StationID string
	Fuel      string
}

type Loaders struct {
	Current    *dataloadgen.Loader[string, []prices.Current]
	History    *dataloadgen.Loader[HistoryKey, []prices.DailyPrice]
	Prediction *dataloadgen.Loader[PredictionKey, *predictions.Prediction]
}

// Sources est ce dont les loaders ont besoin pour aller chercher les données.
type Sources struct {
	Current     *prices.CurrentCache
	History     *prices.HistoryRepository
	Predictions *predictions.Repository
}

func New(src Sources) *Loaders {
	return &Loaders{
		Current: dataloadgen.NewLoader(func(ctx context.Context, ids []string) ([][]prices.Current, []error) {
			found, err := src.Current.GetMany(ctx, ids)
			if err != nil {
				return nil, []error{err}
			}
			out := make([][]prices.Current, len(ids))
			for i, id := range ids {
				out[i] = found[id] // nil si la station n'est plus dans le flux
			}
			return out, nil
		}, dataloadgen.WithWait(wait)),

		History: dataloadgen.NewLoader(func(ctx context.Context, keys []HistoryKey) ([][]prices.DailyPrice, []error) {
			return fetchGrouped(keys, func(k HistoryKey) [2]any { return [2]any{k.Fuel, k.Since.Unix()} },
				func(ids []string, k HistoryKey) (map[string][]prices.DailyPrice, error) {
					return src.History.ForStations(ctx, ids, k.Fuel, k.Since)
				}, func(k HistoryKey) string { return k.StationID })
		}, dataloadgen.WithWait(wait)),

		Prediction: dataloadgen.NewLoader(func(ctx context.Context, keys []PredictionKey) ([]*predictions.Prediction, []error) {
			return fetchGrouped(keys, func(k PredictionKey) [2]any { return [2]any{k.Fuel, nil} },
				func(ids []string, k PredictionKey) (map[string]*predictions.Prediction, error) {
					return src.Predictions.LatestForStations(ctx, ids, k.Fuel)
				}, func(k PredictionKey) string { return k.StationID })
		}, dataloadgen.WithWait(wait)),
	}
}

// fetchGrouped fait une requête par groupe de clés (par exemple une par
// carburant) au lieu d'une par station, puis remet les résultats dans
// l'ordre des clés comme l'attend le dataloader.
func fetchGrouped[K comparable, V any](
	keys []K,
	group func(K) [2]any,
	fetch func(ids []string, sample K) (map[string]V, error),
	stationID func(K) string,
) ([]V, []error) {
	type batch struct {
		sample K
		ids    []string
	}
	groups := map[[2]any]*batch{}
	var order [][2]any
	for _, k := range keys {
		g := group(k)
		if groups[g] == nil {
			groups[g] = &batch{sample: k}
			order = append(order, g)
		}
		groups[g].ids = append(groups[g].ids, stationID(k))
	}

	results := make(map[[2]any]map[string]V, len(groups))
	for _, g := range order {
		found, err := fetch(groups[g].ids, groups[g].sample)
		if err != nil {
			return nil, []error{err}
		}
		results[g] = found
	}

	out := make([]V, len(keys))
	for i, k := range keys {
		out[i] = results[group(k)][stationID(k)]
	}
	return out, nil
}

type ctxKey struct{}

// Middleware crée des loaders neufs pour chaque requête HTTP.
func Middleware(src Sources, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), ctxKey{}, New(src))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// For récupère les loaders de la requête en cours.
func For(ctx context.Context) *Loaders {
	return ctx.Value(ctxKey{}).(*Loaders)
}
