import numpy as np
import pandas as pd
import pytest

from pumpcast_ml import evaluate
from pumpcast_ml.recommend import (
    HorizonPrediction as HP,
    Settings,
    confidence_drop,
    confidence_rise,
    confidence_stable,
    reasons,
    recommend,
)

ALL = {1, 2, 3}


def test_confidence_from_interval() -> None:
    # Fourchette entièrement sous zéro : le modèle est sûr de la baisse
    assert confidence_drop(HP(1, -3, -5, -1)) == 1.0
    # Fourchette centrée sur zéro : une chance sur deux
    assert confidence_drop(HP(1, 0, -2, 2)) == pytest.approx(0.5)
    assert confidence_rise(HP(1, 2, -1, 3)) == pytest.approx(0.75)
    # Fourchette [-1, 3], part entre -2 et +2 : 3/4
    assert confidence_stable(HP(1, 1, -1, 3), 2.0) == pytest.approx(0.75)


@pytest.mark.parametrize(
    "preds, expected",
    [
        # Baisse de 3 c sûre à J+2 : attendre
        ([HP(1, -0.5, -1, 0.5), HP(2, -3, -5, -1), HP(3, -2.5, -4, 0)], ("wait", 2)),
        # Hausse sûre à J+3 : faire le plein
        ([HP(1, 0.5, 0, 1), HP(2, 1.5, 0, 3), HP(3, 3, 1, 5)], ("fill_now", 3)),
        # Baisse prévue mais pas sûre (fourchette à cheval sur zéro) : stable
        ([HP(1, 0, -1, 1), HP(2, -2.5, -6, 4), HP(3, -2, -6, 4)], ("stable", 3)),
        # Rien ne bouge
        ([HP(1, 0, -0.5, 0.5), HP(2, 0.2, -1, 1), HP(3, 0.1, -1, 1)], ("stable", 3)),
    ],
)
def test_recommend(preds: list[HP], expected: tuple[str, int]) -> None:
    rec = recommend(preds, ALL)
    assert rec is not None
    assert (rec.kind, rec.horizon) == expected
    assert 0 <= rec.confidence <= 1


def test_no_recommendation_without_a_horizon_that_beats_the_baseline() -> None:
    preds = [HP(1, -3, -5, -1), HP(2, -3, -5, -1), HP(3, -3, -5, -1)]
    assert recommend(preds, allowed_horizons=set()) is None
    # Seul J+5 bat la baseline, mais la recommandation regarde 3 jours
    assert recommend(preds, allowed_horizons={5}) is None
    # Si seul J+1 est autorisé, la baisse de J+2 n'est pas utilisée
    rec = recommend([HP(1, 0, -1, 1), HP(2, -3, -5, -1)], allowed_horizons={1})
    assert rec is not None and rec.kind == "stable" and rec.horizon == 1


def test_thresholds_are_settings() -> None:
    preds = [HP(1, -1.5, -2, -1)]
    assert recommend(preds, {1}).kind == "stable"
    assert recommend(preds, {1}, Settings(move_cents=1.0)).kind == "wait"


def test_reasons() -> None:
    out = reasons({"nat_trend7": 3.2, "dep_gap": 6.0, "brent_d14": -4.0, "days_since_change": 20})
    assert out[0].startswith("Les prix montent dans toute la France")
    assert "3,2 c" in out[0]
    assert "au-dessus de la moyenne du département" in out[1]
    assert "baissé de 4,0 c" in out[2]
    assert "20 jours" in out[3]
    assert reasons({"nat_trend7": 0.5, "dep_gap": 1.0}) == [
        "Pas de signal fort : le pétrole et les prix autour sont stables en ce moment."
    ]


def test_metrics() -> None:
    y = np.array([1.0, -2.0, 0.0, 3.0])
    assert evaluate.mae(y, np.zeros(4)) == pytest.approx(1.5)
    assert evaluate.coverage(y, np.full(4, -1.0), np.full(4, 2.0)) == pytest.approx(0.5)


