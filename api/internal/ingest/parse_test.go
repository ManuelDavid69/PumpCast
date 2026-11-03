package ingest

import (
	"math"
	"os"
	"testing"
	"time"
)

// parseAll lit la fixture et renvoie les stations valides avec le nombre de rejets.
func parseAll(t *testing.T) ([]Station, int) {
	t.Helper()
	f, err := os.Open("testdata/sample.xml")
	if err != nil {
		t.Fatalf("ouverture de la fixture : %v", err)
	}
	defer f.Close()

	var list []Station
	skipped, err := Parse(f, func(st Station) error {
		list = append(list, st)
		return nil
	})
	if err != nil {
		t.Fatalf("Parse : %v", err)
	}
	return list, skipped
}

// station retrouve une station par identifiant, échoue si absente.
func station(t *testing.T, list []Station, id string) Station {
	t.Helper()
	for _, st := range list {
		if st.ID == id {
			return st
		}
	}
	t.Fatalf("station %s absente du flux", id)
	return Station{}
}

// priceOf retrouve un prix par carburant, échoue si absent.
func priceOf(t *testing.T, list []Price, fuel string) Price {
	t.Helper()
	for _, p := range list {
		if p.Fuel == fuel {
			return p
		}
	}
	t.Fatalf("carburant %s absent", fuel)
	return Price{}
}

func TestParseSample(t *testing.T) {
	list, skipped := parseAll(t)
	if skipped != 1 {
		t.Fatalf("skipped = %d, attendu 1 (la station sans id)", skipped)
	}
	if len(list) != 3 {
		t.Fatalf("len(list) = %d, attendu 3", len(list))
	}

	// Station complète : coordonnées, horaires, prix et ruptures.
	st := station(t, list, "69100001")
	if st.Department != "69" {
		t.Errorf("Department = %q, attendu 69", st.Department)
	}
	if st.RoadType != "R" {
		t.Errorf("RoadType = %q, attendu R", st.RoadType)
	}
	if st.City != "Villeurbanne" {
		t.Errorf("City = %q, attendu Villeurbanne", st.City)
	}
	if !st.HasCoords {
		t.Fatal("HasCoords = false, attendu true")
	}
	if math.Abs(st.Latitude-45.7712) > 1e-9 {
		t.Errorf("Latitude = %v, attendu 45.7712", st.Latitude)
	}
	if math.Abs(st.Longitude-4.8835) > 1e-9 {
		t.Errorf("Longitude = %v, attendu 4.8835", st.Longitude)
	}

	// Horaires : automate 24h/24, deux jours dont un créneau le lundi.
	if !st.Hours.Automate24h {
		t.Error("Automate24h = false, attendu true")
	}
	if len(st.Hours.Days) != 2 {
		t.Fatalf("len(Days) = %d, attendu 2", len(st.Hours.Days))
	}
	lundi := st.Hours.Days[0]
	if lundi.Name != "Lundi" || lundi.Closed || len(lundi.Slots) != 1 {
		t.Fatalf("Lundi = %+v, attendu un jour ouvert avec un créneau", lundi)
	}
	if lundi.Slots[0].Open != "07:00" || lundi.Slots[0].Close != "19:30" {
		t.Errorf("créneau = %+v, attendu 07:00-19:30", lundi.Slots[0])
	}
	dimanche := st.Hours.Days[1]
	if !dimanche.Closed || len(dimanche.Slots) != 0 {
		t.Errorf("Dimanche = %+v, attendu fermé sans créneau", dimanche)
	}

	// Services et prix.
	if len(st.Services) != 2 {
		t.Errorf("len(Services) = %d, attendu 2", len(st.Services))
	}
	if len(st.Prices) != 3 {
		t.Fatalf("len(Prices) = %d, attendu 3", len(st.Prices))
	}
	gazole := priceOf(t, st.Prices, "GAZOLE")
	if gazole.Value != 1.629 {
		t.Errorf("GAZOLE = %v, attendu 1.629", gazole.Value)
	}
	if want := time.Date(2026, 11, 3, 8, 15, 0, 0, paris); !gazole.UpdatedAt.Equal(want) {
		t.Errorf("UpdatedAt GAZOLE = %v, attendu %v", gazole.UpdatedAt, want)
	}
	if priceOf(t, st.Prices, "SP98").Value != 1.799 {
		t.Errorf("SP98 = %v, attendu 1.799", priceOf(t, st.Prices, "SP98").Value)
	}
	if priceOf(t, st.Prices, "E10").Value != 1.699 {
		t.Errorf("E10 = %v, attendu 1.699", priceOf(t, st.Prices, "E10").Value)
	}

	// Ruptures : E85 temporaire en cours, GPLC définitive.
	if len(st.Ruptures) != 2 {
		t.Fatalf("len(Ruptures) = %d, attendu 2", len(st.Ruptures))
	}
	var e85, gplc Rupture
	for _, r := range st.Ruptures {
		switch r.Fuel {
		case "E85":
			e85 = r
		case "GPLC":
			gplc = r
		}
	}
	if e85.Type != "temporaire" || !e85.Ongoing() {
		t.Errorf("rupture E85 = %+v, attendu temporaire en cours", e85)
	}
	if gplc.Type != "definitive" {
		t.Errorf("rupture GPLC = %+v, attendu definitive", gplc)
	}

	// Station corse : accent décodé, vieux prix en millièmes.
	st2 := station(t, list, "20000005")
	if st2.Department != "2A" || st2.RoadType != "A" {
		t.Errorf("20000005 = dept %q road %q, attendu 2A / A", st2.Department, st2.RoadType)
	}
	if st2.Address != "Route de l'Aéroport" {
		t.Errorf("Address = %q, attendu Route de l'Aéroport", st2.Address)
	}
	if st2.Services == nil || len(st2.Services) != 0 {
		t.Errorf("Services = %v, attendu non nil et vide", st2.Services)
	}
	sp95 := priceOf(t, st2.Prices, "SP95")
	if sp95.Value != 1.459 {
		t.Errorf("SP95 = %v, attendu 1.459", sp95.Value)
	}
	if want := time.Date(2007, 1, 2, 9, 1, 0, 0, paris); !sp95.UpdatedAt.Equal(want) {
		t.Errorf("UpdatedAt SP95 = %v, attendu %v", sp95.UpdatedAt, want)
	}
	if len(st2.Ruptures) != 1 || st2.Ruptures[0].Ongoing() {
		t.Errorf("Ruptures 20000005 = %+v, attendu une rupture terminée", st2.Ruptures)
	}

	// Station d'outre-mer sans coordonnées, hydrogène ignoré.
	st3 := station(t, list, "97100001")
	if st3.Department != "971" || st3.HasCoords {
		t.Errorf("97100001 = dept %q HasCoords %v, attendu 971 / false", st3.Department, st3.HasCoords)
	}
	if len(st3.Prices) != 1 || st3.Prices[0].Fuel != "GAZOLE" {
		t.Errorf("Prices 97100001 = %+v, attendu seulement GAZOLE", st3.Prices)
	}
}

