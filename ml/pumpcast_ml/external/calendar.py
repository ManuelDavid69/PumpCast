"""Jours fériés et vacances scolaires.

Les départs en vacances et les ponts font souvent bouger les prix,
surtout sur autoroute. On stocke un document par jour dans `calendar`.
"""

from datetime import date, datetime, timedelta
from zoneinfo import ZoneInfo

import requests
from pymongo import UpdateOne
from pymongo.database import Database

from pumpcast_ml.db import as_mongo_date

HOLIDAYS_URL = "https://calendrier.api.gouv.fr/jours-feries/metropole/{year}.json"
VACATIONS_URL = (
    "https://data.education.gouv.fr/api/explore/v2.1/catalog/datasets/"
    "fr-en-calendrier-scolaire/exports/json"
)
ZONES = ("A", "B", "C")
PARIS = ZoneInfo("Europe/Paris")
TIMEOUT = 60


def fetch_holidays(year: int) -> dict[date, str]:
    resp = requests.get(HOLIDAYS_URL.format(year=year), timeout=TIMEOUT)
    resp.raise_for_status()
    return {date.fromisoformat(d): name for d, name in resp.json().items()}


def fetch_vacation_records(first_year: int) -> list[dict]:
    """Récupère les périodes de vacances des zones A, B et C qui finissent
    à partir du 1er janvier de first_year."""
    zones = ", ".join(f'"Zone {z}"' for z in ZONES)
    # L'API refuse les comparaisons sur du texte (annee_scolaire), on filtre
    # donc sur end_date qui est un vrai champ date
    where = f"zones in ({zones}) and end_date >= date'{first_year}-01-01'"
    resp = requests.get(VACATIONS_URL, params={"where": where}, timeout=TIMEOUT)
    resp.raise_for_status()
    return resp.json()


def _paris_date(iso: str) -> date:
    """Les dates de l'API sont en UTC ("2025-10-17T22:00:00+00:00"),
    ce qui correspond à minuit heure de Paris le lendemain."""
    return datetime.fromisoformat(iso).astimezone(PARIS).date()


def vacation_days(records: list[dict]) -> dict[date, set[str]]:
    """Transforme les périodes en jours : pour chaque jour, les zones en vacances.

    La date de fin est le jour de la rentrée, elle est donc exclue.
    Les lignes "Enseignants" sont ignorées (on veut les dates des élèves),
    comme les lignes dont le début est égal à la fin (simples annonces).
    L'API renvoie une ligne par académie, d'où les doublons gérés par le set.
    """
    days: dict[date, set[str]] = {}
    for r in records:
        if r.get("population") == "Enseignants":
            continue
        zone = (r.get("zones") or "").removeprefix("Zone ").strip()
        if zone not in ZONES or not r.get("start_date") or not r.get("end_date"):
            continue
        start, end = _paris_date(r["start_date"]), _paris_date(r["end_date"])
        d = start
        while d < end:
            days.setdefault(d, set()).add(zone)
            d += timedelta(days=1)
    return days


def build_calendar_rows(
    start: date, end: date, holidays: dict[date, str], vacations: dict[date, set[str]]
) -> list[dict]:
    """Un document par jour entre start et end inclus."""
    rows = []
    d = start
    while d <= end:
        rows.append(
            {
                "date": as_mongo_date(d),
                "is_holiday": d in holidays,
                "holiday_name": holidays.get(d),
                "school_vacation_zones": sorted(vacations.get(d, set())),
            }
        )
        d += timedelta(days=1)
    return rows


def save_calendar(db: Database, rows: list[dict]) -> int:
    coll = db["calendar"]
    coll.create_index("date", unique=True)
    if not rows:
        return 0
    ops = [UpdateOne({"date": r["date"]}, {"$set": r}, upsert=True) for r in rows]
    res = coll.bulk_write(ops, ordered=False)
    return res.upserted_count + res.modified_count
