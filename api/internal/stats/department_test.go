package stats

import (
	"testing"
	"time"
)

func TestAccumulator(t *testing.T) {
	d1 := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)

	t.Run("résultats complets", func(t *testing.T) {
		a := NewAccumulator()
		a.Add(d1, "69", "GAZOLE", 1.60)
		a.Add(d1, "69", "GAZOLE", 1.70)
		a.Add(d1, "69", "GAZOLE", 1.65)
		a.Add(d1, "69", "SP98", 1.90)
		a.Add(d1, "01", "GAZOLE", 1.55)
		a.Add(d2, "69", "GAZOLE", 1.62)

		res := a.Results(d2)
		if len(res) != 4 {
			t.Fatalf("len(Results(d2)) = %d, attendu 4", len(res))
		}

		// Ordre : jour, puis département, puis carburant.
		wantOrder := [][2]string{
			{"01", "GAZOLE"},
			{"69", "GAZOLE"},
			{"69", "SP98"},
			{"69", "GAZOLE"},
		}
		wantDays := []time.Time{d1, d1, d1, d2}
		for i, r := range res {
			if !r.TS.Equal(wantDays[i]) || r.Meta.Department != wantOrder[i][0] || r.Meta.Fuel != wantOrder[i][1] {
				t.Errorf("res[%d] = %v / %q / %q, attendu %v / %q / %q",
					i, r.TS, r.Meta.Department, r.Meta.Fuel, wantDays[i], wantOrder[i][0], wantOrder[i][1])
			}
		}

		// Ligne agrégée : trois prix gazole en 69.
		gazole := res[1]
		if gazole.AvgPrice != 1.65 || gazole.MinPrice != 1.60 || gazole.MaxPrice != 1.70 || gazole.StationCount != 3 {
			t.Errorf("69/GAZOLE = %+v, attendu avg 1.65 min 1.60 max 1.70 count 3", gazole)
		}

		// Ligne à une seule valeur.
		ain := res[0]
		if ain.StationCount != 1 || ain.AvgPrice != 1.55 || ain.MinPrice != 1.55 || ain.MaxPrice != 1.55 {
			t.Errorf("01/GAZOLE = %+v, attendu avg=min=max 1.55 count 1", ain)
		}

		// lastDay d1 : le jour d2 est après, donc absent.
		res1 := a.Results(d1)
		if len(res1) != 3 {
			t.Fatalf("len(Results(d1)) = %d, attendu 3", len(res1))
		}
		for _, r := range res1 {
			if !r.TS.Equal(d1) {
				t.Errorf("Results(d1) contient %v, attendu uniquement d1", r.TS)
			}
		}
	})

	t.Run("arrondi", func(t *testing.T) {
		a := NewAccumulator()
		a.Add(d1, "38", "E10", 1.001)
		a.Add(d1, "38", "E10", 1.002)
		a.Add(d1, "38", "E10", 1.002)

		res := a.Results(d1)
		if len(res) != 1 {
			t.Fatalf("len = %d, attendu 1", len(res))
		}
		if res[0].AvgPrice != 1.0017 {
			t.Errorf("AvgPrice = %v, attendu 1.0017", res[0].AvgPrice)
		}
	})

	t.Run("vide", func(t *testing.T) {
		a := NewAccumulator()
		if res := a.Results(d2); len(res) != 0 {
			t.Errorf("len = %d, attendu 0", len(res))
		}
	})
}
