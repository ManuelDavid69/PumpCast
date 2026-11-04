"""Chargement des données depuis MongoDB vers des DataFrames pandas.

Un DataFrame c'est un tableau en mémoire (comme le résultat d'un SELECT),
sur lequel on peut faire des calculs colonne par colonne très rapidement.
"""

from datetime import date

import pandas as pd
from pymongo.database import Database

from pumpcast_ml.db import as_mongo_date

# 1 baril = 158,987 litres
LITERS_PER_BARREL = 158.987


def _to_day(series: pd.Series) -> pd.Series:
    """Les dates sont stockées à minuit UTC : on garde juste le jour, sans fuseau."""
    return pd.to_datetime(series, utc=True).dt.tz_localize(None).dt.normalize()


def load_prices(db: Database, fuel: str, start: date | None = None) -> pd.DataFrame:
    """Historique d'un carburant : une ligne par station et par jour.

    Colonnes : date, station_id, price. On charge un carburant à la fois
    pour ne pas mettre 6 millions de lignes en mémoire d'un coup.
    """
    query: dict = {"meta.fuel": fuel}
    if start is not None:
        query["ts"] = {"$gte": as_mongo_date(start)}
    cursor = db["prices"].find(query, {"_id": 0, "ts": 1, "meta.station_id": 1, "price": 1})
    rows = [(d["ts"], d["meta"]["station_id"], d["price"]) for d in cursor]
    df = pd.DataFrame(rows, columns=["date", "station_id", "price"])
    df["date"] = _to_day(df["date"])
    # category : chaque identifiant n'est stocké qu'une fois, gros gain de mémoire
    df["station_id"] = df["station_id"].astype("category")
    return df.sort_values(["station_id", "date"], ignore_index=True)


def load_stations(db: Database) -> pd.DataFrame:
    """Infos fixes des stations : département, type de route, coordonnées."""
    cursor = db["stations"].find(
        {}, {"_id": 1, "department": 1, "road_type": 1, "city": 1, "location": 1}
    )
    rows = []
    for d in cursor:
        lng, lat = (d.get("location") or {}).get("coordinates", [None, None])
        rows.append((d["_id"], d["department"], d.get("road_type"), d.get("city"), lat, lng))
    return pd.DataFrame(
        rows, columns=["station_id", "department", "road_type", "city", "lat", "lng"]
    )


def load_department_daily(db: Database, fuel: str) -> pd.DataFrame:
    """Moyennes quotidiennes par département (toute la France) pour un carburant."""
    cursor = db["department_daily"].find({"meta.fuel": fuel}, {"_id": 0})
    rows = [
        (d["ts"], d["meta"]["department"], d["avg_price"], d["min_price"], d["max_price"], d["station_count"])
        for d in cursor
    ]
    df = pd.DataFrame(
        rows, columns=["date", "department", "avg_price", "min_price", "max_price", "station_count"]
    )
    df["date"] = _to_day(df["date"])
    return df.sort_values(["department", "date"], ignore_index=True)


def load_market(db: Database) -> pd.DataFrame:
    """Brent et EUR/USD, une ligne par jour calendaire.

    Les marchés sont fermés le week-end : on reprend la dernière cotation
    (forward fill), c'est ce que "voit" une station qui fixe ses prix un samedi.
    On ajoute le Brent converti en euros par litre, plus parlant à côté
    d'un prix à la pompe.
    """
    cursor = db["market_data"].find({}, {"_id": 0})
    df = pd.DataFrame(list(cursor))
    df["date"] = _to_day(df["date"])
    df = df.set_index("date").sort_index()
    full = pd.date_range(df.index.min(), df.index.max(), freq="D")
    df = df.reindex(full).ffill()
    df.index.name = "date"
    df["brent_eur_l"] = df["brent_usd"] / df["eur_usd"] / LITERS_PER_BARREL
    return df.reset_index()


def load_calendar(db: Database) -> pd.DataFrame:
    """Jours fériés et vacances scolaires, une ligne par jour."""
    cursor = db["calendar"].find({}, {"_id": 0})
    df = pd.DataFrame(list(cursor))
    df["date"] = _to_day(df["date"])
    for zone in ("A", "B", "C"):
        df[f"vacation_{zone.lower()}"] = df["school_vacation_zones"].apply(lambda z, zone=zone: zone in z)
    return df.drop(columns=["school_vacation_zones"]).sort_values("date", ignore_index=True)
