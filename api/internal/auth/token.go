package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Un jeton est valable 7 jours, ensuite il faut se reconnecter
const TokenLifetime = 7 * 24 * time.Hour

const issuer = "pumpcast"

// Tokens signe et vérifie les JWT. Un JWT contient l'identifiant de
// l'utilisateur et une date d'expiration, signés avec un secret que seul le
// serveur connaît : on peut vérifier qu'il n'a pas été modifié sans base de données.
type Tokens struct {
	secret []byte
	now    func() time.Time
}

func NewTokens(secret string) (*Tokens, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET doit faire au moins 32 caractères")
	}
	return &Tokens{secret: []byte(secret), now: time.Now}, nil
}

// RandomSecret génère un secret pour le développement local.
func RandomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Sign crée un jeton pour cet utilisateur.
func (t *Tokens) Sign(userID string) (string, error) {
	now := t.now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(TokenLifetime)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", fmt.Errorf("signature du jeton : %w", err)
	}
	return token, nil
}

// Parse vérifie le jeton et renvoie l'identifiant de l'utilisateur.
func (t *Tokens) Parse(raw string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) { return t.secret, nil },
		// Seul HS256 est accepté : sinon un jeton forgé avec "alg": "none"
		// ou un autre algorithme pourrait passer
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil {
		return "", errors.New("jeton invalide ou expiré")
	}
	if claims.Subject == "" {
		return "", errors.New("jeton sans utilisateur")
	}
	return claims.Subject, nil
}
