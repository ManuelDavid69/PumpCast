// Package apperr distingue les erreurs qu'on peut montrer à l'utilisateur
// ("le mot de passe doit faire au moins 10 caractères") des erreurs internes
// (une panne de MongoDB) qui ne doivent pas sortir de l'API : leur message
// peut révéler des détails sur l'infrastructure.
package apperr

import "fmt"

// Public est une erreur dont le message est fait pour l'utilisateur.
type Public struct {
	Msg string
}

func (e *Public) Error() string { return e.Msg }

// New crée une erreur publique.
func New(format string, args ...any) error {
	return &Public{Msg: fmt.Sprintf(format, args...)}
}
