package config

import (
	"slices"
	"testing"
)

func TestParseDepartments(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{name: "all", raw: "all", want: nil},
		{name: "vide", raw: "", want: nil},
		{name: "liste avec espaces", raw: "01, 69,38", want: []string{"01", "69", "38"}},
		{name: "minuscule", raw: "2a", want: []string{"2A"}},
		{name: "outre-mer", raw: "971", want: []string{"971"}},
		{name: "trop court", raw: "1", wantErr: true},
		{name: "que des virgules", raw: ",,", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDepartments(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseDepartments(%q) : erreur attendue", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDepartments(%q) : erreur inattendue : %v", tt.raw, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("parseDepartments(%q) = %v, attendu %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	// Variables vides : getEnv doit retomber sur les valeurs par défaut.
	t.Setenv("MONGO_URI", "")
	t.Setenv("MONGO_DB", "")
	t.Setenv("REDIS_URL", "")
	t.Setenv("API_PORT", "")
	t.Setenv("FEED_URL", "")
	t.Setenv("HISTORY_DEPARTMENTS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() : erreur inattendue : %v", err)
	}

	if cfg.MongoURI != "mongodb://localhost:27017" {
		t.Errorf("MongoURI = %q", cfg.MongoURI)
	}
	if cfg.MongoDB != "pumpcast" {
		t.Errorf("MongoDB = %q", cfg.MongoDB)
	}
	if cfg.RedisURL != "redis://localhost:6379/0" {
		t.Errorf("RedisURL = %q", cfg.RedisURL)
	}
	if cfg.APIPort != "8080" {
		t.Errorf("APIPort = %q", cfg.APIPort)
	}
	if cfg.FeedURL != "https://donnees.roulez-eco.fr/opendata/instantane" {
		t.Errorf("FeedURL = %q", cfg.FeedURL)
	}
	if cfg.HistoryDepartments != nil {
		t.Errorf("HistoryDepartments = %v, attendu nil", cfg.HistoryDepartments)
	}
}

func TestLoadValues(t *testing.T) {
	t.Setenv("MONGO_URI", "mongodb://mongo:27017")
	t.Setenv("MONGO_DB", "test")
	t.Setenv("REDIS_URL", "redis://redis:6379/1")
	t.Setenv("API_PORT", "9090")
	t.Setenv("FEED_URL", "https://example.com/feed")
	t.Setenv("HISTORY_DEPARTMENTS", "01,69")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() : erreur inattendue : %v", err)
	}

	if cfg.MongoURI != "mongodb://mongo:27017" {
		t.Errorf("MongoURI = %q", cfg.MongoURI)
	}
	if cfg.MongoDB != "test" {
		t.Errorf("MongoDB = %q", cfg.MongoDB)
	}
	if cfg.RedisURL != "redis://redis:6379/1" {
		t.Errorf("RedisURL = %q", cfg.RedisURL)
	}
	if cfg.APIPort != "9090" {
		t.Errorf("APIPort = %q", cfg.APIPort)
	}
	if cfg.FeedURL != "https://example.com/feed" {
		t.Errorf("FeedURL = %q", cfg.FeedURL)
	}
	if !slices.Equal(cfg.HistoryDepartments, []string{"01", "69"}) {
		t.Errorf("HistoryDepartments = %v", cfg.HistoryDepartments)
	}
}

func TestKeepHistory(t *testing.T) {
	tests := []struct {
		name string
		deps []string
		dept string
		want bool
	}{
		{name: "nil garde tout", deps: nil, dept: "01", want: true},
		{name: "présent", deps: []string{"01", "69"}, dept: "69", want: true},
		{name: "absent", deps: []string{"01", "69"}, dept: "38", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{HistoryDepartments: tt.deps}
			if got := cfg.KeepHistory(tt.dept); got != tt.want {
				t.Errorf("KeepHistory(%q) = %v, attendu %v", tt.dept, got, tt.want)
			}
		})
	}
}
