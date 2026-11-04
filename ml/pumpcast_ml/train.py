"""Entraînement et évaluation du modèle, une fois par semaine.

Usage : python -m pumpcast_ml.train [--sample 400000] [--skip-test] [--no-save]

Déroulé :
1. Construction des variables pour tous les carburants (features.py)
2. Découpage dans le temps, jamais au hasard (le modèle verrait le futur) :
     - validation (mai à octobre 2025) : sert à faire les choix et à calibrer
       les fourchettes. C'est la seule période regardée pendant la mise au point.
     - test (novembre 2025 à aujourd'hui) : le constat final, publié tel quel
       sur la page du modèle. On ne règle rien en le regardant.
3. Pour chaque horizon h (J+1 à J+7) : baseline, Ridge, HistGradientBoosting,
   et deux modèles quantiles (10 % et 90 %) pour la fourchette.
4. Règle d'honnêteté : un horizon qui ne bat pas la baseline sur le test
   n'est pas utilisé par l'appli.
5. Réentraînement sur toutes les données pour les prédictions de la semaine.
"""

import argparse
import time
from datetime import datetime, timezone
from pathlib import Path

import joblib
import numpy as np
import pandas as pd
from sklearn.compose import ColumnTransformer
from sklearn.ensemble import HistGradientBoostingRegressor
from sklearn.impute import SimpleImputer
from sklearn.inspection import permutation_importance
from sklearn.linear_model import Ridge
from sklearn.pipeline import make_pipeline
from sklearn.preprocessing import OneHotEncoder, StandardScaler

from pumpcast_ml import data, evaluate, features
from pumpcast_ml.db import get_db
from pumpcast_ml.recommend import Settings

FUELS = ["GAZOLE", "E10", "SP98", "SP95", "E85", "GPLC"]
MODELS_DIR = Path(__file__).resolve().parent.parent / "models"

# Relevés aberrants (fautes de saisie des stations) écartés partout, à
# l'entraînement comme à l'évaluation, avec la même règle
MAX_DEP_GAP_CENTS = 40.0
MAX_MOVE_CENTS = 40.0

# Réglages du gradient boosting, choisis simples et fixes : on ne les règle
# pas sur la période de test, ce serait tricher
HGB_PARAMS = dict(max_iter=250, learning_rate=0.08, max_leaf_nodes=31, min_samples_leaf=200, l2_regularization=0.1)

# Les deux morceaux de la période de test, très différents : un hiver calme
# puis une année 2026 agitée (Brent au-dessus de 120 $)
TEST_PERIODS = {"nov2025_fev2026": ("2025-11-01", "2026-02-28"), "mars_oct2026": ("2026-03-01", "2100-01-01")}


def load_dataset(db) -> pd.DataFrame:
    stations = data.load_stations(db)
    market = data.load_market(db)
    calendar = data.load_calendar(db)
    frames = []
    for fuel in FUELS:
        inp = features.Inputs(
            prices=data.load_prices(db, fuel),
            stations=stations,
            department_daily=data.load_department_daily(db, fuel),
            market=market,
            calendar=calendar,
        )
        frames.append(features.build_fuel_frame(fuel, inp))
        print(f"  {fuel} : {len(frames[-1]):,} lignes")
    df = pd.concat(frames, ignore_index=True)
    departments = sorted(df.department.dropna().unique())
    return features.as_categories(df, FUELS, departments)


def clean(df: pd.DataFrame) -> tuple[pd.DataFrame, dict]:
    """Retire les prix aberrants et les cibles impossibles (même règle partout)."""
    before = len(df)
    df = df[df.dep_gap.abs() <= MAX_DEP_GAP_CENTS].copy()
    removed_targets = 0
    for h in features.HORIZONS:
        col = features.target_name(h)
        bad = df[col].abs() > MAX_MOVE_CENTS
        removed_targets += int(bad.sum())
        df.loc[bad, col] = np.nan
    return df, {"rows_removed": before - len(df), "targets_removed": removed_targets}


def make_ridge():
    numeric = make_pipeline(SimpleImputer(strategy="median"), StandardScaler())
    pre = ColumnTransformer(
        [
            ("num", numeric, features.NUMERIC_FEATURES),
            ("cat", OneHotEncoder(handle_unknown="ignore"), features.CATEGORICAL_FEATURES),
        ]
    )
    return make_pipeline(pre, Ridge(alpha=1.0))


