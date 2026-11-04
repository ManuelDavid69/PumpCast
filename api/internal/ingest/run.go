package ingest

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ManuelDavid69/pumpcast/api/internal/prices"
	"github.com/ManuelDavid69/pumpcast/api/internal/stations"
	"github.com/ManuelDavid69/pumpcast/api/internal/stats"
)

// Au delà de 30 jours sans mise à jour, un prix est sûrement abandonné
// (carburant plus vendu mais jamais retiré du flux). On ne le garde pas.
const MaxPriceAge = 30 * 24 * time.Hour

// Deps regroupe ce dont l'ingestion a besoin pour écrire.
type Deps struct {
	Stations    *stations.Repository
	History     *prices.HistoryRepository
	Current     *prices.CurrentCache
	Stats       *stats.Repository
	KeepHistory func(department string) bool
	Now         func() time.Time
}

// Summary résume une ingestion, affiché dans les logs.
type Summary struct {
	Stations       int
	Skipped        int
	StationsWrites int64
	CurrentPrices  int
	History        prices.SaveResult
	DeptStats      int
	Day            time.Time
}

func (s Summary) String() string {
	return fmt.Sprintf("%d stations lues (%d ignorées), %d écritures stations, %d prix actuels en cache, historique du %s : %d ajoutés, %d modifiés, %d inchangés, %d moyennes département",
		s.Stations, s.Skipped, s.StationsWrites, s.CurrentPrices, s.Day.Format("2006-01-02"),
		s.History.Inserted, s.History.Updated, s.History.Unchanged, s.DeptStats)
}

// Batch est le résultat de la transformation du flux, avant écriture.
// Séparer cette étape permet de la tester sans base de données.
type Batch struct {
	Stations []stations.Station
	Current  map[string][]prices.Current
	History  []prices.DailyPrice
	// Moyennes du jour par département, pour toute la France
	DeptStats []stats.DepartmentDaily
}

// StationDoc convertit une station du flux en document MongoDB.
func StationDoc(st Station, updatedAt time.Time) stations.Station {
	doc := stations.Station{
		ID:           st.ID,
		Address:      st.Address,
		City:         st.City,
		PostalCode:   st.PostalCode,
		Department:   st.Department,
		RoadType:     st.RoadType,
		Services:     st.Services,
		OpeningHours: st.Hours,
		UpdatedAt:    updatedAt,
	}
	if st.HasCoords {
		doc.Location = stations.NewPoint(st.Latitude, st.Longitude)
	}
	return doc
}

// BuildBatch transforme les stations du flux en documents à écrire.
func BuildBatch(list []Station, now time.Time, keepHistory func(string) bool) Batch {
	b := Batch{Current: make(map[string][]prices.Current, len(list))}
	day := prices.Day(now, Paris)
	acc := stats.NewAccumulator()

	for _, st := range list {
		b.Stations = append(b.Stations, StationDoc(st, now))

		// Carburants en rupture temporaire en ce moment.
		// Une rupture "definitive" veut dire que la station ne vend plus ce carburant.
		outOfStock := map[string]bool{}
		dropped := map[string]bool{}
		for _, r := range st.Ruptures {
			if !r.Ongoing() {
				continue
			}
			if r.Type == "definitive" {
				dropped[r.Fuel] = true
			} else {
				outOfStock[r.Fuel] = true
			}
		}

		var current []prices.Current
		seen := map[string]bool{}
		for _, p := range st.Prices {
			if dropped[p.Fuel] || now.Sub(p.UpdatedAt) > MaxPriceAge {
				continue
			}
			seen[p.Fuel] = true
			current = append(current, prices.Current{
				Fuel: p.Fuel, Price: p.Value, UpdatedAt: p.UpdatedAt, OutOfStock: outOfStock[p.Fuel],
			})
			// Pendant une rupture le prix affiché n'est pas payable, on ne le met
			// pas dans l'historique pour ne pas fausser le modèle
			if outOfStock[p.Fuel] {
				continue
			}
			// Mêmes prix que l'historique, mais pour tous les départements
			acc.Add(day, st.Department, p.Fuel, p.Value)
			if keepHistory(st.Department) {
				b.History = append(b.History, prices.DailyPrice{
					Meta:  prices.Meta{StationID: st.ID, Fuel: p.Fuel},
					Price: p.Value,
				})
			}
		}
		// Rupture sans prix connu : on la signale quand même à l'utilisateur
		for fuel := range outOfStock {
			if !seen[fuel] && !dropped[fuel] {
				current = append(current, prices.Current{Fuel: fuel, OutOfStock: true})
			}
		}

		if len(current) > 0 {
			b.Current[st.ID] = current
		}
	}
	b.DeptStats = acc.Results(day)
	return b
}

// Run lit le XML du flux et écrit tout : stations, prix actuels, historique.
func Run(ctx context.Context, xmlData []byte, d Deps) (Summary, error) {
	now := d.Now()
	var list []Station
	skipped, err := Parse(bytes.NewReader(xmlData), func(st Station) error {
		list = append(list, st)
		return nil
	})
	if err != nil {
		return Summary{}, err
	}

	b := BuildBatch(list, now, d.KeepHistory)
	sum := Summary{Stations: len(list), Skipped: skipped, CurrentPrices: len(b.Current), Day: prices.Day(now, Paris)}

	if sum.StationsWrites, err = d.Stations.UpsertMany(ctx, b.Stations); err != nil {
		return sum, err
	}
	if sum.History, err = d.History.SaveDay(ctx, sum.Day, b.History); err != nil {
		return sum, err
	}
	if err := d.Stats.ReplaceDays(ctx, sum.Day, sum.Day, b.DeptStats); err != nil {
		return sum, err
	}
	sum.DeptStats = len(b.DeptStats)

	// Le cache est mis à jour en dernier : si mongo a planté on garde
	// les anciens prix plutôt qu'un cache incohérent avec la base
	if err := d.Current.SaveAll(ctx, b.Current); err != nil {
		return sum, err
	}
	return sum, nil
}
