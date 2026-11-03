// Package health expose la route /health utilisée par Render et Docker.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Check vérifie qu'une dépendance (mongo, redis...) répond.
type Check func(ctx context.Context) error

type response struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// Handler renvoie 200 si toutes les dépendances répondent, 503 sinon.
// Le détail par dépendance aide à savoir ce qui est tombé.
func Handler(checks map[string]Check) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		res := response{Status: "ok", Checks: make(map[string]string, len(checks))}
		code := http.StatusOK
		for name, check := range checks {
			if err := check(ctx); err != nil {
				res.Checks[name] = "down"
				res.Status = "degraded"
				code = http.StatusServiceUnavailable
				continue
			}
			res.Checks[name] = "ok"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		// Si l'écriture échoue le client est déjà parti, rien de plus à faire
		_ = json.NewEncoder(w).Encode(res)
	}
}