def make_hgb(quantile: float | None = None) -> HistGradientBoostingRegressor:
    if quantile is None:
        # absolute_error : le modèle vise la médiane, c'est ce qui minimise la MAE
        return HistGradientBoostingRegressor(loss="absolute_error", categorical_features="from_dtype", random_state=0, **HGB_PARAMS)
    return HistGradientBoostingRegressor(
        loss="quantile", quantile=quantile, categorical_features="from_dtype", random_state=0, **HGB_PARAMS
    )


def window(df: pd.DataFrame, y: str, start: pd.Timestamp | None, end: pd.Timestamp, h: int) -> pd.DataFrame:
    """Lignes dont la cible (le prix à t+h) tombe avant end : rien ne déborde
    sur la période suivante."""
    mask = (df.date < end - pd.Timedelta(days=h)) & df[y].notna()
    if start is not None:
        mask &= df.date >= start
    return df[mask]


def conformal_margin(y: np.ndarray, low: np.ndarray, high: np.ndarray, level: float = 0.8) -> float:
    """Calibration conformale des fourchettes (CQR).

    Sur des données que les modèles quantiles n'ont pas vues, on mesure de
    combien la vraie valeur sort de la fourchette, et on élargit la fourchette
    de ce qu'il faut pour en couvrir `level`. Si la fourchette était déjà trop
    large, la marge est négative et la resserre.
    """
    scores = np.maximum(low - y, y - high)
    q = min(1.0, level * (1 + 1 / len(scores)))
    return float(np.quantile(scores, q))


def sample(df: pd.DataFrame, n: int, seed: int) -> pd.DataFrame:
    # Tirer des lignes au hasard DANS la période d'entraînement ne fait pas
    # voir le futur : ça réduit juste le temps de calcul
    return df.sample(n=min(n, len(df)), random_state=seed)


def quantile_pair(q10, q90, X: pd.DataFrame) -> tuple[np.ndarray, np.ndarray]:
    # Appris séparément, les deux quantiles peuvent se croiser : on les remet dans l'ordre
    low, high = np.sort(np.vstack([q10.predict(X), q90.predict(X)]), axis=0)
    return low, high


