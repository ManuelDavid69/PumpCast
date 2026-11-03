// Package archive importe les archives annuelles des prix.
//
// Les archives ne donnent pas un prix par jour mais la liste des
// changements de prix. On reconstruit donc un prix par jour, comme le
// ferait l'ingestion quotidienne : chaque jour, le dernier prix connu.
package archive

import (
	"sort"
	"time"

	"github.com/ManuelDavid69/pumpcast/api/internal/ingest"
	"github.com/ManuelDavid69/pumpcast/api/internal/prices"
)

const day = 24 * time.Hour

// LastPrice est le dernier prix connu d'une série, reporté d'une année
// sur l'autre pour ne pas avoir de trou au 1er janvier.
type LastPrice struct {
	Value float64
	Day   time.Time
}

// Point est le prix d'un jour.
type Point struct {
	Day   time.Time
	Price float64
}

// DailySeries reconstruit les prix jour par jour entre start et end (inclus)
// pour un carburant d'une station.
//
//   - events : les changements de prix de l'année, dans n'importe quel ordre
//   - carry : le dernier prix de l'année précédente (nil si inconnu)
//   - ruptures : les ruptures de ce carburant, les jours concernés sont sautés
//
// Un prix n'est plus reporté après ingest.MaxPriceAge sans mise à jour,
// même règle que pour l'ingestion quotidienne.
// Renvoie aussi le dernier prix connu, à passer à l'année suivante.
func DailySeries(events []ingest.Price, carry *LastPrice, ruptures []ingest.Rupture, start, end time.Time) ([]Point, *LastPrice) {
	sorted := make([]ingest.Price, len(events))
	copy(sorted, events)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].UpdatedAt.Before(sorted[j].UpdatedAt) })

	// Les ruptures sont ramenées à des intervalles de jours.
	// Une rupture encore en cours court jusqu'à la fin de la période.
	type interval struct{ from, to time.Time }
	var blocked []interval
	for _, r := range ruptures {
		to := end
		if !r.Ongoing() {
			to = prices.Day(r.End, ingest.Paris)
		}
		blocked = append(blocked, interval{from: prices.Day(r.Start, ingest.Paris), to: to})
	}
	isBlocked := func(d time.Time) bool {
		for _, b := range blocked {
			if !d.Before(b.from) && !d.After(b.to) {
				return true
			}
		}
		return false
	}

	last := carry
	next := 0
	var points []Point
	for d := start; !d.After(end); d = d.Add(day) {
		// On avance jusqu'au dernier changement de prix de la journée
		for next < len(sorted) && !prices.Day(sorted[next].UpdatedAt, ingest.Paris).After(d) {
			e := sorted[next]
			last = &LastPrice{Value: e.Value, Day: prices.Day(e.UpdatedAt, ingest.Paris)}
			next++
		}
		if last == nil || d.Sub(last.Day) > ingest.MaxPriceAge || isBlocked(d) {
			continue
		}
		points = append(points, Point{Day: d, Price: last.Value})
	}

	// S'il reste des changements après end (ne devrait pas arriver), on les
	// prend quand même en compte pour le report
	for ; next < len(sorted); next++ {
		e := sorted[next]
		last = &LastPrice{Value: e.Value, Day: prices.Day(e.UpdatedAt, ingest.Paris)}
	}
	return points, last
}
