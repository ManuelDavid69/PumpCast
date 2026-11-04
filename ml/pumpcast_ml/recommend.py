"""Règles de recommandation et explications.

Le modèle donne pour chaque horizon une variation prévue et une fourchette
(quantiles 10 % et 90 %). Ce fichier transforme ça en une phrase simple :
"Fais le plein maintenant", "Attends, baisse probable" ou "Prix stable".
"""

from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    # Variation minimale (en centimes) pour conseiller d'agir
    move_cents: float = 2.0
    # Confiance minimale pour dire "attends" ou "fais le plein"
    min_confidence: float = 0.7
    # La recommandation regarde les 3 prochains jours
    window_days: int = 3


@dataclass(frozen=True)
class HorizonPrediction:
    horizon: int
    delta: float  # variation prévue en centimes
    low: float  # quantile 10 %
    high: float  # quantile 90 %


@dataclass(frozen=True)
class Recommendation:
    kind: str  # "fill_now", "wait" ou "stable"
    confidence: float
    horizon: int  # jour visé (le jour de la baisse ou de la hausse)


def _share_below(value: float, low: float, high: float) -> float:
    """Part de la fourchette [low, high] située sous value, entre 0 et 1.

    C'est une estimation simple de la probabilité que la vraie variation soit
    sous value, en supposant la fourchette uniforme. Elle suffit pour dire
    "le modèle est sûr" (toute la fourchette du même côté) ou pas.
    """
    if high - low < 1e-9:
        return 1.0 if high < value else 0.0
    return min(max((value - low) / (high - low), 0.0), 1.0)


def confidence_drop(p: HorizonPrediction) -> float:
    return _share_below(0.0, p.low, p.high)


def confidence_rise(p: HorizonPrediction) -> float:
    return 1.0 - _share_below(0.0, p.low, p.high)


def confidence_stable(p: HorizonPrediction, move_cents: float) -> float:
    """Part de la fourchette qui reste entre -move et +move centimes."""
    return _share_below(move_cents, p.low, p.high) - _share_below(-move_cents, p.low, p.high)


def recommend(
    predictions: list[HorizonPrediction], allowed_horizons: set[int], settings: Settings = Settings()
) -> Recommendation | None:
    """Recommandation à partir des horizons autorisés (ceux qui battent la baseline).

    Renvoie None si aucun horizon de la fenêtre n'est autorisé : dans ce cas
    l'appli ne donne pas de conseil plutôt qu'un conseil non fiable.
    """
    usable = [p for p in predictions if p.horizon <= settings.window_days and p.horizon in allowed_horizons]
    if not usable:
        return None

    drop = min(usable, key=lambda p: p.delta)
    rise = max(usable, key=lambda p: p.delta)
    candidates = []
    if drop.delta <= -settings.move_cents and confidence_drop(drop) >= settings.min_confidence:
        candidates.append(Recommendation("wait", round(confidence_drop(drop), 2), drop.horizon))
    if rise.delta >= settings.move_cents and confidence_rise(rise) >= settings.min_confidence:
        candidates.append(Recommendation("fill_now", round(confidence_rise(rise), 2), rise.horizon))
    if candidates:
        # Hausse et baisse à la fois (rare) : on garde la plus sûre
        return max(candidates, key=lambda r: r.confidence)

    last = max(usable, key=lambda p: p.horizon)
    return Recommendation("stable", round(confidence_stable(last, settings.move_cents), 2), last.horizon)


def _cents(value: float) -> str:
    return f"{abs(value):.1f}".replace(".", ",")


def reasons(features: dict[str, float]) -> list[str]:
    """Explications simples, tirées des variables les plus importantes du modèle.

    L'ordre suit l'importance mesurée (permutation importance, voir train.py) :
    tendance nationale, écart au département, pétrole, puis ancienneté du prix.
    """
    out: list[str] = []

    nat = features.get("nat_trend7")
    if nat is not None and abs(nat) >= 2:
        sens = "montent" if nat > 0 else "baissent"
        out.append(f"Les prix {sens} dans toute la France depuis une semaine ({'+' if nat > 0 else '-'}{_cents(nat)} c en moyenne).")

    gap = features.get("dep_gap")
    if gap is not None and abs(gap) >= 5:
        if gap > 0:
            out.append(f"Cette station est {_cents(gap)} c au-dessus de la moyenne du département, ce genre d'écart a tendance à se réduire.")
        else:
            out.append(f"Cette station est {_cents(gap)} c en dessous de la moyenne du département, elle remonte souvent vers la moyenne.")

    brent = features.get("brent_d14")
    if brent is not None and abs(brent) >= 3:
        sens = "augmenté" if brent > 0 else "baissé"
        out.append(
            f"Le pétrole brut a {sens} de {_cents(brent)} c par litre en deux semaines. "
            "La pompe suit en général avec 5 à 14 jours de retard."
        )

    days = features.get("days_since_change")
    if days is not None and days >= 14:
        out.append(f"Le prix de cette station n'a pas bougé depuis {int(days)} jours.")

    if not out:
        out.append("Pas de signal fort : le pétrole et les prix autour sont stables en ce moment.")
    return out
