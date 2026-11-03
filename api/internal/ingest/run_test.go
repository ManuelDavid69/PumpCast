package ingest

import (
	"math"
	"testing"
	"time"

	"github.com/ManuelDavid69/pumpcast/api/internal/prices"
)

// findCurrent retrouve un prix actuel par carburant.
func findCurrent(list []prices.Current, fuel string) (prices.Current, bool) {
	for _, c := range list {
		if c.Fuel == fuel {
			return c, true
		}
	}
	return prices.Current{}, false
}

func TestBuildBatch(t *testing.T) {
	list, skipped := parseAll(t)
	if skipped != 1 {
		t.Fatalf("skipped = %d, attendu 1", skipped)
	}

	now := time.Date(2026, 11, 3, 12, 0, 0, 0, paris)
	b := BuildBatch(list, now, func(d string) bool { return d == "69" })

	if len(b.Stations) != 3 {
		t.Fatalf("len(Stations) = %d, attendu 3", len(b.Stations))
	}

	// Coordonnées : longitude en premier, station sans coordonnées sans Location.
	var s691, s971 bool
	for _, s := range b.Stations {
		switch s.ID {
		case "69100001":
			s691 = true
			if s.Location == nil {
				t.Fatal("Location de 69100001 nulle")
			}
			wantLng, wantLat := 4.8835, 45.7712
			if len(s.Location.Coordinates) != 2 ||
				math.Abs(s.Location.Coordinates[0]-wantLng) > 1e-9 ||
				math.Abs(s.Location.Coordinates[1]-wantLat) > 1e-9 {
				t.Errorf("Coordinates = %v, attendu [%v %v]", s.Location.Coordinates, wantLng, wantLat)
			}
		case "97100001":
			s971 = true
			if s.Location != nil {
				t.Errorf("Location de 97100001 = %v, attendu nil", s.Location)
			}
		}
	}
	if !s691 || !s971 {
		t.Fatalf("stations attendues absentes (69100001=%v, 97100001=%v)", s691, s971)
	}

	// Prix actuels de la station 69100001.
	cur := b.Current["69100001"]
	if len(cur) != 3 {
		t.Fatalf("len(Current[69100001]) = %d, attendu 3", len(cur))
	}
	if g, ok := findCurrent(cur, "GAZOLE"); !ok || g.Price != 1.629 || g.OutOfStock {
		t.Errorf("GAZOLE = %+v (ok %v), attendu 1.629 en stock", g, ok)
	}
	if s, ok := findCurrent(cur, "SP98"); !ok || s.Price != 1.799 || s.OutOfStock {
		t.Errorf("SP98 = %+v (ok %v), attendu 1.799 en stock", s, ok)
	}
	if e, ok := findCurrent(cur, "E85"); !ok || e.Price != 0 || !e.OutOfStock {
		t.Errorf("E85 = %+v (ok %v), attendu une rupture sans prix", e, ok)
	}
	if _, ok := findCurrent(cur, "E10"); ok {
		t.Error("E10 présent alors que son prix date de plus de 30 jours")
	}
	if _, ok := findCurrent(cur, "GPLC"); ok {
		t.Error("GPLC présent alors que la rupture est définitive")
	}

	// Station corse : seul prix daté de 2007, donc rien en cache.
	if _, ok := b.Current["20000005"]; ok {
		t.Error("Current[20000005] existe, attendu absent")
	}

	// Station d'outre-mer : seulement le gazole.
	cur971 := b.Current["97100001"]
	if len(cur971) != 1 {
		t.Fatalf("len(Current[97100001]) = %d, attendu 1", len(cur971))
	}
	if cur971[0].Fuel != "GAZOLE" {
		t.Errorf("Current[97100001][0] = %+v, attendu GAZOLE", cur971[0])
	}

	// Historique : seuls les prix récents du département 69.
	if len(b.History) != 2 {
		t.Fatalf("len(History) = %d, attendu 2", len(b.History))
	}
	byFuel := map[string]float64{}
	for _, h := range b.History {
		if h.Meta.StationID != "69100001" {
			t.Errorf("History station %q, attendu 69100001", h.Meta.StationID)
		}
		byFuel[h.Meta.Fuel] = h.Price
	}
	if byFuel["GAZOLE"] != 1.629 || byFuel["SP98"] != 1.799 {
		t.Errorf("History = %v, attendu GAZOLE 1.629 et SP98 1.799", byFuel)
	}
}