def test_evaluate_recommendations() -> None:
    row_wait = {"pred1": -3, "low1": -5, "high1": -1, "y1": -4, "pred2": 0, "low2": -1, "high2": 1, "y2": -4,
                "pred3": 0, "low3": -1, "high3": 1, "y3": -4}
    row_fill = {"pred1": 0, "low1": -1, "high1": 1, "y1": 0, "pred2": 1, "low2": 0, "high2": 2, "y2": 1,
                "pred3": 3, "low3": 1, "high3": 5, "y3": -1}
    row_stable = {"pred1": 0, "low1": -1, "high1": 1, "y1": 0, "pred2": 0, "low2": -1, "high2": 1, "y2": 0,
                  "pred3": 0, "low3": -1, "high3": 1, "y3": 0.5}
    res = evaluate.evaluate_recommendations(pd.DataFrame([row_wait, row_fill, row_stable]), ALL)
    assert res["counts"] == {"wait": 1, "fill_now": 1, "stable": 1, "none": 0}
    # Attendre a fait gagner 4 c, faire le plein a fait perdre 1 c (le prix a baissé)
    assert res["avg_gain_cents"] == {"wait": 4.0, "fill_now": -1.0}
    assert res["accuracy_by_kind"] == {"wait": 1.0, "fill_now": 0.0, "stable": 1.0}
    assert res["accuracy"] == pytest.approx(2 / 3)


def test_evaluate_with_disabled_kinds() -> None:
    row_wait = {"pred1": -3, "low1": -5, "high1": -1, "y1": -4, "pred2": 0, "low2": -1, "high2": 1, "y2": -4,
                "pred3": 0, "low3": -1, "high3": 1, "y3": -4}
    res = evaluate.evaluate_recommendations(pd.DataFrame([row_wait]), ALL, enabled_kinds={"stable"})
    # "Attends" désactivé : pas de conseil plutôt qu'un autre conseil
    assert res["counts"] == {"wait": 0, "fill_now": 0, "stable": 0, "none": 1}
    assert res["accuracy"] is None


def stats(wait=(0, None, None), fill_now=(0, None, None), stable=(0, None)) -> dict:
    """Résultat de evaluate_recommendations écrit à la main : (nombre, taux, gain)."""
    return {
        "counts": {"wait": wait[0], "fill_now": fill_now[0], "stable": stable[0], "none": 0},
        "accuracy_by_kind": {"wait": wait[1], "fill_now": fill_now[1], "stable": stable[1]},
        "avg_gain_cents": {"wait": wait[2], "fill_now": fill_now[2]},
    }


def test_select_kinds() -> None:
    val = stats(wait=(108, 0.74, 4.5), fill_now=(40, 0.85, 3.9), stable=(199_852, 0.86))
    test = stats(wait=(50, 0.66, 2.1), fill_now=(25, 0.40, 1.0), stable=(150_000, 0.70))
    d = evaluate.select_kinds(val, test)
    assert d["wait"] == {"validation": True, "test": True, "enabled": True}
    # Bon sur la validation mais pas confirmé sur le test (40 % de bons conseils)
    assert d["fill_now"] == {"validation": True, "test": False, "enabled": False}
    assert d["stable"]["enabled"] is True


@pytest.mark.parametrize(
    "val_wait, reason",
    [
        ((20, 0.9, 5.0), "trop peu de cas pour juger"),
        ((100, 0.55, 2.0), "pas assez souvent juste"),
        ((100, 0.7, -0.5), "fait perdre de l'argent en moyenne"),
    ],
)
def test_select_kinds_rejects(val_wait: tuple, reason: str) -> None:
    test = stats(wait=(100, 0.9, 5.0))
    assert evaluate.select_kinds(stats(wait=val_wait), test)["wait"]["enabled"] is False, reason


def test_select_kinds_needs_enough_test_cases() -> None:
    val = stats(wait=(100, 0.8, 3.0))
    assert evaluate.select_kinds(val, stats(wait=(5, 1.0, 6.0)))["wait"]["enabled"] is False

