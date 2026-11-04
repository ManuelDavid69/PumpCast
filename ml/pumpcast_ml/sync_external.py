"""Met à jour les données externes : Brent, EUR/USD, jours fériés, vacances.

Usage : python -m pumpcast_ml.sync_external --start 2023-01-01

Peut être relancé autant qu'on veut, tout est écrit en upsert par date.
"""

import argparse
from datetime import date, timedelta

from pumpcast_ml.db import get_db
from pumpcast_ml.external import calendar, market


def main(argv: list[str] | None = None) -> None:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--start", type=date.fromisoformat, default=date(2023, 1, 1))
    args = parser.parse_args(argv)
    start: date = args.start
    today = date.today()

    db = get_db()

    brent = market.fetch_brent(start)
    eur_usd = market.fetch_eur_usd(start)
    n = market.save_market(db, market.build_market_rows(brent, eur_usd))
    print(
        f"marché : {len(brent)} jours de Brent (dernier {max(brent)}), "
        f"{len(eur_usd)} jours d'EUR/USD (dernier {max(eur_usd)}), {n} lignes écrites"
    )

    # Le calendrier va un an dans le futur : le modèle en a besoin pour
    # prédire les 7 prochains jours (vacances qui approchent, ponts...)
    end = today + timedelta(days=365)
    holidays: dict[date, str] = {}
    for year in range(start.year, end.year + 1):
        holidays.update(calendar.fetch_holidays(year))
    vacations = calendar.vacation_days(calendar.fetch_vacation_records(start.year))
    rows = calendar.build_calendar_rows(start, end, holidays, vacations)
    n = calendar.save_calendar(db, rows)
    print(
        f"calendrier : du {start} au {end}, {len(holidays)} jours fériés, "
        f"{len(vacations)} jours de vacances (toutes zones), {n} lignes écrites"
    )


if __name__ == "__main__":
    main()
