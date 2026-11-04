package archive

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ManuelDavid69/pumpcast/api/internal/ingest"
	"github.com/ManuelDavid69/pumpcast/api/internal/prices"
	"github.com/ManuelDavid69/pumpcast/api/internal/stations"
	"github.com/ManuelDavid69/pumpcast/api/internal/stats"
)

// Nombre de relevés envoyés à MongoDB par requête
const insertBatch = 20000

var fuels = []string{"GAZOLE", "SP95", "SP98", "E10", "E85", "GPLC"}

// Carry garde le dernier prix de chaque série entre deux années.
type Carry map[prices.Meta]*LastPrice

type Importer struct {
	History     *prices.HistoryRepository
	Stations    *stations.Repository
	Stats       *stats.Repository
	KeepHistory func(department string) bool
}

// YearSummary résume l'import d'une année.
type YearSummary struct {
	Year            int
	Start, End      time.Time
	StationsInFile  int
	StationsInScope int
	NewStations     int64
	Deleted         int64
	Inserted        int
	DeptStats       int
}

func (s YearSummary) String() string {
	return fmt.Sprintf("%d : %d stations dans le fichier, %d dans le périmètre (%d nouvelles en base), du %s au %s : %d relevés supprimés, %d insérés, %d moyennes département",
		s.Year, s.StationsInFile, s.StationsInScope, s.NewStations,
		s.Start.Format("2006-01-02"), s.End.Format("2006-01-02"), s.Deleted, s.Inserted, s.DeptStats)
}

// ImportYear importe une archive annuelle. carry est lu puis mis à jour
// avec les derniers prix de l'année, pour l'année suivante.
// lastDay est le dernier jour importable (en général hier : aujourd'hui
// est géré par l'ingestion du flux instantané).
func (im *Importer) ImportYear(ctx context.Context, r io.Reader, year int, carry Carry, lastDay time.Time) (YearSummary, error) {
	sum := YearSummary{Year: year}
	sum.Start = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	// Fin provisoire. La vraie fin n'est connue qu'à la fin de la lecture :
	// l'archive de l'année en cours s'arrête souvent un ou deux jours avant hier.
	maxEnd := minDate(time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC), lastDay)

	// 1. Lecture station par station. Chaque station est transformée tout de
	// suite en série jour par jour : la série sert aux moyennes de toute la
	// France, et on ne garde les points que pour les stations du périmètre.
	acc := stats.NewAccumulator()
	var fileEnd time.Time
	var docs []stations.Station
	var ids []string
	var history []prices.DailyPrice

	_, err := ingest.Parse(r, func(st ingest.Station) error {
		sum.StationsInFile++
		for _, p := range st.Prices {
			if d := prices.Day(p.UpdatedAt, ingest.Paris); d.After(fileEnd) {
				fileEnd = d
			}
		}
		inScope := im.KeepHistory(st.Department)
		if inScope {
			docs = append(docs, ingest.StationDoc(st, maxEnd))
			ids = append(ids, st.ID)
		}

		for _, fuel := range fuels {
			meta := prices.Meta{StationID: st.ID, Fuel: fuel}
			events := pricesOf(st.Prices, fuel)
			if len(events) == 0 && carry[meta] == nil {
				continue
			}
			points, last := DailySeries(events, carry[meta], rupturesOf(st.Ruptures, fuel), sum.Start, maxEnd)
			carry[meta] = last
			for _, pt := range points {
				acc.Add(pt.Day, st.Department, fuel, pt.Price)
				if inScope {
					history = append(history, prices.DailyPrice{TS: pt.Day, Meta: meta, Price: pt.Price})
				}
			}
		}
		return nil
	})
	if err != nil {
		return sum, fmt.Errorf("lecture de l'archive %d : %w", year, err)
	}
	sum.StationsInScope = len(ids)

	// 2. Période réellement couverte par le fichier
	sum.End = minDate(maxEnd, fileEnd)
	if sum.End.Before(sum.Start) {
		return sum, fmt.Errorf("archive %d vide ou hors période", year)
	}

	// 3. Les stations fermées depuis ne sont plus dans le flux du jour,
	// mais il faut leurs infos (département, type de route) pour le modèle
	if sum.NewStations, err = im.Stations.InsertMissing(ctx, docs); err != nil {
		return sum, err
	}

	// 4. Moyennes par département pour toute la France
	deptStats := acc.Results(sum.End)
	if err := im.Stats.ReplaceDays(ctx, sum.Start, sum.End, deptStats); err != nil {
		return sum, err
	}
	sum.DeptStats = len(deptStats)

	// 5. Historique du périmètre : on vide la période avant de réinsérer,
	// relancer l'import ne crée pas de doublons
	if sum.Deleted, err = im.History.DeleteRange(ctx, ids, sum.Start, sum.End); err != nil {
		return sum, err
	}
	batch := make([]prices.DailyPrice, 0, insertBatch)
	for _, p := range history {
		if p.TS.After(sum.End) {
			continue
		}
		batch = append(batch, p)
		if len(batch) == insertBatch {
			if err := im.History.InsertMany(ctx, batch); err != nil {
				return sum, err
			}
			sum.Inserted += len(batch)
			batch = batch[:0]
		}
	}
	if err := im.History.InsertMany(ctx, batch); err != nil {
		return sum, err
	}
	sum.Inserted += len(batch)
	return sum, nil
}

func pricesOf(list []ingest.Price, fuel string) []ingest.Price {
	var out []ingest.Price
	for _, p := range list {
		if p.Fuel == fuel {
			out = append(out, p)
		}
	}
	return out
}

func rupturesOf(list []ingest.Rupture, fuel string) []ingest.Rupture {
	var out []ingest.Rupture
	for _, r := range list {
		if r.Fuel == fuel {
			out = append(out, r)
		}
	}
	return out
}

func minDate(first time.Time, others ...time.Time) time.Time {
	m := first
	for _, t := range others {
		if t.Before(m) {
			m = t
		}
	}
	return m
}