func TestDepartmentFromPostalCode(t *testing.T) {
	tests := []struct {
		cp   string
		want string
	}{
		{cp: "69100", want: "69"},
		{cp: "01000", want: "01"},
		{cp: "20090", want: "2A"},
		{cp: "20199", want: "2A"},
		{cp: "20200", want: "2B"},
		{cp: "20600", want: "2B"},
		{cp: "97139", want: "971"},
		{cp: "97400", want: "974"},
		{cp: "98800", want: "988"},
		{cp: "6900", want: ""},
		{cp: "ABCDE", want: ""},
	}

	for _, tt := range tests {
		if got := DepartmentFromPostalCode(tt.cp); got != tt.want {
			t.Errorf("DepartmentFromPostalCode(%q) = %q, attendu %q", tt.cp, got, tt.want)
		}
	}
}

func TestNormalizeFuel(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "Gazole", want: "GAZOLE"},
		{in: "gazole", want: "GAZOLE"},
		{in: " SP95 ", want: "SP95"},
		{in: "SP98", want: "SP98"},
		{in: "E10", want: "E10"},
		{in: "SP95-E10", want: "E10"},
		{in: "E85", want: "E85"},
		{in: "GPLc", want: "GPLC"},
		{in: "GPL", want: "GPLC"},
		{in: "Hydrogène", want: ""},
	}

	for _, tt := range tests {
		if got := NormalizeFuel(tt.in); got != tt.want {
			t.Errorf("NormalizeFuel(%q) = %q, attendu %q", tt.in, got, tt.want)
		}
	}
}

func TestParsePrice(t *testing.T) {
	tests := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{in: "1.629", want: 1.629},
		{in: "1459", want: 1.459},
		{in: "0.05", wantErr: true},
		{in: "25", wantErr: true},
		{in: "abc", wantErr: true},
	}

	for _, tt := range tests {
		got, err := parsePrice(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parsePrice(%q) = %v, attendu une erreur", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePrice(%q) : erreur inattendue : %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parsePrice(%q) = %v, attendu %v", tt.in, got, tt.want)
		}
	}
}

func TestParseCoord(t *testing.T) {
	tests := []struct {
		in      string
		limit   float64
		want    float64
		wantErr bool
	}{
		{in: "4577120", limit: 90, want: 45.7712},
		{in: "45.77", limit: 90, want: 45.77},
		{in: "", limit: 90, wantErr: true},
		{in: "99999999999", limit: 90, wantErr: true},
	}

	for _, tt := range tests {
		got, err := parseCoord(tt.in, tt.limit)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseCoord(%q) = %v, attendu une erreur", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseCoord(%q) : erreur inattendue : %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseCoord(%q) = %v, attendu %v", tt.in, got, tt.want)
		}
	}
}
