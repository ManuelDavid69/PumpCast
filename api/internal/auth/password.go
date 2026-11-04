// Package auth gère les mots de passe, les jetons de connexion (JWT) et
// l'identification de l'utilisateur à chaque requête.
package auth

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// Coût de bcrypt : chaque hachage prend environ 250 ms. C'est négligeable pour
// une connexion, mais ça rend très lent le test de millions de mots de passe
// si la base fuite un jour.
const bcryptCost = 12

const (
	minPasswordLength = 10
	// bcrypt ignore tout ce qui dépasse 72 octets sans prévenir : on refuse
	// plutôt que de laisser croire qu'un mot de passe plus long est plus sûr
	maxPasswordBytes = 72
	maxEmailLength   = 254
)

var ErrInvalidCredentials = errors.New("email ou mot de passe incorrect")

// dummyHash sert à faire un calcul bcrypt même quand l'email n'existe pas,
// pour que le temps de réponse ne révèle pas quels comptes existent.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("pumpcast-dummy-password"), bcryptCost)

// NormalizeEmail met l'email en minuscules et vérifie sa forme.
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > maxEmailLength {
		return "", errors.New("email invalide")
	}
	addr, err := mail.ParseAddress(email)
	// ParseAddress accepte aussi "Nom <a@b.fr>" : on veut juste l'adresse
	if err != nil || addr.Address != email || !strings.Contains(email[strings.Index(email, "@"):], ".") {
		return "", errors.New("email invalide")
	}
	return email, nil
}

// ValidatePassword vérifie la longueur du mot de passe.
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < minPasswordLength {
		return fmt.Errorf("le mot de passe doit faire au moins %d caractères", minPasswordLength)
	}
	if len(password) > maxPasswordBytes {
		return fmt.Errorf("le mot de passe est trop long (%d octets maximum)", maxPasswordBytes)
	}
	return nil
}

// HashPassword renvoie le hash bcrypt (il contient son propre sel).
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hachage du mot de passe : %w", err)
	}
	return string(hash), nil
}

// CheckPassword compare un mot de passe à son hash. hash vide = utilisateur
// inconnu : on fait quand même un calcul pour garder le même temps de réponse.
func CheckPassword(hash, password string) error {
	if hash == "" {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return ErrInvalidCredentials
	}
	return nil
}
