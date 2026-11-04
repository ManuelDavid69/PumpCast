package graph

import (
	"testing"
	"time"

	"github.com/ManuelDavid69/pumpcast/api/graph/model"
	"github.com/ManuelDavid69/pumpcast/api/internal/predictions"
)

func TestToPredictionWithoutRecommendation(t *testing.T) {
	// Le script Python écrit recommendation: null quand le modèle n'est pas fiable
	p := &predictions.Prediction{
		BaseDate:  time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC),
		BasePrice: 2.143,
		Horizon:   []predictions.Point{{Date: time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC), Price: 2.15, Low: 2.10, High: 2.20}},
	}
	out, err := toPrediction(p)
	if err != nil {
		t.Fatal(err)
	}
	if out.Recommendation != nil {
		t.Errorf("recommandation = %v, attendu nil", *out.Recommendation)
	}
	if out.BaseDate != "2026-11-04" || out.BasePrice != 2.143 || out.Horizon[0].Date != "2026-11-08" {
		t.Errorf("prédiction mal convertie : %+v", out)
	}
	if out.Reasons == nil {
		t.Error("reasons doit être une liste vide, pas nil (champ non nul dans le schéma)")
	}
}

func TestToPredictionRecommendation(t *testing.T) {
	wait, bad := "wait", "attendre"
	out, err := toPrediction(&predictions.Prediction{Recommendation: &wait})
	if err != nil || out.Recommendation == nil || *out.Recommendation != model.RecommendationWait {
		t.Fatalf("wait mal converti : %v, %v", out, err)
	}
	if _, err := toPrediction(&predictions.Prediction{Recommendation: &bad}); err == nil {
		t.Error("une valeur inconnue en base doit donner une erreur")
	}
}

func TestToModelRun(t *testing.T) {
	acc := 0.61
	var run predictions.ModelRun
	run.ModelVersion = "2026-11-05.0900"
	run.TestPeriod = predictions.Period{Start: time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)}
	run.Metrics.Horizons = []predictions.HorizonScore{{
		Horizon: 3, MAEBaseline: 1.5, MAEModel: 1.4, BeatsBaseline: true,
		ByPeriod: map[string]predictions.PeriodScore{
			"mars_oct2026":    {MAEBaseline: 2, MAEModel: 1.9},
			"nov2025_fev2026": {MAEBaseline: 1, MAEModel: 1},
		},
	}}
	run.Metrics.Recommendations = predictions.RecommendationStats{
		Accuracy: &acc,
		Counts:   map[string]int{"wait": 3, "fill_now": 2, "stable": 10, "none": 1},
	}

	out := toModelRun(run)
	if out.TestStart != "2025-11-01" || out.TestEnd != "2026-11-03" {
		t.Errorf("période de test = %s / %s", out.TestStart, out.TestEnd)
	}
	// Les périodes sortent toujours dans le même ordre, avec un nom lisible
	p := out.Horizons[0].Periods
	if len(p) != 2 || p[0].Name != "Novembre 2025 à février 2026" || p[1].MaeModel != 1.9 {
		t.Errorf("périodes = %+v", p)
	}
	r := out.Recommendations
	if r.WaitCount != 3 || r.FillNowCount != 2 || r.StableCount != 10 || r.NoneCount != 1 || *r.Accuracy != 0.61 {
		t.Errorf("recommandations = %+v", r)
	}
	if out.AllowedHorizons == nil || out.FeatureImportance == nil {
		t.Error("les listes doivent être vides et pas nil")
	}
}

func TestToModelRunPolicy(t *testing.T) {
	acc, gain := 0.74, 4.5
	var run predictions.ModelRun
	run.Policy = predictions.RecommendationPolicy{
		EnabledKinds: []string{"wait"},
		Decision: map[string]predictions.KindDecision{
			"wait":     {Validation: true, Test: true, Enabled: true},
			"fill_now": {Validation: true, Test: false, Enabled: false},
		},
		Criteria: predictions.Criteria{MinCountValidation: 30, MinAccuracyValidation: 0.6, MinCountTest: 10, MinAccuracyTest: 0.5},
	}
	run.Metrics.RecommendationsValidation = predictions.RecommendationStats{
		Counts:         map[string]int{"wait": 108},
		AccuracyByKind: map[string]*float64{"wait": &acc},
		AvgGainCents:   map[string]*float64{"wait": &gain},
	}

	out := toModelRun(run)
	if len(out.EnabledRecommendations) != 1 || out.EnabledRecommendations[0] != model.RecommendationWait {
		t.Fatalf("conseils activés = %v, attendu seulement WAIT", out.EnabledRecommendations)
	}
	// Un type de conseil par ligne, toujours dans le même ordre
	if len(out.RecommendationRules) != 3 || out.RecommendationRules[1].Kind != model.RecommendationFillNow {
		t.Fatalf("règles = %+v", out.RecommendationRules)
	}
	w := out.RecommendationRules[0]
	if !w.Enabled || w.ValidationCount != 108 || *w.ValidationAccuracy != 0.74 || *w.ValidationGainCents != 4.5 {
		t.Errorf("règle attends = %+v", w)
	}
	f := out.RecommendationRules[1]
	if f.Enabled || !f.PassesValidation || f.PassesTest {
		t.Errorf("fais le plein devrait être refusé par le test : %+v", f)
	}
	if out.RecommendationCriteria.MinAccuracyValidation != 0.6 || out.RecommendationCriteria.MinCountTest != 10 {
		t.Errorf("critères = %+v", out.RecommendationCriteria)
	}
}
