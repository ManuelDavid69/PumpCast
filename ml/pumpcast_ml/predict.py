"""Prédictions du jour, une fois par jour après l'ingestion.

Usage : python -m pumpcast_ml.predict

Pour chaque station et carburant de l'historique (AURA), on construit les
variables du dernier jour connu, on prédit J+1 à J+7 avec la fourchette, et
on écrit le résultat dans la collection predictions que l'API Go sert ensuite.
"""

import argparse
from datetime import datetime, timedelta, timezone

import joblib
import numpy as np
import pandas as pd
from pymongo import ASCENDING, DESCENDING

from pumpcast_ml import data, features
from pumpcast_ml.db import as_mongo_date, get_db
from pumpcast_ml.recommend import HorizonPrediction, Settings, reasons, recommend
from pumpcast_ml.train import MAX_DEP_GAP_CENTS, MODELS_DIR

# Il faut au moins 90 jours d'historique pour calculer toutes les variables
HISTORY_DAYS = 120
# On garde un mois de prédictions pour pouvoir les comparer à la réalité plus tard
KEEP_DAYS = 30


def load_models(version: str) -> tuple[dict[int, dict], dict]:
    folder = MODELS_DIR / version
    models = {h: joblib.load(folder / f"h{h}.joblib") for h in features.HORIZONS}
    return models, joblib.load(folder / "categories.joblib")


def main(argv: list[str] | None = None) -> None:
    parser = argparse.ArgumentParser(description="Prédictions du jour")
    parser.parse_args(argv)

    version = (MODELS_DIR / "latest.txt").read_text().strip()
    models, cats = load_models(version)
    db = get_db()
    run = db["model_runs"].find_one({"model_version": version})
    if run is None:
        raise SystemExit(f"modèle {version} absent de model_runs, relancer l'entraînement")
    allowed = set(run.get("allowed_horizons", []))
    settings = Settings(**{k: run["settings"][k] for k in ("move_cents", "min_confidence", "window_days")})

    start = (datetime.now(timezone.utc) - timedelta(days=HISTORY_DAYS)).date()
    stations = data.load_stations(db)
    market = data.load_market(db)
    calendar = data.load_calendar(db)
    generated_at = datetime.now(timezone.utc)

    docs = []
    for fuel in cats["fuels"]:
        prices = data.load_prices(db, fuel, start=start)
        if prices.empty:
            continue
        inp = features.Inputs(prices, stations, data.load_department_daily(db, fuel), market, calendar)
        df = features.build_fuel_frame(fuel, inp, with_targets=False)
        today = df[(df.date == df.date.max()) & (df.dep_gap.abs() <= MAX_DEP_GAP_CENTS)]
        today = features.as_categories(today, cats["fuels"], cats["departments"])
        if today.empty:
            continue
        X = today[features.FEATURES]

        preds = {}
        for h, bundle in models.items():
            # Les trois modèles sont appris séparément, on remet les valeurs dans l'ordre
            trio = np.sort(np.vstack([bundle["low"].predict(X), bundle["median"].predict(X), bundle["high"].predict(X)]), axis=0)
            # Fourchette élargie de la marge calibrée à l'entraînement (calibration conformale)
            trio[0] -= bundle["margin"]
            trio[2] += bundle["margin"]
            preds[h] = trio

        base_day = today.date.iloc[0]
        for i, row in enumerate(today.itertuples(index=False)):
            per_h = [HorizonPrediction(h, float(preds[h][1, i]), float(preds[h][0, i]), float(preds[h][2, i])) for h in features.HORIZONS]
            rec = recommend(per_h, allowed, settings)
            price = round(float(row.price), 3)  # float32 dans le DataFrame, on revient au millième
            docs.append(
                {
                    "station_id": row.station_id,
                    "fuel": fuel,
                    "generated_at": generated_at,
                    "base_date": as_mongo_date(base_day.date()),
                    "base_price": price,
                    "model_version": version,
                    # Seulement les horizons où le modèle bat la baseline (règle d'honnêteté)
                    "horizon": [
                        {
                            "date": as_mongo_date((base_day + pd.Timedelta(days=p.horizon)).date()),
                            "price": round(price + p.delta / 100, 4),
                            "low": round(price + p.low / 100, 4),
                            "high": round(price + p.high / 100, 4),
                        }
                        for p in per_h
                        if p.horizon in allowed
                    ],
                    "recommendation": rec.kind if rec else None,
                    "confidence": rec.confidence if rec else 0.0,
                    "reasons": reasons({k: float(getattr(row, k)) for k in ("nat_trend7", "dep_gap", "brent_d14", "days_since_change") if not pd.isna(getattr(row, k))}),
                }
            )
        print(f"{fuel} : {len(today)} stations, prix du {base_day:%d/%m/%Y}")

    coll = db["predictions"]
    coll.create_index([("station_id", ASCENDING), ("fuel", ASCENDING), ("generated_at", DESCENDING)])
    if docs:
        coll.insert_many(docs)
    removed = coll.delete_many({"generated_at": {"$lt": generated_at - timedelta(days=KEEP_DAYS)}}).deleted_count
    counts = pd.Series([d["recommendation"] for d in docs]).value_counts(dropna=False).to_dict()
    print(f"{len(docs)} prédictions écrites (modèle {version}), {removed} anciennes supprimées, recommandations : {counts}")


if __name__ == "__main__":
    main()
