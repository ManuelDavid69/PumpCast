"""Mesures de qualité du modèle, comparées à la baseline "le prix ne bouge pas".

MAE (erreur absolue moyenne) : la moyenne des écarts entre la prédiction et
la réalité, en centimes. La baseline prédit toujours une variation de 0,
son erreur est donc simplement la variation réelle.
"""

import numpy as np
import pandas as pd

from pumpcast_ml.recommend import HorizonPrediction, Settings, recommend


def mae(y_true: np.ndarray, y_pred: np.ndarray) -> float:
    return float(np.mean(np.abs(y_true - y_pred)))


def coverage(y_true: np.ndarray, low: np.ndarray, high: np.ndarray) -> float:
    """Part des cas où la vraie valeur tombe dans la fourchette. Visé : environ 80 %
    pour une fourchette entre les quantiles 10 % et 90 %."""
    return float(np.mean((y_true >= low) & (y_true <= high)))


def evaluate_recommendations(
    frame: pd.DataFrame, allowed_horizons: set[int], settings: Settings = Settings()
) -> dict:
    """Rejoue les recommandations sur la période de test.

    frame contient, pour chaque station et jour : y1..y3 (variations réelles)
    et pred{h}, low{h}, high{h} pour h = 1..3.

    Pour chaque type de conseil on regarde si le suivre a fait gagner de l'argent :
      - "attends" : le prix le jour visé était-il plus bas qu'aujourd'hui ?
      - "fais le plein" : le prix dans 3 jours était-il plus haut qu'aujourd'hui ?
      - "stable" : le prix a-t-il bougé de moins de 2 centimes en 3 jours ?
    Le gain est en centimes par litre par rapport à l'autre choix (attendre ou non).
    """
    w = settings.window_days
    counts = {"wait": 0, "fill_now": 0, "stable": 0, "none": 0}
    correct = {"wait": 0, "fill_now": 0, "stable": 0}
    gains = {"wait": [], "fill_now": []}

    cols = {h: (f"pred{h}", f"low{h}", f"high{h}", f"y{h}") for h in range(1, w + 1)}
    values = {h: frame[list(c)].to_numpy() for h, c in cols.items()}
    for i in range(len(frame)):
        preds = [HorizonPrediction(h, *values[h][i, :3]) for h in range(1, w + 1)]
        rec = recommend(preds, allowed_horizons, settings)
        if rec is None:
            counts["none"] += 1
            continue
        counts[rec.kind] += 1
        if rec.kind == "wait":
            actual = values[rec.horizon][i, 3]
            gains["wait"].append(-actual)
            correct["wait"] += actual < 0
        elif rec.kind == "fill_now":
            actual = values[w][i, 3]
            gains["fill_now"].append(actual)
            correct["fill_now"] += actual > 0
        else:
            correct["stable"] += abs(values[w][i, 3]) < settings.move_cents

    given = counts["wait"] + counts["fill_now"] + counts["stable"]
    return {
        "counts": counts,
        "accuracy": (sum(correct.values()) / given) if given else None,
        "accuracy_by_kind": {k: (correct[k] / counts[k]) if counts[k] else None for k in correct},
        # Gain moyen en centimes par litre quand on suit un conseil "agir"
        "avg_gain_cents": {k: float(np.mean(v)) if v else None for k, v in gains.items()},
    }
