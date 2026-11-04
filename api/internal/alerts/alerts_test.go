package alerts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ManuelDavid69/pumpcast/api/internal/cache"
	"github.com/ManuelDavid69/pumpcast/api/internal/db"
	"github.com/ManuelDavid69/pumpcast/api/internal/predictions"
	"github.com/ManuelDavid69/pumpcast/api/internal/prices"
	"github.com/ManuelDavid69/pumpcast/api/internal/stations"
	"github.com/ManuelDavid69/pumpcast/api/internal/users"
)

func ptr[T any](v T) *T { return &v }

func TestConditionMet(t *testing.T) {
	seuil := Alert{Type: TypeThreshold, Threshold: ptr(1.80)}
	tests := []struct {
		name  string
		alert Alert
		s     Situation
		want  bool
	}{
		{"sous le seuil", seuil, Situation{Price: 1.79, HasPrice: true}, true},
		{"pile au seuil", seuil, Situation{Price: 1.80, HasPrice: true}, true},
		{"au-dessus", seuil, Situation{Price: 1.81, HasPrice: true}, false},
		{"rupture (pas de prix)", seuil, Situation{}, false},
		{"baisse prévue", Alert{Type: TypePredictedDrop}, Situation{Recommendation: "wait"}, true},
		{"baisse demandée mais prix stable", Alert{Type: TypePredictedDrop}, Situation{Recommendation: "stable"}, false},
		{"hausse prévue", Alert{Type: TypePredictedRise}, Situation{Recommendation: "fill_now"}, true},
		{"pas de conseil", Alert{Type: TypePredictedRise}, Situation{}, false},
	}
	for _, tt := range tests {
		if got := ConditionMet(tt.alert, tt.s); got != tt.want {
			t.Errorf("%s : %v, attendu %v", tt.name, got, tt.want)
		}
	}
}

func TestDecide(t *testing.T) {
	now := time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC)
	recent := now.Add(-2 * time.Hour)
	old := now.Add(-2 * 24 * time.Hour)
	tests := []struct {
		name                  string
		alert                 Alert
		met                   bool
		wantSend, wantTrigger bool
	}{
		{"la condition devient vraie : email", Alert{}, true, true, true},
		{"toujours vraie : pas de 2e email", Alert{Triggered: true}, true, false, true},
		{"redevient fausse : alerte réarmée", Alert{Triggered: true}, false, false, false},
		{"de nouveau vraie mais email récent : on attend", Alert{LastSentAt: &recent}, true, false, true},
		{"de nouveau vraie, dernier email il y a 2 jours", Alert{LastSentAt: &old}, true, true, true},
	}
	for _, tt := range tests {
		send, trig := Decide(tt.alert, tt.met, now)
		if send != tt.wantSend || trig != tt.wantTrigger {
			t.Errorf("%s : envoi %v état %v, attendu %v / %v", tt.name, send, trig, tt.wantSend, tt.wantTrigger)
		}
	}
}

func TestBuildMessage(t *testing.T) {
	st := StationInfo{ID: "69100001", Address: "145, Rue Anatole France", City: "Villeurbanne"}
	m := BuildMessage("manu@example.fr", Alert{Type: TypeThreshold, Fuel: "GAZOLE", Threshold: ptr(2.15)}, st,
		Situation{Price: 2.143, HasPrice: true}, "https://pumpcast.example/")
	if m.Subject != "Gazole à 2,143 € à Villeurbanne" {
		t.Errorf("sujet = %q", m.Subject)
	}
	for _, want := range []string{"ton seuil : 2,150 €", "https://pumpcast.example/stations/69100001?carburant=GAZOLE", "https://pumpcast.example/compte"} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("le texte ne contient pas %q :\n%s", want, m.Text)
		}
	}

	drop := BuildMessage("manu@example.fr", Alert{Type: TypePredictedDrop, Fuel: "E10"}, st,
		Situation{TargetPrice: 1.95, TargetDate: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)}, "https://pumpcast.example")
	if !strings.Contains(drop.Text, "descendre vers 1,950 € d'ici vendredi") || !strings.Contains(drop.Text, "/modele") {
		t.Errorf("email de baisse :\n%s", drop.Text)
	}
}

func TestTextToHTMLEscapes(t *testing.T) {
	out := textToHTML("Station <script>alert(1)</script>\n\nDeuxième paragraphe")
	if strings.Contains(out, "<script>") || !strings.Contains(out, "&lt;script&gt;") {
		t.Errorf("le HTML n'est pas échappé : %s", out)
	}
	if strings.Count(out, "<p>") != 2 {
		t.Errorf("2 paragraphes attendus : %s", out)
	}
}

