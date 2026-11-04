// Package config lit la configuration depuis les variables d'environnement.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config regroupe tous les réglages de l'API et de l'ingestion.
type Config struct {
	MongoURI string
	MongoDB  string
	RedisURL string
	APIPort  string
	FeedURL  string
	// Active la page /playground pour tester l'API à la main
	EnablePlayground bool
	// Nombre de requêtes GraphQL autorisées par IP et par minute
	RateLimitPerMinute int
	// L'API est derrière un proxy (Render) qui ajoute X-Forwarded-For
	TrustProxy bool
	// Origines autorisées à appeler l'API depuis un navigateur (le front)
	CORSOrigins []string
	// Départements dont on garde l'historique des prix.
	// nil veut dire toute la France.
	HistoryDepartments []string
}

// Load lit les variables d'environnement. Les valeurs par défaut
// correspondent au docker compose local, comme ça un simple `go run` marche.
func Load() (Config, error) {
	cfg := Config{
		MongoURI: getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:  getEnv("MONGO_DB", "pumpcast"),
		RedisURL: getEnv("REDIS_URL", "redis://localhost:6379/0"),
		APIPort:  getEnv("API_PORT", "8080"),
		FeedURL:  getEnv("FEED_URL", "https://donnees.roulez-eco.fr/opendata/instantane"),
	}

	cfg.EnablePlayground = getEnv("ENABLE_PLAYGROUND", "true") == "true"
	cfg.TrustProxy = getEnv("TRUST_PROXY", "false") == "true"
	for _, o := range strings.Split(getEnv("CORS_ORIGINS", "http://localhost:3000"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, o)
		}
	}

	limit, err := strconv.Atoi(getEnv("RATE_LIMIT_PER_MINUTE", "120"))
	if err != nil || limit < 1 {
		return Config{}, fmt.Errorf("RATE_LIMIT_PER_MINUTE invalide : %q", getEnv("RATE_LIMIT_PER_MINUTE", ""))
	}
	cfg.RateLimitPerMinute = limit

	deps, err := parseDepartments(getEnv("HISTORY_DEPARTMENTS", "all"))
	if err != nil {
		return Config{}, err
	}
	cfg.HistoryDepartments = deps

	return cfg, nil
}

// KeepHistory dit si on stocke l'historique des prix pour ce département.
func (c Config) KeepHistory(department string) bool {
	if c.HistoryDepartments == nil {
		return true
	}
	for _, d := range c.HistoryDepartments {
		if d == department {
			return true
		}
	}
	return false
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// parseDepartments transforme "01, 69,38" en ["01", "69", "38"].
// "all" (ou vide) renvoie nil pour dire "pas de filtre".
func parseDepartments(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "all") {
		return nil, nil
	}

	var deps []string
	for _, part := range strings.Split(raw, ",") {
		d := strings.ToUpper(strings.TrimSpace(part))
		if d == "" {
			continue
		}
		// 2 caractères en métropole (01, 2A...), 3 pour l'outre-mer (971...)
		if len(d) < 2 || len(d) > 3 {
			return nil, fmt.Errorf("HISTORY_DEPARTMENTS : département invalide %q", d)
		}
		deps = append(deps, d)
	}
	if len(deps) == 0 {
		return nil, fmt.Errorf("HISTORY_DEPARTMENTS ne contient aucun département")
	}
	return deps, nil
}
