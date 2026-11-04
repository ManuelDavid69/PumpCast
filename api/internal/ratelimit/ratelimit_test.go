package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remote     string
		forwarded  string
		trustProxy bool
		want       string
	}{
		{"sans proxy", "203.0.113.7:51234", "", false, "203.0.113.7"},
		{"proxy de confiance sans en-tête", "203.0.113.7:51234", "", true, "203.0.113.7"},
		{"en-tête ignoré sans proxy", "203.0.113.7:51234", "1.1.1.1, 198.51.100.9", false, "203.0.113.7"},
		// Le client peut inventer 1.1.1.1, seule la dernière IP vient du proxy
		{"dernière IP du proxy", "203.0.113.7:51234", "1.1.1.1, 198.51.100.9", true, "198.51.100.9"},
		{"une seule IP", "203.0.113.7:51234", "1.1.1.1", true, "1.1.1.1"},
		{"adresse sans port", "203.0.113.7", "", false, "203.0.113.7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/graphql", nil)
			r.RemoteAddr = tt.remote
			if tt.forwarded != "" {
				r.Header.Set("X-Forwarded-For", tt.forwarded)
			}
			if got := ClientIP(r, tt.trustProxy); got != tt.want {
				t.Errorf("ClientIP = %q, attendu %q", got, tt.want)
			}
		})
	}
}

// testRedis se connecte à la base Redis de test, ou saute le test.
func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		t.Skip("REDIS_TEST_URL non défini")
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opts)
	ctx := context.Background()
	if err := rdb.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rdb.FlushDB(ctx).Err(); _ = rdb.Close() })
	return rdb
}

func TestAllow(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	clock := time.Date(2026, 11, 4, 12, 0, 30, 0, time.UTC)
	l := &Limiter{rdb: rdb, limit: 3, window: time.Minute, now: func() time.Time { return clock }}

	for i, wantRemaining := range []int{2, 1, 0} {
		res, err := l.Allow(ctx, "1.2.3.4")
		if err != nil {
			t.Fatal(err)
		}
		if !res.Allowed || res.Remaining != wantRemaining {
			t.Fatalf("appel %d : %+v, attendu autorisé avec %d restantes", i+1, res, wantRemaining)
		}
	}

	// 4e appel dans la même minute : refusé jusqu'à 12:01:00, donc 30 s
	res, err := l.Allow(ctx, "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if res.Allowed || res.Remaining != 0 || res.RetryAfter != 30*time.Second {
		t.Errorf("4e appel : %+v, attendu refusé avec RetryAfter 30s", res)
	}

	// Chaque IP a son propre compteur
	if res, _ := l.Allow(ctx, "5.6.7.8"); !res.Allowed {
		t.Error("une autre IP devrait passer")
	}

	// La clé expire toute seule à la fin de la fenêtre
	ttl, err := rdb.TTL(ctx, "ratelimit:1.2.3.4:"+strconv.FormatInt(clock.Truncate(time.Minute).Unix(), 10)).Result()
	if err != nil {
		t.Fatal(err)
	}
	if ttl <= 0 || ttl > time.Minute {
		t.Errorf("TTL = %v, attendu entre 0 et 1 min", ttl)
	}

	// Nouvelle minute : le compteur repart de zéro
	clock = time.Date(2026, 11, 4, 12, 1, 0, 0, time.UTC)
	if res, _ := l.Allow(ctx, "1.2.3.4"); !res.Allowed || res.Remaining != 2 {
		t.Errorf("nouvelle fenêtre : %+v, attendu autorisé avec 2 restantes", res)
	}
}

func TestMiddleware(t *testing.T) {
	rdb := testRedis(t)
	h := New(rdb, 2).Middleware(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/graphql", nil)
		r.RemoteAddr = "203.0.113.7:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	for _, wantRemaining := range []string{"1", "0"} {
		w := send()
		if w.Code != http.StatusOK || w.Header().Get("X-RateLimit-Remaining") != wantRemaining {
			t.Fatalf("code %d, restantes %q, attendu 200 et %s", w.Code, w.Header().Get("X-RateLimit-Remaining"), wantRemaining)
		}
	}

	w := send()
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("3e requête : code %d, attendu 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" || w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("en-têtes du 429 incomplets : %v", w.Header())
	}
	if !strings.Contains(w.Body.String(), "trop de requêtes") {
		t.Errorf("corps du 429 = %s", w.Body.String())
	}
}

// Si Redis est en panne, l'API doit continuer à répondre
func TestMiddlewareRedisDown(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:1", DialTimeout: 100 * time.Millisecond, MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	called := false
	h := New(rdb, 2).Middleware(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/graphql", nil))
	if !called || w.Code != http.StatusOK {
		t.Errorf("la requête aurait dû passer (code %d)", w.Code)
	}
}
