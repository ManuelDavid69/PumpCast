package prices

import (
	"testing"
	"time"
	_ "time/tzdata" // garantit la présence du fuseau Europe/Paris
)

func TestDay(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatalf("chargement du fuseau Paris : %v", err)
	}

	tests := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{
			name: "minuit passé à Paris",
			in:   time.Date(2026, 11, 3, 0, 30, 0, 0, paris),
			want: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "23h30 UTC = lendemain à Paris",
			in:   time.Date(2026, 11, 2, 23, 30, 0, 0, time.UTC),
			want: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "heure d'été",
			in:   time.Date(2026, 7, 14, 21, 59, 0, 0, time.UTC),
			want: time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Day(tt.in, paris); !got.Equal(tt.want) {
				t.Errorf("Day(%v) = %v, attendu %v", tt.in, got, tt.want)
			}
		})
	}
}
