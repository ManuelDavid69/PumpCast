"""Cours du Brent (FRED) et taux de change EUR/USD (BCE).

Le pétrole est acheté en dollars : à Brent égal, un euro qui baisse fait
monter le prix à la pompe. Les deux séries sont donc utiles au modèle.
"""

import csv
import io
from datetime import date

import requests
from pymongo import UpdateOne
from pymongo.database import Database

from pumpcast_ml.db import as_mongo_date

# Export CSV public de FRED, pas besoin de clé API pour une série simple
FRED_URL = "https://fred.stlouisfed.org/graph/fredgraph.csv"
BRENT_SERIES = "DCOILBRENTEU"

# Taux de référence quotidien de la BCE : combien de dollars pour 1 euro
ECB_URL = "https://data-api.ecb.europa.eu/service/data/EXR/D.USD.EUR.SP00.A"

TIMEOUT = 60


def parse_fred_csv(text: str) -> dict[date, float]:
    """Lit l'export CSV de FRED.

    Les jours sans cotation (jours fériés à Londres) valent "." et sont ignorés.
    Le fichier contient parfois deux fois la même date, la dernière gagne.
    """
    values: dict[date, float] = {}
    reader = csv.reader(io.StringIO(text))
    next(reader, None)  # en-tête
    for row in reader:
        if len(row) < 2 or row[1].strip() in ("", "."):
            continue
        values[date.fromisoformat(row[0].strip())] = float(row[1])
    return values


def parse_ecb_csv(text: str) -> dict[date, float]:
    """Lit le CSV de l'API de la BCE (colonnes TIME_PERIOD et OBS_VALUE)."""
    values: dict[date, float] = {}
    for row in csv.DictReader(io.StringIO(text)):
        raw = (row.get("OBS_VALUE") or "").strip()
        if raw == "":
            continue
        values[date.fromisoformat(row["TIME_PERIOD"].strip())] = float(raw)
    return values


def fetch_brent(start: date) -> dict[date, float]:
    resp = requests.get(
        FRED_URL, params={"id": BRENT_SERIES, "cosd": start.isoformat()}, timeout=TIMEOUT
    )
    resp.raise_for_status()
    return parse_fred_csv(resp.text)


def fetch_eur_usd(start: date) -> dict[date, float]:
    resp = requests.get(
        ECB_URL,
        params={"format": "csvdata", "startPeriod": start.isoformat()},
        timeout=TIMEOUT,
    )
    resp.raise_for_status()
    return parse_ecb_csv(resp.text)


def build_market_rows(brent: dict[date, float], eur_usd: dict[date, float]) -> list[dict]:
    """Fusionne les deux séries, une ligne par date.

    On ne bouche pas les trous (week-ends, jours fériés) ici : c'est un choix
    de construction des features, il sera fait en phase 5.
    """
    rows = []
    for d in sorted(set(brent) | set(eur_usd)):
        rows.append({"date": as_mongo_date(d), "brent_usd": brent.get(d), "eur_usd": eur_usd.get(d)})
    return rows


def save_market(db: Database, rows: list[dict]) -> int:
    """Enregistre les lignes (upsert par date). Renvoie le nombre de lignes créées ou modifiées."""
    coll = db["market_data"]
    coll.create_index("date", unique=True)
    if not rows:
        return 0
    # On n'écrit que les valeurs connues : le Brent est publié avec quelques
    # jours de retard, un None ne doit pas écraser une valeur déjà en base
    ops = [
        UpdateOne(
            {"date": r["date"]},
            {"$set": {k: v for k, v in r.items() if v is not None}},
            upsert=True,
        )
        for r in rows
    ]
    res = coll.bulk_write(ops, ordered=False)
    return res.upserted_count + res.modified_count
