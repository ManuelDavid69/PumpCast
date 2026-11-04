package graph

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/99designs/gqlgen/client"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/ManuelDavid69/pumpcast/api/internal/cache"
	"github.com/ManuelDavid69/pumpcast/api/internal/dataloader"
	"github.com/ManuelDavid69/pumpcast/api/internal/db"
	"github.com/ManuelDavid69/pumpcast/api/internal/ingest"
	"github.com/ManuelDavid69/pumpcast/api/internal/predictions"
	"github.com/ManuelDavid69/pumpcast/api/internal/prices"
	"github.com/ManuelDavid69/pumpcast/api/internal/stations"
	"github.com/ManuelDavid69/pumpcast/api/internal/stats"
)

// commandCounter compte les commandes envoyées à MongoDB, par type et par
// collection ("find prices", "aggregate predictions"...).
type commandCounter struct {
	mu     sync.Mutex
	counts map[string]int
}

func (c *commandCounter) started(_ context.Context, e *event.CommandStartedEvent) {
	coll, _ := e.Command.Lookup(e.CommandName).StringValueOK()
	c.mu.Lock()
	c.counts[e.CommandName+" "+coll]++
	c.mu.Unlock()
}

func (c *commandCounter) get(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[key]
}

func (c *commandCounter) reset() {
	c.mu.Lock()
	c.counts = map[string]int{}
	c.mu.Unlock()
}

type testEnv struct {
	client   *client.Client
	counter  *commandCounter
	database *mongo.Database
	rdb      *redis.Client
}

// setup prépare une base MongoDB et une base Redis de test avec quelques
// stations autour de Lyon. Lancé seulement si MONGO_TEST_URI est défini.
func setup(t *testing.T) *testEnv {
	t.Helper()
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI non défini")
	}
	ctx := context.Background()

	counter := &commandCounter{counts: map[string]int{}}
	mc, err := mongo.Connect(options.Client().ApplyURI(uri).
		SetMonitor(&event.CommandMonitor{Started: counter.started}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mc.Disconnect(ctx) })
	database := mc.Database("pumpcast_test_graph_" + time.Now().Format("150405.000000")[7:])
	t.Cleanup(func() { _ = database.Drop(ctx) })
	if err := db.EnsureSchema(ctx, database); err != nil {
		t.Fatal(err)
	}

	// Base Redis 15 pour ne pas toucher aux données de dev (base 0)
	redisURL := os.Getenv("REDIS_TEST_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/15"
	}
	rdb, err := cache.Connect(ctx, redisURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := rdb.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rdb.FlushDB(ctx).Err(); _ = rdb.Close() })

	// A, B et C sont actives autour de Lyon, D est fermée (pas de prix actuels),
	// E est à Paris (hors rayon)
	stRepo := stations.NewRepository(database)
	mk := func(id string, lat, lng float64, road string) stations.Station {
		return stations.Station{ID: id, Address: "rue " + id, City: "LYON", PostalCode: "69003", Department: "69",
			Location: stations.NewPoint(lat, lng), RoadType: road, Services: []string{}}
	}
	paris := mk("E", 48.8566, 2.3522, "R")
	paris.Department, paris.City, paris.PostalCode = "75", "PARIS", "75001"
	if _, err := stRepo.UpsertMany(ctx, []stations.Station{
		mk("A", 45.7600, 4.8500, "R"),
		mk("B", 45.7650, 4.8600, "R"),
		mk("C", 45.7700, 4.8700, "A"),
		mk("D", 45.7610, 4.8510, "R"),
		paris,
	}); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 4, 12, 0, 0, 0, ingest.Paris)
	current := prices.NewCurrentCache(rdb)
	if err := current.SaveAll(ctx, map[string][]prices.Current{
		"A": {{Fuel: "GAZOLE", Price: 1.80, UpdatedAt: now}},
		"B": {{Fuel: "GAZOLE", Price: 1.70, UpdatedAt: now}, {Fuel: "E10", Price: 1.75, UpdatedAt: now}},
		"C": {{Fuel: "E10", Price: 1.85, UpdatedAt: now}, {Fuel: "GAZOLE", OutOfStock: true}},
		"E": {{Fuel: "GAZOLE", Price: 1.60, UpdatedAt: now}},
	}); err != nil {
		t.Fatal(err)
	}

	// 10 jours d'historique gazole pour A, B et C
	history := prices.NewHistoryRepository(database)
	var points []prices.DailyPrice
	for i := 0; i < 10; i++ {
		day := time.Date(2026, 10, 26+i, 0, 0, 0, 0, time.UTC)
		for _, id := range []string{"A", "B", "C"} {
			points = append(points, prices.DailyPrice{TS: day, Meta: prices.Meta{StationID: id, Fuel: "GAZOLE"}, Price: 1.70 + float64(i)/100})
		}
	}
	if err := history.InsertMany(ctx, points); err != nil {
		t.Fatal(err)
	}

	preds := predictions.NewRepository(database)
	res := &Resolver{
		Stations:    stRepo,
		Current:     current,
		Stats:       stats.NewRepository(database),
		Predictions: preds,
		Redis:       rdb,
		Now:         func() time.Time { return now },
	}
	h := NewHandler(res, dataloader.Sources{Current: current, History: history, Predictions: preds})
	return &testEnv{client: client.New(h), counter: counter, database: database, rdb: rdb}
}

type nearResp struct {
	StationsNear []struct {
		ID            string
		DistanceKm    float64
		CurrentPrices []struct {
			Fuel  string
			Price *float64
		}
		Prediction   *struct{ Recommendation string }
		PriceHistory []struct {
			Date  string
			Price float64
		}
	}
}

func ids(r nearResp) string {
	var out []string
	for _, s := range r.StationsNear {
		out = append(out, s.ID)
	}
	return strings.Join(out, ",")
}

