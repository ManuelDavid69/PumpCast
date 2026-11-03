package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type body struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func TestHandlerAllOK(t *testing.T) {
	checks := map[string]Check{
		"mongo": func(context.Context) error { return nil },
		"redis": func(context.Context) error { return nil },
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	Handler(checks)(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, attendu %d", rec.Code, http.StatusOK)
	}

	var res body
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("décodage JSON : %v", err)
	}
	if res.Status != "ok" {
		t.Errorf("status = %q, attendu %q", res.Status, "ok")
	}
	if res.Checks["mongo"] != "ok" || res.Checks["redis"] != "ok" {
		t.Errorf("checks = %v, attendu tout ok", res.Checks)
	}
}

func TestHandlerDegraded(t *testing.T) {
	checks := map[string]Check{
		"mongo": func(context.Context) error { return nil },
		"redis": func(context.Context) error { return errors.New("connexion refusée") },
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	Handler(checks)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("code = %d, attendu %d", rec.Code, http.StatusServiceUnavailable)
	}

	var res body
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("décodage JSON : %v", err)
	}
	if res.Status != "degraded" {
		t.Errorf("status = %q, attendu %q", res.Status, "degraded")
	}
	if res.Checks["redis"] != "down" {
		t.Errorf("checks[redis] = %q, attendu %q", res.Checks["redis"], "down")
	}
	if res.Checks["mongo"] != "ok" {
		t.Errorf("checks[mongo] = %q, attendu %q", res.Checks["mongo"], "ok")
	}
}
