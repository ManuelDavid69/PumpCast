package graph

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/ManuelDavid69/pumpcast/api/internal/apperr"
)

func TestPresentError(t *testing.T) {
	ctx := context.Background()

	// Erreur faite pour l'utilisateur, même enveloppée : message gardé
	pub := fmt.Errorf("création : %w", apperr.New("le seuil doit être un prix entre 0,30 € et 5 €"))
	if got := presentError(ctx, pub).Message; got != "le seuil doit être un prix entre 0,30 € et 5 €" {
		t.Errorf("erreur publique = %q", got)
	}

	// Panne interne : le détail ne doit pas sortir
	internal := errors.New("lecture des stations : connection refused 10.0.3.7:27017")
	if got := presentError(ctx, internal).Message; got != "erreur interne, réessaie dans un moment" {
		t.Errorf("erreur interne visible : %q", got)
	}

	// Erreur de requête GraphQL (champ inconnu...) : utile au développeur du front
	q := gqlerror.Errorf("Cannot query field \"prix\" on type \"Station\".")
	if got := presentError(ctx, q).Message; got != q.Message {
		t.Errorf("erreur de requête = %q", got)
	}
}