func TestStationsNear(t *testing.T) {
	env := setup(t)

	// Tri par distance : D est fermée et E trop loin, elles n'apparaissent pas
	var r nearResp
	env.client.MustPost(`{ stationsNear(lat: 45.76, lng: 4.85, radiusKm: 5) { id distanceKm } }`, &r)
	if got := ids(r); got != "A,B,C" {
		t.Fatalf("stations = %s, attendu A,B,C", got)
	}
	if r.StationsNear[0].DistanceKm > r.StationsNear[1].DistanceKm {
		t.Errorf("pas trié par distance : %+v", r.StationsNear)
	}

	// Tri par prix du gazole : la rupture de C passe en dernier
	r = nearResp{}
	env.client.MustPost(`{ stationsNear(lat: 45.76, lng: 4.85, radiusKm: 5, fuel: GAZOLE, sort: PRICE) { id } }`, &r)
	if got := ids(r); got != "B,A,C" {
		t.Errorf("tri par prix = %s, attendu B,A,C", got)
	}

	// Filtre carburant : seules B et C vendent de l'E10
	r = nearResp{}
	env.client.MustPost(`{ stationsNear(lat: 45.76, lng: 4.85, radiusKm: 5, fuel: E10) { id } }`, &r)
	if got := ids(r); got != "B,C" {
		t.Errorf("filtre E10 = %s, attendu B,C", got)
	}

	// Paramètre invalide
	err := env.client.Post(`{ stationsNear(lat: 45.76, lng: 4.85, sort: PRICE) { id } }`, &r)
	if err == nil || !strings.Contains(err.Error(), "carburant") {
		t.Errorf("erreur attendue pour le tri par prix sans carburant, reçu %v", err)
	}
}

// Le test qui justifie le dataloader : 3 stations avec leur historique et leur
// prédiction doivent donner 1 requête sur prices et 1 sur predictions, pas 3 + 3.
func TestNoNPlusOne(t *testing.T) {
	env := setup(t)
	env.counter.reset()

	var r nearResp
	env.client.MustPost(`{ stationsNear(lat: 45.76, lng: 4.85, radiusKm: 5) {
		id
		currentPrices { fuel price }
		priceHistory(fuel: GAZOLE, days: 5) { date price }
		prediction(fuel: GAZOLE) { recommendation }
	} }`, &r)

	if len(r.StationsNear) != 3 {
		t.Fatalf("%d stations, attendu 3", len(r.StationsNear))
	}
	if n := env.counter.get("find prices"); n != 1 {
		t.Errorf("%d requêtes sur prices pour 3 stations, attendu 1", n)
	}
	if n := env.counter.get("aggregate predictions"); n != 1 {
		t.Errorf("%d requêtes sur predictions pour 3 stations, attendu 1", n)
	}

	// Les 5 derniers jours, du 31 octobre au 4 novembre
	h := r.StationsNear[0].PriceHistory
	if len(h) != 5 || h[0].Date != "2026-10-31" || h[4].Date != "2026-11-04" {
		t.Errorf("historique = %+v, attendu 5 jours du 31/10 au 04/11", h)
	}
}

func TestDepartmentStatsCache(t *testing.T) {
	env := setup(t)
	ctx := context.Background()
	repo := stats.NewRepository(env.database)
	day := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	save := func(avg float64) {
		t.Helper()
		if err := repo.ReplaceDays(ctx, day, day, []stats.DepartmentDaily{
			{TS: day, Meta: stats.Meta{Department: "69", Fuel: "GAZOLE"}, AvgPrice: avg, MinPrice: 1.70, MaxPrice: 1.80, StationCount: 2},
		}); err != nil {
			t.Fatal(err)
		}
	}
	type resp struct {
		DepartmentStats struct {
			AvgPrice        float64
			CheapestStation struct{ ID string }
		}
	}
	query := `{ departmentStats(department: "69", fuel: GAZOLE) { avgPrice cheapestStation { id } } }`

	save(1.75)
	var r resp
	env.client.MustPost(query, &r)
	if r.DepartmentStats.AvgPrice != 1.75 || r.DepartmentStats.CheapestStation.ID != "B" {
		t.Fatalf("1re lecture = %+v, attendu 1.75 et la station B", r.DepartmentStats)
	}

	// La base change mais la valeur vient du cache
	save(1.99)
	env.client.MustPost(query, &r)
	if r.DepartmentStats.AvgPrice != 1.75 {
		t.Errorf("2e lecture = %v, attendu 1.75 (cache)", r.DepartmentStats.AvgPrice)
	}

	// Après une ingestion le cache est vidé, on lit la nouvelle valeur
	if _, err := cache.DeletePrefix(ctx, env.rdb, cache.StatsPrefix); err != nil {
		t.Fatal(err)
	}
	env.client.MustPost(query, &r)
	if r.DepartmentStats.AvgPrice != 1.99 {
		t.Errorf("après invalidation = %v, attendu 1.99", r.DepartmentStats.AvgPrice)
	}
}

func TestCompareStations(t *testing.T) {
	env := setup(t)
	var r struct{ CompareStations []struct{ ID string } }
	env.client.MustPost(`{ compareStations(ids: ["B", "inconnue", "A", "B"]) { id } }`, &r)
	if len(r.CompareStations) != 2 || r.CompareStations[0].ID != "B" || r.CompareStations[1].ID != "A" {
		t.Errorf("comparaison = %+v, attendu B puis A", r.CompareStations)
	}
	err := env.client.Post(`{ compareStations(ids: ["A","B","C","D","E","F"]) { id } }`, &r)
	if err == nil {
		t.Error("erreur attendue au delà de 5 stations")
	}
}
