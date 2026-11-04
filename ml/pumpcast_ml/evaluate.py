"""Mesures de qualité du modèle, comparées à la baseline "le prix ne bouge pas".

MAE (erreur absolue moyenne) : la moyenne des écarts entre la prédiction et
la réalité, en centimes. La baseline prédit toujours une variation de 0,
son erreur est donc simplement la variation réelle.

Les conseils sont jugés autrement : pas sur l'erreur moyenne, mais sur ce
qu'ils auraient fait gagner à quelqu'un qui les suit (voir select_kinds).
"""

from dataclasses import dataclass

import numpy as np
import pandas as pd

from pumpcast_ml.recommend import HorizonPrediction, Settings, recommend

KINDS = ("wait", "fill_now", "stable")


def mae(y_true: np.ndarray, y_pred: np.ndarray) -> float:
    return float(np.mean(np.abs(y_true - y_pred)))


def coverage(y_true: np.ndarray, low: np.ndarray, high: np.ndarray) -> float:
    """Part des cas où la vraie valeur tombe dans la fourchette. Visé : environ 80 %
    pour une fourchette entre les quantiles 10 % et 90 %."""
    return float(np.mean((y_true >= low) & (y_true <= high)))


def evaluate_recommendations(
    frame: pd.DataFrame,
    allowed_horizons: set[int],
    settings: Settings = Settings(),
    enabled_kinds: set[str] | None = None,
) -> dict:
    """Rejoue les recommandations sur une période passée.

    frame contient, pour chaque station et jour : y1..y3 (variations réelles)
    et pred{h}, low{h}, high{h} pour h = 1..3.

    Pour chaque type de conseil on regarde si le suivre a fait gagner de l'argent :
      - "attends" : le prix le jour visé était-il plus bas qu'aujourd'hui ?
      - "fais le plein" : le prix dans 3 jours était-il plus haut qu'aujourd'hui ?
      - "stable" : le prix a-t-il bougé de moins de 2 centimes en 3 jours ?
    Le gain est en centimes par litre par rapport à l'autre choix (attendre ou non).

    enabled_kinds : types de conseil activés. Un conseil d'un type désactivé
    n'est pas donné (compté dans "none"). None veut dire tous les types.
    """
    w = settings.window_days
    counts = {"wait": 0, "fill_now": 0, "stable": 0, "none": 0}
    correct = {"wait": 0, "fill_now": 0, "stable": 0}
    gains: dict[str, list[float]] = {"wait": [], "fill_now": []}

    cols = {h: (f"pred{h}", f"low{h}", f"high{h}", f"y{h}") for h in range(1, w + 1)}
    values = {h: frame[list(c)].to_numpy() for h, c in cols.items()}
    for i in range(len(frame)):
        preds = [HorizonPrediction(h, *values[h][i, :3]) for h in range(1, w + 1)]
        rec = recommend(preds, allowed_horizons, settings)
        if rec is None or (enabled_kinds is not None and rec.kind not in enabled_kinds):
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
        "accuracy": float(sum(correct.values()) / given) if given else None,
        "accuracy_by_kind": {k: float(correct[k] / counts[k]) if counts[k] else None for k in correct},
        # Gain moyen en centimes par litre quand on suit un conseil "agir"
        "avg_gain_cents": {k: float(np.mean(v)) if v else None for k, v in gains.items()},
    }


@dataclass(frozen=True)
class Criteria:
    """Quand un type de conseil a-t-il le droit d'être affiché ?

    Il doit d'abord faire ses preuves sur la validation (la période qui sert à
    faire les choix), puis le confirmer sur le test (des jours jamais utilisés
    pour décider). Les seuils du test sont plus bas car on vérifie juste que le
    conseil ne s'effondre pas, on ne cherche pas à le sélectionner une 2e fois.
    """

    min_count_validation: int = 30
    min_accuracy_validation: float = 0.6
    min_count_test: int = 10
    min_accuracy_test: float = 0.5


def _passes(stats: dict, kind: str, min_count: int, min_accuracy: float) -> bool:
    count = stats["counts"][kind]
    accuracy = stats["accuracy_by_kind"][kind]
    if count < min_count or accuracy is None or accuracy < min_accuracy:
        return False
    # "Attends" et "fais le plein" doivent en plus faire gagner de l'argent en moyenne
    if kind in stats["avg_gain_cents"]:
        gain = stats["avg_gain_cents"][kind]
        return gain is not None and gain > 0
    return True


def select_kinds(validation: dict, test: dict, criteria: Criteria = Criteria()) -> dict[str, dict]:
    """Décide, type par type, si le conseil est activé.

    validation et test sont des résultats de evaluate_recommendations calculés
    avec tous les types de conseil. Renvoie pour chaque type le verdict sur la
    validation, sur le test, et la décision finale (les deux doivent passer).
    """
    out = {}
    for kind in KINDS:
        on_val = _passes(validation, kind, criteria.min_count_validation, criteria.min_accuracy_validation)
        on_test = _passes(test, kind, criteria.min_count_test, criteria.min_accuracy_test)
        out[kind] = {"validation": on_val, "test": on_test, "enabled": on_val and on_test}
    return out
