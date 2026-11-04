"""Construction des variables (features) et des cibles du modèle.

Le modèle apprend à partir d'exemples : pour une station, un carburant et
un jour t, les variables décrivent ce qu'on sait le jour t, et la cible est
ce qui s'est passé ensuite (la variation du prix entre t et t+h).

Règle absolue : une variable du jour t n'utilise QUE des données connues le
jour t. Sinon le modèle "voit le futur" pendant l'entraînement et ses scores
sont faux. Les tests de tests/test_features.py vérifient ce point.

Les calculs se font en format large (une ligne par date, une colonne par
station) : les décalages dans le temps deviennent de simples shift(), très
rapides. On repasse en format long (une ligne par station et par jour) à la fin.
"""

from dataclasses import dataclass

import numpy as np
import pandas as pd

HORIZONS = range(1, 8)

# Le Brent de FRED est publié avec environ une semaine de retard : le jour t
# on ne connaît que le cours de t-8 au mieux. On décale donc le Brent pour
# rester honnête, même si les cours récents seraient plus utiles au modèle.
BRENT_DELAY_DAYS = 8

# Le taux de la BCE est publié le jour même vers 16 h : on prend celui de la veille
EURUSD_DELAY_DAYS = 1

NUMERIC_FEATURES = [
    "price",
    "d1",
    "d3",
    "d7",
    "gap_ma7",
    "gap_ma30",
    "days_since_change",
    "change_freq_60",
    "expected_changes",
    "dep_gap",
    "dep_moved_since_change",
    "nat_moved_since_change",
    "dep_trend7",
    "nat_trend1",
    "nat_trend3",
    "nat_trend7",
    "brent_d7",
    "brent_d14",
    "brent_d21",
    "eurusd_d14",
    "margin_gap",
    "weekday",
    "holiday_next7",
    "vacation_a",
    "vacation_a_start_in7",
    "autoroute",
]
CATEGORICAL_FEATURES = ["fuel", "department"]
FEATURES = NUMERIC_FEATURES + CATEGORICAL_FEATURES


def target_name(h: int) -> str:
    return f"y{h}"


@dataclass
class Inputs:
    """Données brutes nécessaires pour un carburant."""

    prices: pd.DataFrame  # date, station_id, price
    stations: pd.DataFrame  # station_id, department, road_type
    department_daily: pd.DataFrame  # date, department, avg_price, station_count
    market: pd.DataFrame  # date, brent_eur_l, eur_usd (une ligne par jour)
    calendar: pd.DataFrame  # date, is_holiday, vacation_a


def _days_since_change(changed: pd.DataFrame, cap: int = 60) -> pd.DataFrame:
    """Nombre de jours depuis le dernier changement de prix, colonne par colonne."""
    idx = np.arange(len(changed), dtype="float32")[:, None]
    last = pd.DataFrame(np.where(changed.to_numpy(), idx, np.nan), index=changed.index, columns=changed.columns)
    last = last.ffill()
    out = idx - last.to_numpy()
    # Pas de changement connu depuis le début des données : on met le plafond
    return pd.DataFrame(np.nan_to_num(np.minimum(out, cap), nan=cap), index=changed.index, columns=changed.columns)


