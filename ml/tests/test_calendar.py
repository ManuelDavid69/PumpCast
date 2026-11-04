"""Tests des fonctions pures du calendrier (fériés et vacances scolaires)."""

from datetime import date, datetime, timezone

from pumpcast_ml.external.calendar import build_calendar_rows, vacation_days


def test_vacation_days_zones_doublons_et_lignes_ignorees() -> None:
    records = [
        {
            "zones": "Zone A",
            "population": "-",
            "start_date": "2025-10-17T22:00:00+00:00",
            "end_date": "2025-11-02T23:00:00+00:00",
        },
        {
            "zones": "Zone B",
            "population": "-",
            "start_date": "2025-10-17T22:00:00+00:00",
            "end_date": "2025-11-02T23:00:00+00:00",
        },
        {
            "zones": "Zone A",
            "population": "-",
            "start_date": "2025-10-17T22:00:00+00:00",
            "end_date": "2025-11-02T23:00:00+00:00",
        },
        {
            "zones": "Zone A",
            "population": "Enseignants",
            "start_date": "2026-07-03T22:00:00+00:00",
            "end_date": "2026-08-30T22:00:00+00:00",
        },
        {
            "zones": "Zone A",
            "population": "Élèves",
            "start_date": "2026-07-03T22:00:00+00:00",
            "end_date": "2026-08-31T22:00:00+00:00",
        },
        {
            "zones": "Zone C",
            "population": "-",
            "start_date": "2028-07-03T22:00:00+00:00",
            "end_date": "2028-07-03T22:00:00+00:00",
        },
        {
            "zones": "Corse",
            "population": "-",
            "start_date": "2025-10-17T22:00:00+00:00",
            "end_date": "2025-11-02T23:00:00+00:00",
        },
    ]

    days = vacation_days(records)

    assert date(2025, 10, 17) not in days
    assert days[date(2025, 10, 18)] == {"A", "B"}
    assert days[date(2025, 11, 2)] == {"A", "B"}
    assert date(2025, 11, 3) not in days
    assert days[date(2026, 7, 4)] == {"A"}
    assert days[date(2026, 8, 31)] == {"A"}
    assert date(2026, 9, 1) not in days
    assert not any(d.year == 2028 for d in days)
    assert all("Corse" not in zones for zones in days.values())


def test_build_calendar_rows_jours_feries_et_zones() -> None:
    holidays = {date(2026, 4, 6): "Lundi de Pâques"}
    vacations = {date(2026, 4, 4): {"A"}, date(2026, 4, 5): {"C", "A"}}

    rows = build_calendar_rows(date(2026, 4, 4), date(2026, 4, 7), holidays, vacations)

    assert len(rows) == 4
    assert all(r["date"].tzinfo == timezone.utc for r in rows)

    by_date = {r["date"].date(): r for r in rows}

    # 6 avril : jour férié, les autres non
    assert by_date[date(2026, 4, 6)]["is_holiday"] is True
    assert by_date[date(2026, 4, 6)]["holiday_name"] == "Lundi de Pâques"
    for d in (date(2026, 4, 4), date(2026, 4, 5), date(2026, 4, 7)):
        assert by_date[d]["is_holiday"] is False
        assert by_date[d]["holiday_name"] is None

    # zones triées par ordre alphabétique
    assert by_date[date(2026, 4, 5)]["school_vacation_zones"] == ["A", "C"]
    assert by_date[date(2026, 4, 7)]["school_vacation_zones"] == []
