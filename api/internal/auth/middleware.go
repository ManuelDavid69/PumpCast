package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type ctxKey struct{}

// ErrNotLoggedIn est renvoyé par les resolvers qui demandent un compte.
var ErrNotLoggedIn = errors.New("il faut être connecté")

// Middleware lit l'en-tête "Authorization: Bearer <jeton>" et met
// l'identifiant de l'utilisateur dans le contexte de la requête.
//
// Un jeton absent ou invalide ne bloque pas la requête : la plupart des
// requêtes (stations, prix) sont publiques. Ce sont les resolvers qui
// demandent un compte qui vérifient avec UserID.
func Middleware(tokens *Tokens) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if raw, ok := strings.CutPrefix(header, "Bearer "); ok {
				if id, err := tokens.Parse(strings.TrimSpace(raw)); err == nil {
					r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, id))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserID renvoie l'utilisateur connecté, ou ErrNotLoggedIn.
func UserID(ctx context.Context) (string, error) {
	id, ok := ctx.Value(ctxKey{}).(string)
	if !ok || id == "" {
		return "", ErrNotLoggedIn
	}
	return id, nil
}

// WithUser sert aux tests : simule un utilisateur connecté.
func WithUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, userID)
}
