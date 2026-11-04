package prices

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// Test sur un vrai Redis (base 15), lancé si REDIS_TEST_URL est défini
func TestCurrentCache(t *testing.T) {
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		t.Skip("REDIS_TEST_URL non défini")
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	// Une base Redis par package : go test lance les packages en parallèle
	opts.DB = 14
	rdb := redis.NewClient(opts)
	ctx := context.Background()
	if err := rdb.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rdb.FlushDB(ctx).Err(); _ = rdb.Close() })

	c := NewCurrentCache(rdb)
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	if err := c.SaveAll(ctx, map[string][]Current{
		"A": {{Fuel: "GAZOLE", Price: 1.80, UpdatedAt: now}},
		"B": {{Fuel: "E10", Price: 1.75, UpdatedAt: now}},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := c.GetMany(ctx, []string{"A", "B", "inconnue"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["A"][0].Price != 1.80 || got["B"][0].Fuel != "E10" {
		t.Fatalf("GetMany = %+v", got)
	}

	// Ingestion suivante : B a quitté le flux, il doit disparaître du cache
	if err := c.SaveAll(ctx, map[string][]Current{"A": {{Fuel: "GAZOLE", Price: 1.79, UpdatedAt: now}}}); err != nil {
		t.Fatal(err)
	}
	if b, _ := c.Get(ctx, "B"); b != nil {
		t.Errorf("B devrait avoir disparu : %+v", b)
	}
	if a, _ := c.Get(ctx, "A"); len(a) != 1 || a[0].Price != 1.79 {
		t.Errorf("A = %+v", a)
	}
	// Pas de clé temporaire qui traîne, et le hash a bien une expiration
	if n := rdb.Exists(ctx, currentTempKey).Val(); n != 0 {
		t.Error("la clé temporaire n'a pas été renommée")
	}
	if ttl := rdb.TTL(ctx, CurrentKey).Val(); ttl <= 0 || ttl > currentTTL {
		t.Errorf("TTL = %v", ttl)
	}
}
