"""Tests des variables : surtout qu'aucune variable du jour t ne regarde le futur."""

import numpy as np
import pandas as pd
import pytest

from pumpcast_ml import features

DAYS = pd.date_range("2025-01-01", periods=60, freq="D")


def make_inputs(price_a: list[float], brent: list[float] | None = None) -> features.Inputs:
    prices = pd.DataFrame(
        [(d, "A", p) for d, p in zip(DAYS, price_a)] + [(d, "B", 1.80) for d in DAYS],
        columns=["date", "station_id", "price"],
    )
    stations = pd.DataFrame(
        {"station_id": ["A", "B"], "department": ["69", "69"], "road_type": ["R", "A"]}
    )
    dep = pd.DataFrame({"date": DAYS, "department": "69", "avg_price": 1.75, "station_count": 10})
    market = pd.DataFrame(
        {"date": DAYS, "brent_eur_l": brent if brent is not None else [0.5] * len(DAYS), "eur_usd": 1.1}
    )
    calendar = pd.DataFrame({"date": DAYS, "is_holiday": False, "vacation_a": False})
    calendar.loc[calendar.date == "2025-01-20", "vacation_a"] = True
    return features.Inputs(prices, stations, dep, market, calendar)


def station_a(df: pd.DataFrame) -> pd.DataFrame:
    return df[df.station_id == "A"].set_index("date")


def test_targets_are_future_changes_in_cents() -> None:
    prices = [1.70] * 30 + [1.75] * 30
    df = station_a(features.build_fuel_frame("GAZOLE", make_inputs(prices)))
    # Le prix passe de 1,70 à 1,75 le 31e jour
    assert df.loc["2025-01-30", "y1"] == pytest.approx(5.0)
    assert df.loc["2025-01-28", "y3"] == pytest.approx(5.0)
    assert df.loc["2025-01-27", "y3"] == pytest.approx(0.0)
    # Pas de cible pour le dernier jour (on ne connaît pas le lendemain)
    assert np.isnan(df.loc["2025-03-01", "y1"])


def test_features_do_not_look_at_the_future() -> None:
    base = [1.70] * 60
    changed = [1.70] * 40 + [2.50] * 20  # on change seulement le futur après le 40e jour
    a = station_a(features.build_fuel_frame("GAZOLE", make_inputs(base)))
    b = station_a(features.build_fuel_frame("GAZOLE", make_inputs(changed)))
    cutoff = DAYS[39]
    cols = features.NUMERIC_FEATURES
    pd.testing.assert_frame_equal(a.loc[:cutoff, cols], b.loc[:cutoff, cols])


def test_brent_uses_only_published_prices() -> None:
    brent_a = [0.5] * 60
    # Le Brent change 5 jours avant le jour t : il n'est pas encore publié le jour t
    brent_b = [0.5] * 35 + [0.9] * 25
    t = DAYS[39]
    a = station_a(features.build_fuel_frame("GAZOLE", make_inputs([1.7] * 60, brent_a)))
    b = station_a(features.build_fuel_frame("GAZOLE", make_inputs([1.7] * 60, brent_b)))
    assert a.loc[t, "brent_d7"] == b.loc[t, "brent_d7"] == 0
    # 8 jours plus tard la hausse est connue
    assert b.loc[DAYS[43], "brent_d7"] == pytest.approx(40.0)


def test_days_since_change_and_department_gap() -> None:
    prices = [1.70] * 10 + [1.72] * 50
    df = station_a(features.build_fuel_frame("GAZOLE", make_inputs(prices)))
    assert df.loc[DAYS[10], "days_since_change"] == 0
    assert df.loc[DAYS[15], "days_since_change"] == 5
    assert df.loc[DAYS[15], "dep_gap"] == pytest.approx(-3.0)  # 1,72 - 1,75
    assert df.loc[DAYS[15], "d7"] == pytest.approx(2.0)


def test_static_and_calendar_features() -> None:
    df = features.build_fuel_frame("E10", make_inputs([1.7] * 60))
    b = df[df.station_id == "B"].iloc[0]
    assert b.autoroute == 1.0 and b.fuel == "E10" and b.department == "69"
    a = station_a(df)
    # Un départ en vacances le 20 janvier : signalé dans les 7 jours qui précèdent
    assert a.loc["2025-01-13", "vacation_a_start_in7"] == 1
    assert a.loc["2025-01-12", "vacation_a_start_in7"] == 0
    assert a.loc["2025-01-20", "vacation_a"] == 1


def test_missing_prices_are_not_rows() -> None:
    prices = [1.70] * 20 + [np.nan] * 5 + [1.70] * 35
    inp = make_inputs(prices)
    inp.prices = inp.prices.dropna()
    df = station_a(features.build_fuel_frame("GAZOLE", inp))
    assert len(df) == 55
    # Pas de cible quand le prix de t+h est inconnu
    assert np.isnan(df.loc[DAYS[19], "y1"])