def main(argv: list[str] | None = None) -> None:
    parser = argparse.ArgumentParser(description="Entraînement du modèle PumpCast")
    parser.add_argument("--sample", type=int, default=400_000, help="lignes d'entraînement par horizon")
    parser.add_argument("--val-start", type=pd.Timestamp, default=pd.Timestamp("2025-05-01"))
    parser.add_argument("--test-start", type=pd.Timestamp, default=pd.Timestamp("2025-11-01"))
    parser.add_argument("--no-save", action="store_true", help="évalue sans enregistrer (essais)")
    parser.add_argument("--skip-test", action="store_true", help="n'affiche que la validation (mise au point)")
    args = parser.parse_args(argv)
    t0 = time.time()

    db = get_db()
    print("Chargement et construction des variables...")
    df = load_dataset(db)
    df, cleaning = clean(df)
    print(f"{len(df):,} lignes après nettoyage ({cleaning})")

    val_start, test_start = args.val_start, args.test_start
    last_day = df.date.max()
    version = datetime.now(timezone.utc).strftime("%Y-%m-%d.%H%M")
    out_dir = MODELS_DIR / version
    settings = Settings()

    horizons_metrics = []
    test_preds: dict[int, pd.DataFrame] = {}
    val_preds: dict[int, pd.DataFrame] = {}
    importance = None

    for h in features.HORIZONS:
        y = features.target_name(h)

        # 1. Validation : apprentissage avant mai 2025, mesure sur mai-octobre 2025
        fit_val = sample(window(df, y, None, val_start, h), args.sample, seed=h)
        val = window(df, y, val_start, test_start, h)
        X_fv, y_fv = fit_val[features.FEATURES], fit_val[y]
        X_v, y_v = val[features.FEATURES], val[y].to_numpy()
        hgb_v = make_hgb().fit(X_fv, y_fv)
        q10, q90 = make_hgb(0.1).fit(X_fv, y_fv), make_hgb(0.9).fit(X_fv, y_fv)
        lo_v, hi_v = quantile_pair(q10, q90, X_v)
        margin = conformal_margin(y_v, lo_v, hi_v)
        val_metrics = {
            "mae_baseline": evaluate.mae(y_v, np.zeros_like(y_v)),
            "mae_model": evaluate.mae(y_v, hgb_v.predict(X_v)),
            "coverage_raw": evaluate.coverage(y_v, lo_v, hi_v),
            "conformal_margin": margin,
        }
        if h <= settings.window_days:
            val_preds[h] = pd.DataFrame(
                {f"pred{h}": hgb_v.predict(X_v), f"low{h}": lo_v - margin, f"high{h}": hi_v + margin, f"y{h}": y_v},
                index=val.index,
            )
        line = (
            f"J+{h} validation : baseline {val_metrics['mae_baseline']:.3f} c, boosting {val_metrics['mae_model']:.3f} c, "
            f"fourchette brute {val_metrics['coverage_raw']:.0%}, marge {margin:+.2f} c"
        )
        if args.skip_test:
            print(f"{line} ({time.time() - t0:.0f} s)")
            continue

        # 2. Test : apprentissage jusqu'à fin octobre 2025, constat sur la suite.
        # Les quantiles sont ceux de l'étape 1 (calibrés sur la validation qu'ils
        # n'ont pas vue), élargis de la marge conformale.
        fit = sample(window(df, y, None, test_start, h), args.sample, seed=10 + h)
        test = df[(df.date >= test_start) & df[y].notna()]
        X_tr, y_tr = fit[features.FEATURES], fit[y].to_numpy()
        X_te, y_te = test[features.FEATURES], test[y].to_numpy()
        ridge = make_ridge().fit(X_tr, y_tr)
        hgb = make_hgb().fit(X_tr, y_tr)
        p_hgb = hgb.predict(X_te)
        lo_raw, hi_raw = quantile_pair(q10, q90, X_te)
        low, high = lo_raw - margin, hi_raw + margin
        moved = np.abs(y_te) >= 1

        m = {
            "horizon": h,
            "mae_baseline": evaluate.mae(y_te, np.zeros_like(y_te)),
            "mae_ridge": evaluate.mae(y_te, ridge.predict(X_te)),
            "mae_model": evaluate.mae(y_te, p_hgb),
            "interval_coverage": evaluate.coverage(y_te, low, high),
            "interval_coverage_raw": evaluate.coverage(y_te, lo_raw, hi_raw),
            # Quand le prix bouge vraiment (1 c ou plus), le modèle annonce-t-il le bon sens ?
            "direction_accuracy": float(np.mean(np.sign(p_hgb[moved]) == np.sign(y_te[moved]))),
            "validation": val_metrics,
            "test_rows": int(len(test)),
            "train_rows": int(len(fit)),
        }
        m["by_period"] = {}
        for name, (start, end) in TEST_PERIODS.items():
            mk = ((test.date >= start) & (test.date <= end)).to_numpy()
            if mk.any():
                m["by_period"][name] = {
                    "mae_baseline": evaluate.mae(y_te[mk], np.zeros(mk.sum())),
                    "mae_model": evaluate.mae(y_te[mk], p_hgb[mk]),
                }
        m["beats_baseline"] = m["mae_model"] < m["mae_baseline"]
        m["recommendation_accuracy"] = None
        horizons_metrics.append(m)
        print(
            f"{line}\nJ+{h} test : baseline {m['mae_baseline']:.3f} c, ridge {m['mae_ridge']:.3f} c, "
            f"boosting {m['mae_model']:.3f} c, fourchette {m['interval_coverage']:.0%} (brute {m['interval_coverage_raw']:.0%}), "
            f"bon sens {m['direction_accuracy']:.0%} ({time.time() - t0:.0f} s)"
        )

        if h <= settings.window_days:
            test_preds[h] = pd.DataFrame(
                {f"pred{h}": p_hgb, f"low{h}": low, f"high{h}": high, f"y{h}": y_te},
                index=test.index,
            )
        if h == 3:
            # Quelles variables comptent vraiment ? On mélange une variable à la fois
            # et on regarde de combien l'erreur augmente (permutation importance)
            sub = test.sample(n=min(40_000, len(test)), random_state=0)
            r = permutation_importance(
                hgb, sub[features.FEATURES], sub[y], scoring="neg_mean_absolute_error", n_repeats=3, random_state=0
            )
            importance = sorted(
                ({"feature": f, "mae_increase": float(v)} for f, v in zip(features.FEATURES, r.importances_mean)),
                key=lambda x: -x["mae_increase"],
            )
            print("Variables les plus importantes (J+3) :", ", ".join(i["feature"] for i in importance[:8]))

    # Diagnostic sur la validation : si on donnait les conseils sans filtrer les
    # horizons, feraient-ils gagner de l'argent ? Ça répond à une autre question
    # que la MAE (le sens du prix compte plus que sa valeur exacte).
    joined_val = pd.concat([val_preds[h] for h in range(1, settings.window_days + 1)], axis=1, join="inner")
    rec_val_ungated = evaluate.evaluate_recommendations(
        joined_val.sample(n=min(200_000, len(joined_val)), random_state=0), set(range(1, settings.window_days + 1)), settings
    )
    print("Validation, conseils sans filtre :", rec_val_ungated)

    if args.skip_test:
        return

    # Recommandations rejouées sur la période de test, avec seulement les
    # horizons qui battent la baseline (règle d'honnêteté)
    allowed = {m["horizon"] for m in horizons_metrics if m["beats_baseline"]}
    joined = pd.concat([test_preds[h] for h in range(1, settings.window_days + 1)], axis=1, join="inner")
    rec_eval = evaluate.evaluate_recommendations(joined.sample(n=min(200_000, len(joined)), random_state=0), allowed, settings)
    horizons_metrics[settings.window_days - 1]["recommendation_accuracy"] = rec_eval["accuracy"]
    print("Horizons utilisés par l'appli :", sorted(allowed))
    print("Recommandations sur la période de test :", rec_eval)

    run = {
        "model_version": version,
        "trained_at": datetime.now(timezone.utc),
        "train_period": {"start": df.date.min().to_pydatetime(), "end": (test_start - pd.Timedelta(days=1)).to_pydatetime()},
        "validation_period": {"start": val_start.to_pydatetime(), "end": (test_start - pd.Timedelta(days=1)).to_pydatetime()},
        "test_period": {"start": test_start.to_pydatetime(), "end": last_day.to_pydatetime()},
        "metrics": {
            "horizons": horizons_metrics,
            "recommendations": rec_eval,
            "validation_recommendations_ungated": rec_val_ungated,
            "feature_importance": importance,
            "cleaning": cleaning,
        },
        "settings": {
            "move_cents": settings.move_cents,
            "min_confidence": settings.min_confidence,
            "window_days": settings.window_days,
            "brent_delay_days": features.BRENT_DELAY_DAYS,
            "hgb_params": HGB_PARAMS,
            "features": features.FEATURES,
        },
        "allowed_horizons": sorted(allowed),
    }

    if args.no_save:
        print("--no-save : rien n'est enregistré")
        return

    # Modèles finaux, mêmes réglages : la médiane apprend sur toutes les données,
    # les quantiles sur tout sauf les 6 derniers mois qui servent à les calibrer
    print("Réentraînement sur toutes les données...")
    out_dir.mkdir(parents=True, exist_ok=True)
    calib_start = last_day - pd.Timedelta(days=180)
    for h in features.HORIZONS:
        y = features.target_name(h)
        full = sample(df[df[y].notna()], args.sample, seed=100 + h)
        before = sample(window(df, y, None, calib_start, h), args.sample, seed=200 + h)
        calib = window(df, y, calib_start, last_day + pd.Timedelta(days=h + 1), h)
        q10 = make_hgb(0.1).fit(before[features.FEATURES], before[y])
        q90 = make_hgb(0.9).fit(before[features.FEATURES], before[y])
        lo, hi = quantile_pair(q10, q90, calib[features.FEATURES])
        bundle = {
            "median": make_hgb().fit(full[features.FEATURES], full[y]),
            "low": q10,
            "high": q90,
            "margin": conformal_margin(calib[y].to_numpy(), lo, hi),
        }
        joblib.dump(bundle, out_dir / f"h{h}.joblib")
        print(f"  J+{h} enregistré (marge de la fourchette {bundle['margin']:+.2f} c)")
    joblib.dump({"fuels": FUELS, "departments": list(df.department.cat.categories)}, out_dir / "categories.joblib")
    (MODELS_DIR / "latest.txt").write_text(version)

    db["model_runs"].insert_one(run)
    print(f"Modèle {version} enregistré dans {out_dir} et dans model_runs ({time.time() - t0:.0f} s)")


if __name__ == "__main__":
    main()