func TestBrevoSender(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("api-key") != "cle-test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	b := BrevoSender{APIKey: "cle-test", FromEmail: "alertes@pumpcast.example", FromName: "PumpCast", URL: srv.URL}
	if err := b.Send(context.Background(), Message{To: "manu@example.fr", Subject: "Sujet", Text: "Bonjour"}); err != nil {
		t.Fatal(err)
	}
	to := got["to"].([]any)[0].(map[string]any)
	if to["email"] != "manu@example.fr" || got["subject"] != "Sujet" || got["textContent"] != "Bonjour" {
		t.Errorf("requête envoyée à Brevo : %v", got)
	}

	bad := BrevoSender{APIKey: "mauvaise-cle", FromEmail: "a@b.fr", URL: srv.URL}
	if err := bad.Send(context.Background(), Message{To: "x@y.fr"}); err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("une erreur Brevo doit remonter : %v", err)
	}
}

// fakeSender garde les emails au lieu de les envoyer
type fakeSender struct {
	mu   sync.Mutex
	sent []Message
}

func (f *fakeSender) Send(_ context.Context, m Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, m)
	return nil
}

// Le parcours complet sur un vrai MongoDB et un vrai Redis
func TestCheckerRun(t *testing.T) {
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI non défini")
	}
	ctx := context.Background()
	mc, err := db.Connect(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mc.Disconnect(ctx) })
	database := mc.Database("pumpcast_test_alerts_" + time.Now().Format("150405"))
	t.Cleanup(func() { _ = database.Drop(ctx) })
	if err := db.EnsureSchema(ctx, database); err != nil {
		t.Fatal(err)
	}
	rdb, err := cache.Connect(ctx, "redis://localhost:6379/15")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rdb.FlushDB(ctx).Err(); _ = rdb.Close() })

	userRepo := users.NewRepository(database)
	u, err := userRepo.Create(ctx, "manu@example.fr", "hash")
	if err != nil {
		t.Fatal(err)
	}
	stRepo := stations.NewRepository(database)
	if _, err := stRepo.UpsertMany(ctx, []stations.Station{{ID: "A", Address: "1 rue de Test", City: "Lyon", Department: "69", Services: []string{}}}); err != nil {
		t.Fatal(err)
	}
	current := prices.NewCurrentCache(rdb)
	setPrice := func(p float64) {
		t.Helper()
		if err := current.SaveAll(ctx, map[string][]prices.Current{"A": {{Fuel: "GAZOLE", Price: p, UpdatedAt: time.Now()}}}); err != nil {
			t.Fatal(err)
		}
	}
	alertRepo := NewRepository(database)
	if _, err := alertRepo.Create(ctx, Alert{UserID: u.ID, StationID: "A", Fuel: "GAZOLE", Type: TypeThreshold, Threshold: ptr(1.80)}); err != nil {
		t.Fatal(err)
	}

	sender := &fakeSender{}
	now := time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC)
	checker := &Checker{
		Alerts: alertRepo, Users: userRepo, Stations: stRepo, Current: current,
		Predictions: predictions.NewRepository(database), Sender: sender, SiteURL: "https://pumpcast.example",
		Now: func() time.Time { return now },
	}
	runOnce := func() Summary {
		t.Helper()
		sum, err := checker.Run(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return sum
	}

	setPrice(1.85)
	if sum := runOnce(); sum.Sent != 0 {
		t.Fatalf("au-dessus du seuil : %v", sum)
	}
	setPrice(1.79)
	if sum := runOnce(); sum.Sent != 1 {
		t.Fatalf("passe sous le seuil : %v", sum)
	}
	if sender.sent[0].To != "manu@example.fr" || !strings.Contains(sender.sent[0].Subject, "1,790 €") {
		t.Errorf("email = %+v", sender.sent[0])
	}
	// Une heure plus tard, toujours sous le seuil : pas de 2e email
	now = now.Add(time.Hour)
	if sum := runOnce(); sum.Sent != 0 {
		t.Fatalf("toujours sous le seuil : %v", sum)
	}
	list, _ := alertRepo.ByUser(ctx, u.ID)
	if !list[0].Triggered || list[0].LastSentAt == nil {
		t.Errorf("état de l'alerte : %+v", list[0])
	}
}

func TestTidyName(t *testing.T) {
	for in, want := range map[string]string{
		"VILLEURBANNE":          "Villeurbanne",
		"SAINT-DENIS-LÈS-BOURG": "Saint-Denis-Lès-Bourg",
		"ROUTE D'HEYRIEUX":      "Route D'Heyrieux",
		"Loire-sur-Rhône":       "Loire-sur-Rhône",
	} {
		if got := tidyName(in); got != want {
			t.Errorf("tidyName(%q) = %q, attendu %q", in, got, want)
		}
	}
}
