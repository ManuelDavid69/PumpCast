package archive

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ManuelDavid69/pumpcast/api/internal/ingest"
	"github.com/ManuelDavid69/pumpcast/api/internal/prices"
	"github.com/ManuelDavid69/pumpcast/api/internal/stations"
)

// Nombre de relevés envoyés à MongoDB par requête
const insertBatch = 20000

var fuels = []string{"GAZOLE", "SP95", "SP98", "E10", "E85", "GPLC"}

// Carry garde le dernier prix de chaque série entre deux années.
type Carry map[prices.Meta]*LastPrice

type Importer struct {
	History     *prices.HistoryRepository
	Stations    *stations.Repository
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
}

func (s YearSummary) String() string {
	return fmt.Sprintf("%d : %d stations dans le fichier, %d dans le périmètre (%d nouvelles en base), du %s au %s : %d relevés supprimés, %d insérés",
		s.Year, s.StationsInFile, s.StationsInScope, s.NewStations,
		s.Start.Format("2006-01-02"), s.End.Format("2006-01-02"), s.Deleted, s.Inserted)
}

// ImportYear importe une archive annuelle. carry est lu puis mis à jour
// avec les derniers prix de l'année, pour l'année suivante.
// lastDay est le dernier jour importable (en général hier : aujourd'hui
// est géré par l'ingestion du flux instantané).
func (im *Importer) ImportYear(ctx context.Context, r io.Reader, year int, carry Carry, lastDay time.Time) (YearSummary, error) {
	sum := YearSummary{Year: year}

	// 1. Lecture du fichier. On ne garde en mémoire que les stations du
	// périmètre, mais on regarde toutes les dates pour savoir jusqu'où
	// va le fichier (l'archive de l'année en cours s'arrête à hier ou avant-hier).
	var scope []ingest.Station
	var fileEnd time.Time
	_, err := ingest.Parse(r, func(st ingest.Station) error {
		sum.StationsInFile++
		for _, p := range st.Prices {
			if d := prices.Day(p.UpdatedAt, ingest.Paris); d.After(fileEnd) {
				fileEnd = d
			}
		}
		if im.KeepHistory(st.Department) {
			scope = append(scope, st)
		}
		return nil
	})
	if err != nil {
		return sum, fmt.Errorf("lecture de l'archive %d : %w", year, err)
	}
	sum.StationsInScope = len(scope)

	// 2. Période couverte : du 1er janvier à la fin du fichier,
	// sans dépasser le 31 décembre ni lastDay
	sum.Start = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	sum.End = minDate(time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC), fileEnd, lastDay)
	if sum.End.Before(sum.Start) {
		return sum, fmt.Errorf("archive %d vide ou hors période", year)
	}

	// 3. Les stations fermées depuis ne sont plus dans le flux du jour,
	// mais il faut leurs infos (département, type de route) pour le modèle
	docs := make([]stations.Station, 0, len(scope))
	ids := make([]string, 0, len(scope))
	for _, st := range scope {
		docs = append(docs, ingest.StationDoc(st, sum.End))
		ids = append(ids, st.ID)
	}
	if sum.NewStations, err = im.Stations.InsertMissing(ctx, docs); err != nil {
		return sum, err
	}

	// 4. On vide la période avant de réinsérer : relancer l'import ne crée pas de doublons
	if sum.Deleted, err = im.History.DeleteRange(ctx, ids, sum.Start, sum.End); err != nil {
		return sum, err
	}

	// 5. Reconstruction jour par jour et insertion par paquets
	batch := make([]prices.DailyPrice, 0, insertBatch)
	flush := func() error {
		if err := im.History.InsertMany(ctx, batch); err != nil {
			return err
		}
		sum.Inserted += len(batch)
		batch = batch[:0]
		return nil
	}

	for _, st := range scope {
		events := map[string][]ingest.Price{}
		for _, p := range st.Prices {
			events[p.Fuel] = append(events[p.Fuel], p)
		}
		ruptures := map[string][]ingest.Rupture{}
		for _, rp := range st.Ruptures {
			ruptures[rp.Fuel] = append(ruptures[rp.Fuel], rp)
		}

		for _, fuel := range fuels {
			meta := prices.Meta{StationID: st.ID, Fuel: fuel}
			if len(events[fuel]) == 0 && carry[meta] == nil {
				continue
			}
			points, last := DailySeries(events[fuel], carry[meta], ruptures[fuel], sum.Start, sum.End)
			carry[meta] = last
			for _, pt := range points {
				batch = append(batch, prices.DailyPrice{TS: pt.Day, Meta: meta, Price: pt.Price})
				if len(batch) == insertBatch {
					if err := flush(); err != nil {
						return sum, err
					}
				}
			}
		}
	}
	if err := flush(); err != nil {
		return sum, err
	}
	return sum, nil
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
