package archive

import (
	"testing"
	"time"

	"github.com/ManuelDavid69/pumpcast/api/internal/ingest"
)

// at construit un instant dans le fuseau de Paris, comme les dates du flux.
func at(y, m, d, h, min int) time.Time {
	return time.Date(y, time.Month(m), d, h, min, 0, 0, ingest.Paris)
}

// utc construit un jour calendaire à minuit UTC, comme prices.Day.
func utc(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

// evt construit un changement de prix gazole.
func evt(v float64, t time.Time) ingest.Price {
	return ingest.Price{Fuel: "GAZOLE", Value: v, UpdatedAt: t}
}

// rup construit une rupture gazole temporaire. end zéro si en cours.
func rup(start, end time.Time) ingest.Rupture {
	return ingest.Rupture{Fuel: "GAZOLE", Start: start, End: end, Type: "temporaire"}
}

// priceAt renvoie le prix du jour day, et false s'il n'y a pas de point.
func priceAt(points []Point, day time.Time) (float64, bool) {
	for _, p := range points {
		if p.Day.Equal(day) {
			return p.Price, true
		}
	}
	return 0, false
}

func TestDailySeries(t *testing.T) {
	start := utc(2025, 1, 1)
	end := utc(2025, 1, 10)

	cases := []struct {
		name       string
		events     []ingest.Price
		carry      *LastPrice
		ruptures   []ingest.Rupture
		start, end time.Time
		wantLen    int
		wantFirst  time.Time
		wantLast   time.Time
		wantPrices []struct {
			day   time.Time
			price float64
		}
		wantMissing []time.Time
		wantCarry   *LastPrice
	}{
		{
			name:    "changements dans la journée",
			events:  []ingest.Price{evt(1.70, at(2025, 1, 3, 8, 0)), evt(1.72, at(2025, 1, 3, 18, 0)), evt(1.68, at(2025, 1, 6, 9, 0))},
			start:   start,
			end:     end,
			wantLen: 8, wantFirst: utc(2025, 1, 3), wantLast: utc(2025, 1, 10),
			wantPrices: []struct {
				day   time.Time
				price float64
			}{{utc(2025, 1, 3), 1.72}, {utc(2025, 1, 5), 1.72}, {utc(2025, 1, 6), 1.68}, {utc(2025, 1, 10), 1.68}},
			wantCarry: &LastPrice{Value: 1.68, Day: utc(2025, 1, 6)},
		},
		{
			name:    "événements dans le désordre",
			events:  []ingest.Price{evt(1.68, at(2025, 1, 6, 9, 0)), evt(1.72, at(2025, 1, 3, 18, 0)), evt(1.70, at(2025, 1, 3, 8, 0))},
			start:   start,
			end:     end,
			wantLen: 8, wantFirst: utc(2025, 1, 3), wantLast: utc(2025, 1, 10),
			wantPrices: []struct {
				day   time.Time
				price float64
			}{{utc(2025, 1, 3), 1.72}, {utc(2025, 1, 5), 1.72}, {utc(2025, 1, 6), 1.68}, {utc(2025, 1, 10), 1.68}},
			wantCarry: &LastPrice{Value: 1.68, Day: utc(2025, 1, 6)},
		},
		{
			name:    "report de l'année précédente",
			carry:   &LastPrice{Value: 1.65, Day: utc(2024, 12, 30)},
			start:   start,
			end:     end,
			wantLen: 10, wantFirst: utc(2025, 1, 1), wantLast: utc(2025, 1, 10),
			wantPrices: []struct {
				day   time.Time
				price float64
			}{{utc(2025, 1, 1), 1.65}, {utc(2025, 1, 10), 1.65}},
			wantCarry: &LastPrice{Value: 1.65, Day: utc(2024, 12, 30)},
		},
		{
			name:    "report trop vieux",
			events:  []ingest.Price{evt(1.70, at(2025, 1, 5, 10, 0))},
			carry:   &LastPrice{Value: 1.65, Day: utc(2024, 11, 1)},
			start:   start,
			end:     end,
			wantLen: 6, wantFirst: utc(2025, 1, 5), wantLast: utc(2025, 1, 10),
			wantPrices: []struct {
				day   time.Time
				price float64
			}{{utc(2025, 1, 5), 1.70}, {utc(2025, 1, 10), 1.70}},
			wantCarry: &LastPrice{Value: 1.70, Day: utc(2025, 1, 5)},
		},
		{
			name:    "prix abandonné après 30 jours",
			events:  []ingest.Price{evt(1.70, at(2025, 1, 1, 10, 0))},
			start:   start,
			end:     utc(2025, 2, 15),
			wantLen: 31, wantFirst: utc(2025, 1, 1), wantLast: utc(2025, 1, 31),
			wantPrices: []struct {
				day   time.Time
				price float64
			}{{utc(2025, 1, 1), 1.70}, {utc(2025, 1, 31), 1.70}},
			wantMissing: []time.Time{utc(2025, 2, 1)},
			wantCarry:   &LastPrice{Value: 1.70, Day: utc(2025, 1, 1)},
		},
		{
			name:     "rupture temporaire",
			events:   []ingest.Price{evt(1.70, at(2025, 1, 1, 10, 0))},
			ruptures: []ingest.Rupture{rup(at(2025, 1, 4, 13, 0), at(2025, 1, 6, 9, 0))},
			start:    start,
			end:      end,
			wantLen:  7, wantFirst: utc(2025, 1, 1), wantLast: utc(2025, 1, 10),
			wantPrices: []struct {
				day   time.Time
				price float64
			}{{utc(2025, 1, 1), 1.70}, {utc(2025, 1, 3), 1.70}, {utc(2025, 1, 7), 1.70}, {utc(2025, 1, 10), 1.70}},
			wantMissing: []time.Time{utc(2025, 1, 4), utc(2025, 1, 5), utc(2025, 1, 6)},
			wantCarry:   &LastPrice{Value: 1.70, Day: utc(2025, 1, 1)},
		},
		{
			name:     "rupture en cours",
			events:   []ingest.Price{evt(1.70, at(2025, 1, 1, 10, 0))},
			ruptures: []ingest.Rupture{rup(at(2025, 1, 8, 9, 0), time.Time{})},
			start:    start,
			end:      end,
			wantLen:  7, wantFirst: utc(2025, 1, 1), wantLast: utc(2025, 1, 7),
			wantPrices: []struct {
				day   time.Time
				price float64
			}{{utc(2025, 1, 7), 1.70}},
			wantCarry: &LastPrice{Value: 1.70, Day: utc(2025, 1, 1)},
		},
		{
			name:    "fuseau de Paris",
			events:  []ingest.Price{evt(1.80, at(2025, 1, 2, 23, 30))},
			start:   start,
			end:     end,
			wantLen: 9, wantFirst: utc(2025, 1, 2), wantLast: utc(2025, 1, 10),
			wantPrices: []struct {
				day   time.Time
				price float64
			}{{utc(2025, 1, 2), 1.80}, {utc(2025, 1, 10), 1.80}},
			wantCarry: &LastPrice{Value: 1.80, Day: utc(2025, 1, 2)},
		},
		{
			name:      "rien du tout",
			start:     start,
			end:       end,
			wantLen:   0,
			wantCarry: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			points, last := DailySeries(tc.events, tc.carry, tc.ruptures, tc.start, tc.end)

			if len(points) != tc.wantLen {
				t.Fatalf("len(points) = %d, attendu %d", len(points), tc.wantLen)
			}
			if tc.wantLen == 0 {
				return
			}
			if !points[0].Day.Equal(tc.wantFirst) {
				t.Errorf("premier point = %v, attendu %v", points[0].Day, tc.wantFirst)
			}
			if !points[len(points)-1].Day.Equal(tc.wantLast) {
				t.Errorf("dernier point = %v, attendu %v", points[len(points)-1].Day, tc.wantLast)
			}
			for _, w := range tc.wantPrices {
				if got, ok := priceAt(points, w.day); !ok || got != w.price {
					t.Errorf("prix du %v = %v (présent %v), attendu %v", w.day, got, ok, w.price)
				}
			}
			for _, d := range tc.wantMissing {
				if _, ok := priceAt(points, d); ok {
					t.Errorf("point inattendu le %v", d)
				}
			}

			if tc.wantCarry == nil {
				if last != nil {
					t.Errorf("last = %+v, attendu nil", last)
				}
				return
			}
			if last == nil || last.Value != tc.wantCarry.Value || !last.Day.Equal(tc.wantCarry.Day) {
				t.Errorf("last = %+v, attendu %+v", last, tc.wantCarry)
			}
		})
	}
}

// DailySeries ne doit pas réordonner le slice d'événements reçu.
func TestDailySeriesPreservesEvents(t *testing.T) {
	events := []ingest.Price{
		evt(1.68, at(2025, 1, 6, 9, 0)),
		evt(1.72, at(2025, 1, 3, 18, 0)),
		evt(1.70, at(2025, 1, 3, 8, 0)),
	}
	orig := append([]ingest.Price(nil), events...)

	DailySeries(events, nil, nil, utc(2025, 1, 1), utc(2025, 1, 10))

	if len(events) != len(orig) {
		t.Fatalf("len(events) = %d, attendu %d", len(events), len(orig))
	}
	for i := range events {
		if events[i] != orig[i] {
			t.Fatalf("events[%d] = %+v, attendu %+v (le slice a été modifié)", i, events[i], orig[i])
		}
	}
}
