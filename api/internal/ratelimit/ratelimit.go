// Package ratelimit limite le nombre de requêtes par IP.
//
// Principe (fenêtre fixe) : une clé Redis par IP et par minute, incrémentée
// à chaque requête et qui expire toute seule à la fin de la minute. Au delà
// de la limite on répond 429. Le compteur est dans Redis et pas en mémoire,
// donc la limite reste juste même avec plusieurs instances de l'API.
package ratelimit

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
	now    func() time.Time
	// Clé partagée avec le serveur Next.js (voir Middleware). Vide = désactivé.
	InternalKey string
}

func New(rdb *redis.Client, perMinute int) *Limiter {
	return &Limiter{rdb: rdb, limit: perMinute, window: time.Minute, now: time.Now}
}

// Result dit si la requête passe et combien il en reste dans la fenêtre.
type Result struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

// Allow compte une requête pour cette IP.
func (l *Limiter) Allow(ctx context.Context, ip string) (Result, error) {
	now := l.now()
	start := now.Truncate(l.window)
	key := fmt.Sprintf("ratelimit:%s:%d", ip, start.Unix())

	// INCR et EXPIRE envoyés ensemble. ExpireNX ne pose l'expiration qu'à la
	// première requête de la fenêtre.
	pipe := l.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, l.window)
	if _, err := pipe.Exec(ctx); err != nil {
		return Result{}, err
	}

	count := int(incr.Val())
	res := Result{Allowed: count <= l.limit, Remaining: max(l.limit-count, 0)}
	if !res.Allowed {
		res.RetryAfter = start.Add(l.window).Sub(now)
	}
	return res, nil
}

// isInternal vérifie la clé interne. subtle.ConstantTimeCompare compare en
// temps constant : on ne peut pas deviner la clé caractère par caractère en
// mesurant le temps de réponse.
func (l *Limiter) isInternal(r *http.Request) bool {
	got := r.Header.Get("X-Internal-Key")
	return l.InternalKey != "" && got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(l.InternalKey)) == 1
}

// ClientIP renvoie l'IP du client.
//
// Sans proxy devant l'API, c'est l'adresse de la connexion. Derrière un proxy
// (Render en production), c'est la dernière IP de X-Forwarded-For : celle
// ajoutée par le proxy. Les premières peuvent être inventées par le client,
// s'y fier permettrait de contourner la limite en changeant d'IP à chaque requête.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			parts := strings.Split(fwd, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Middleware applique la limite. Si Redis ne répond pas, on laisse passer
// la requête : mieux vaut une API sans limite quelques minutes qu'une API en panne.
func (l *Limiter) Middleware(trustProxy bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIP(r, trustProxy)
			// Requête du serveur Next.js avec la bonne clé : on compte sur l'IP du
			// visiteur qu'il transmet. Sans IP transmise (pages rendues et mises en
			// cache par Next), la requête n'est pas limitée.
			if l.isInternal(r) {
				forwarded := strings.TrimSpace(r.Header.Get("X-Client-IP"))
				if forwarded == "" {
					next.ServeHTTP(w, r)
					return
				}
				ip = forwarded
			}
			res, err := l.Allow(r.Context(), ip)
			if err != nil {
				log.Printf("rate limit indisponible, requête acceptée : %v", err)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(l.limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
			if !res.Allowed {
				seconds := int(res.RetryAfter.Seconds()) + 1
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				// Même format qu'une erreur GraphQL, pour que le front l'affiche simplement
				_, _ = fmt.Fprintf(w, `{"errors":[{"message":"trop de requêtes, réessaie dans %d s"}]}`, seconds)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