def build_fuel_frame(fuel: str, inp: Inputs, with_targets: bool = True) -> pd.DataFrame:
    """Variables (et cibles) pour un carburant, une ligne par station et par jour."""
    days = pd.date_range(inp.prices.date.min(), inp.prices.date.max(), freq="D")

    # Prix en format large. Une case vide = pas de prix ce jour-là (rupture, prix abandonné)
    p = inp.prices.pivot(index="date", columns="station_id", values="price").reindex(days)
    p.columns = p.columns.astype(str)
    cents = 100.0

    feats: dict[str, pd.DataFrame] = {"price": p}
    for k in (1, 3, 7):
        feats[f"d{k}"] = (p - p.shift(k)) * cents
    feats["gap_ma7"] = (p - p.rolling(7, min_periods=4).mean()) * cents
    feats["gap_ma30"] = (p - p.rolling(30, min_periods=15).mean()) * cents

    changed = p.diff().abs() > 1e-6
    feats["days_since_change"] = _days_since_change(changed)
    # Part des jours où la station a changé son prix sur les 60 derniers jours
    known = p.notna() & p.shift(1).notna()
    feats["change_freq_60"] = changed.where(known).rolling(60, min_periods=20).mean()
    # Nombre de changements qu'on "attendrait" depuis le dernier, vu le rythme
    # habituel de la station : au-dessus de 1, un changement est sans doute proche
    feats["expected_changes"] = feats["days_since_change"] * feats["change_freq_60"]

    # Moyenne du département de chaque station (connue le jour même : calculée
    # à partir du flux du jour par l'ingestion)
    dep = inp.department_daily.pivot(index="date", columns="department", values="avg_price").reindex(days)
    station_dep = inp.stations.set_index("station_id").department.reindex(p.columns)
    dep_by_station = dep.reindex(columns=station_dep.values)
    dep_by_station.columns = p.columns
    feats["dep_gap"] = (p - dep_by_station) * cents
    feats["dep_trend7"] = (dep_by_station - dep_by_station.shift(7)) * cents
    # De combien le département a bougé depuis le dernier changement de la station.
    # Une station qui n'a pas suivi une hausse générale va souvent "rattraper".
    dep_at_change = dep_by_station.where(changed).ffill()
    feats["dep_moved_since_change"] = (dep_by_station - dep_at_change) * cents

    # Tendance nationale : moyenne des départements pondérée par le nombre de stations
    dd = inp.department_daily.assign(total=inp.department_daily.avg_price * inp.department_daily.station_count)
    g = dd.groupby("date")[["total", "station_count"]].sum()
    national = (g.total / g.station_count).reindex(days)
    national_by_station = pd.DataFrame({c: national for c in p.columns}, index=days)
    feats["nat_moved_since_change"] = (national_by_station - national_by_station.where(changed).ffill()) * cents

    market = inp.market.set_index("date").reindex(days).ffill()
    brent = market.brent_eur_l.shift(BRENT_DELAY_DAYS)
    eurusd = market.eur_usd.shift(EURUSD_DELAY_DAYS)
    # Marge brute "pompe - pétrole" : quand elle est plus haute que d'habitude,
    # les prix ont tendance à baisser (et inversement)
    margin = national - brent
    series = {
        "nat_trend1": (national - national.shift(1)) * cents,
        "nat_trend3": (national - national.shift(3)) * cents,
        "nat_trend7": (national - national.shift(7)) * cents,
        "brent_d7": (brent - brent.shift(7)) * cents,
        "brent_d14": (brent - brent.shift(14)) * cents,
        "brent_d21": (brent - brent.shift(21)) * cents,
        "eurusd_d14": (eurusd / eurusd.shift(14) - 1) * 100,
        "margin_gap": (margin - margin.rolling(90, min_periods=30).mean()) * cents,
    }

    cal = inp.calendar.set_index("date").reindex(pd.date_range(days.min(), days.max() + pd.Timedelta(days=8)))
    holiday = cal.is_holiday.fillna(False).astype(float)
    vac = cal.vacation_a.fillna(False).astype(bool)
    vac_start = (vac & ~vac.shift(1, fill_value=False)).astype(float)
    # Fériés et départs en vacances dans les 7 prochains jours : le calendrier est
    # connu à l'avance, utiliser le futur ici n'est pas tricher
    holiday_next7 = sum(holiday.shift(-k) for k in range(1, 8)).reindex(days)
    vac_start_in7 = sum(vac_start.shift(-k) for k in range(1, 8)).reindex(days).clip(upper=1)
    series["holiday_next7"] = holiday_next7
    series["vacation_a"] = vac.astype(float).reindex(days)
    series["vacation_a_start_in7"] = vac_start_in7
    series["weekday"] = pd.Series(days.day_of_week, index=days, dtype="float64")

    if with_targets:
        for h in HORIZONS:
            feats[target_name(h)] = (p.shift(-h) - p) * cents

    # Passage en format long : une ligne par (date, station) où le prix est connu
    long = pd.concat({name: df.stack(future_stack=True) for name, df in feats.items()}, axis=1)
    long.index.names = ["date", "station_id"]
    long = long[long.price.notna()].reset_index()
    for name, s in series.items():
        long[name] = long.date.map(s).astype("float32")

    info = inp.stations.set_index("station_id")
    long["department"] = long.station_id.map(info.department)
    long["autoroute"] = (long.station_id.map(info.road_type) == "A").astype("float32")
    long["fuel"] = fuel

    num_cols = [c for c in long.columns if c not in ("date", "station_id", "department", "fuel")]
    long[num_cols] = long[num_cols].astype("float32")
    return long


def as_categories(df: pd.DataFrame, fuels: list[str], departments: list[str]) -> pd.DataFrame:
    """Types catégoriels fixes : le modèle doit voir les mêmes catégories à
    l'entraînement et à la prédiction."""
    df = df.copy()
    df["fuel"] = pd.Categorical(df.fuel, categories=fuels)
    df["department"] = pd.Categorical(df.department, categories=departments)
    return df
