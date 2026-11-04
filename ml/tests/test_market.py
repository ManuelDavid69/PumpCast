"""Tests des fonctions pures de récupération des données de marché."""

from datetime import date, datetime, timezone

from pumpcast_ml.external.market import (
    build_market_rows,
    parse_ecb_csv,
    parse_fred_csv,
)


def test_parse_fred_csv_ignore_vides_et_doublons() -> None:
    text = (
        "observation_date,DCOILBRENTEU\n"
        "2026-10-20,115.22\n"
        "2026-10-21,.\n"
        "2026-10-22,\n"
        "2026-10-23,128.30\n"
        "2026-10-23,128.31\n"
    )
    assert parse_fred_csv(text) == {
        date(2026, 10, 20): 115.22,
        date(2026, 10, 23): 128.31,
    }


def test_parse_ecb_csv_ignore_valeur_vide_et_titre_avec_virgule() -> None:
    text = (
        "KEY,FREQ,TIME_PERIOD,OBS_VALUE,TITLE\n"
        'EXR,D,2026-10-28,1.1133,"ECB reference exchange rate, US dollar/Euro"\n'
        'EXR,D,2026-10-29,,"ECB reference exchange rate, US dollar/Euro"\n'
        'EXR,D,2026-10-30,1.1172,"ECB reference exchange rate, US dollar/Euro"\n'
    )
    assert parse_ecb_csv(text) == {
        date(2026, 10, 28): 1.1133,
        date(2026, 10, 30): 1.1172,
    }


def test_build_market_rows_fusionne_et_trie_par_date() -> None:
    d1, d2, d3 = date(2026, 10, 1), date(2026, 10, 2), date(2026, 10, 3)
    rows = build_market_rows({d1: 80.0, d2: 81.0}, {d2: 1.10, d3: 1.11})

    assert [r["date"].date() for r in rows] == [d1, d2, d3]
    assert all(r["date"].tzinfo == timezone.utc for r in rows)
    assert all(r["date"].time() == datetime.min.time() for r in rows)

    by_date = {r["date"].date(): r for r in rows}
    assert by_date[d1]["brent_usd"] == 80.0
    assert by_date[d1]["eur_usd"] is None
    assert by_date[d3]["brent_usd"] is None
    assert by_date[d3]["eur_usd"] == 1.11
