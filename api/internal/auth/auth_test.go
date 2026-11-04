package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const secret = "un-secret-de-test-assez-long-pour-hs256"

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"  Manu.David@Example.FR ", "manu.david@example.fr", true},
		{"a@b.fr", "a@b.fr", true},
		{"pas-un-email", "", false},
		{"a@b", "", false}, // pas de point dans le domaine
		{"Manu <manu@example.fr>", "", false},
		{"", "", false},
		{strings.Repeat("a", 250) + "@b.fr", "", false},
	}
	for _, tt := range tests {
		got, err := NormalizeEmail(tt.in)
		if (err == nil) != tt.ok || got != tt.want {
			t.Errorf("NormalizeEmail(%q) = %q, %v", tt.in, got, err)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if ValidatePassword("court") == nil {
		t.Error("un mot de passe de 5 caractères doit être refusé")
	}
	if err := ValidatePassword("dix-caract"); err != nil {
		t.Errorf("10 caractères doivent passer : %v", err)
	}
	// 10 caractères accentués = plus de 10 octets mais bien 10 caractères
	if err := ValidatePassword("éééééééééé"); err != nil {
		t.Errorf("les accents comptent pour un caractère : %v", err)
	}
	// bcrypt ignorerait tout ce qui dépasse 72 octets
	if ValidatePassword(strings.Repeat("a", 73)) == nil {
		t.Error("73 octets doivent être refusés")
	}
}

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("mon-mot-de-passe")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, "mon-mot-de-passe") || !strings.HasPrefix(hash, "$2a$12$") {
		t.Errorf("hash inattendu : %s", hash)
	}
	if CheckPassword(hash, "mon-mot-de-passe") != nil {
		t.Error("le bon mot de passe doit passer")
	}
	if CheckPassword(hash, "Mon-mot-de-passe") != ErrInvalidCredentials {
		t.Error("un mauvais mot de passe doit être refusé")
	}
	// Utilisateur inconnu : refusé, avec la même erreur
	if CheckPassword("", "mon-mot-de-passe") != ErrInvalidCredentials {
		t.Error("un hash vide doit être refusé")
	}
	// Deux hashs du même mot de passe sont différents (sel aléatoire)
	other, _ := HashPassword("mon-mot-de-passe")
	if other == hash {
		t.Error("le sel doit rendre chaque hash unique")
	}
}

func TestTokens(t *testing.T) {
	if _, err := NewTokens("trop-court"); err == nil {
		t.Error("un secret de moins de 32 caractères doit être refusé")
	}
	tokens, err := NewTokens(secret)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	tokens.now = func() time.Time { return now }

	raw, err := tokens.Sign("6720a1b2c3d4e5f600000001")
	if err != nil {
		t.Fatal(err)
	}
	id, err := tokens.Parse(raw)
	if err != nil || id != "6720a1b2c3d4e5f600000001" {
		t.Fatalf("Parse = %q, %v", id, err)
	}

	// 8 jours plus tard le jeton a expiré
	tokens.now = func() time.Time { return now.Add(8 * 24 * time.Hour) }
	if _, err := tokens.Parse(raw); err == nil {
		t.Error("un jeton expiré doit être refusé")
	}
	tokens.now = func() time.Time { return now }

	// Signé avec un autre secret
	other, _ := NewTokens("un-autre-secret-tout-aussi-long-pour-hs256")
	other.now = tokens.now
	forged, _ := other.Sign("6720a1b2c3d4e5f600000001")
	if _, err := tokens.Parse(forged); err == nil {
		t.Error("un jeton signé avec un autre secret doit être refusé")
	}

	// Jeton modifié à la main (on change l'utilisateur sans re-signer)
	parts := strings.Split(raw, ".")
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	changed := strings.Replace(string(payload), "000001", "000002", 1)
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(changed))
	if _, err := tokens.Parse(strings.Join(parts, ".")); err == nil {
		t.Error("un jeton modifié doit être refusé")
	}
}

func TestTokensRejectOtherAlgorithms(t *testing.T) {
	tokens, _ := NewTokens(secret)
	claims := jwt.RegisteredClaims{
		Subject:   "6720a1b2c3d4e5f600000001",
		Issuer:    issuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	// L'attaque classique : un jeton "alg": "none", sans signature
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := tokens.Parse(none); err == nil {
		t.Error("un jeton sans signature (alg none) doit être refusé")
	}
	// Même secret mais un autre algorithme
	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString([]byte(secret))
	if _, err := tokens.Parse(hs512); err == nil {
		t.Error("seul HS256 doit être accepté")
	}
	// Jeton sans date d'expiration
	noExp, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: "x", Issuer: issuer}).SignedString([]byte(secret))
	if _, err := tokens.Parse(noExp); err == nil {
		t.Error("un jeton sans expiration doit être refusé")
	}
}
