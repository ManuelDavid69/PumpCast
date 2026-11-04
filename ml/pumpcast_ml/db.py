"""Connexion à MongoDB, configurée par les variables d'environnement."""

import os
from datetime import date, datetime, timezone

from pymongo import MongoClient
from pymongo.database import Database


def get_db() -> Database:
    """Renvoie la base PumpCast. Mêmes variables et valeurs par défaut que l'API Go."""
    uri = os.environ.get("MONGO_URI") or "mongodb://localhost:27017"
    name = os.environ.get("MONGO_DB") or "pumpcast"
    # tz_aware : les dates relues depuis Mongo sont en UTC explicite
    client = MongoClient(uri, tz_aware=True, serverSelectionTimeoutMS=5000)
    return client[name]


def as_mongo_date(d: date) -> datetime:
    """Une date calendaire est stockée à minuit UTC, comme les prix côté Go.

    Comme ça une jointure entre prices.ts et market_data.date marche sans
    se soucier des fuseaux horaires.
    """
    return datetime(d.year, d.month, d.day, tzinfo=timezone.utc)
