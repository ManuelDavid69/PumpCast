// Fiche station, rendue côté serveur : rapide à afficher et lisible par
// les moteurs de recherche. Le carburant et la période de la courbe sont
// dans l'URL (?carburant=E10&jours=90), chaque vue a donc son propre lien.

import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Suspense } from "react";
import { PriceChart } from "@/components/PriceChart";
import { Price } from "@/components/Price";
import { UpdatedAt } from "@/components/UpdatedAt";
import { getDepartmentStats, getStation, getStationHistory } from "@/lib/api";
import { departmentName } from "@/lib/departments";
import { FUEL_LABELS, FUELS, formatCents, formatCentsAbs, formatName, type Fuel } from "@/lib/format";

const PERIODS = [30, 90, 365];

function pickFuel(raw: string | string[] | undefined, available: Fuel[]): Fuel {
  const wanted = typeof raw === "string" ? (raw.toUpperCase() as Fuel) : undefined;
  if (wanted && FUELS.includes(wanted)) {
    return wanted;
  }
  return available[0] ?? "GAZOLE";
}

function pickDays(raw: string | string[] | undefined): number {
  const n = Number(raw);
  return PERIODS.includes(n) ? n : 90;
}

export async function generateMetadata({ params }: PageProps<"/stations/[id]">): Promise<Metadata> {
  const { id } = await params;
  const station = await getStation(id);
  if (!station) {
    return { title: "Station introuvable" };
  }
  const where = `${formatName(station.address)}, ${formatName(station.city)}`;
  return {
    title: `Station ${where}`,
    description: `Prix des carburants à la station ${where}, historique et prévision sur 7 jours.`,
  };
}

export default function StationPage(props: PageProps<"/stations/[id]">) {
  return (
    <Suspense fallback={<div className="mx-auto h-96 max-w-7xl animate-pulse px-4 py-8" />}>
      <StationView {...props} />
    </Suspense>
  );
}

async function StationView({ params, searchParams }: PageProps<"/stations/[id]">) {
  const { id } = await params;
  const query = await searchParams;
  const station = await getStation(id);
  if (!station) {
    notFound();
  }

  const sold = station.currentPrices.map((p) => p.fuel);
  const available = FUELS.filter((f) => sold.includes(f));
  const fuel = pickFuel(query.carburant, available);
  const days = pickDays(query.jours);
  const [{ history, prediction }, department] = await Promise.all([
    getStationHistory(id, fuel, days),
    getDepartmentStats(station.department, fuel),
  ]);
  const current = station.currentPrices.find((p) => p.fuel === fuel);
  const closed = station.currentPrices.length === 0;

  const href = (f: Fuel, d: number) => `/stations/${id}?carburant=${f}&jours=${d}`;
  const first = history[0]?.price;
  const last = history.at(-1)?.price;

  return (
    <div className="mx-auto max-w-7xl px-4 py-6">
      <p className="mb-1 text-sm text-muted">
        <Link href={`/departements/${station.department}`} className="hover:underline">
          {departmentName(station.department)}
        </Link>
      </p>
      <h1 className="font-board text-3xl font-semibold leading-tight sm:text-4xl">{formatName(station.address)}</h1>
      <p className="mt-1 text-[17px] text-muted">
        {station.postalCode} {formatName(station.city)}
        {station.roadType === "A" && (
          <span className="ml-2 rounded-sm bg-autoroute px-1.5 py-px align-middle text-xs font-medium text-white">Autoroute</span>
        )}
      </p>

      <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="min-w-0 space-y-8">
          {closed ? (
            <p className="rounded border border-line bg-white p-4 text-[15px]">
              Cette station n&apos;est plus dans le flux officiel des prix : elle a sans doute fermé. Son historique reste
              consultable ci-dessous.
            </p>
          ) : (
            <section aria-labelledby="prediction-title" className="rounded border border-line bg-white p-5">
              <h2 id="prediction-title" className="text-lg font-semibold">
                Faire le plein maintenant ou attendre ?
              </h2>
              {prediction ? (
                <p className="mt-2 text-[15px]">Prévision disponible (affichage complet en cours de construction).</p>
              ) : (
                <p className="mt-2 max-w-prose text-[15px] leading-relaxed text-muted">
                  Pas encore de prévision pour cette station. Le modèle qui estime le prix des 7 prochains jours est en
                  cours d&apos;entraînement.
                </p>
              )}
              {current?.price && department && (
                <p className="mt-3 text-[15px]">
                  {Math.abs(current.price - department.avgPrice) < 0.0005 ? (
                    <>Le {FUEL_LABELS[fuel]} est au prix moyen du département.</>
                  ) : (
                    <>
                      Le {FUEL_LABELS[fuel]} est {formatCentsAbs(current.price - department.avgPrice)}{" "}
                      {current.price < department.avgPrice ? "moins cher" : "plus cher"} que la moyenne du département (
                      {departmentName(station.department)} : <Price value={department.avgPrice} />
                      ).
                    </>
                  )}
                </p>
              )}
            </section>
          )}

          <section aria-labelledby="history-title">
            <div className="mb-3 flex flex-wrap items-end justify-between gap-3">
              <h2 id="history-title" className="text-lg font-semibold">
                Historique du {FUEL_LABELS[fuel]}
              </h2>
              <nav aria-label="Période" className="flex gap-1 text-sm">
                {PERIODS.map((d) => (
                  <Link
                    key={d}
                    href={href(fuel, d)}
                    aria-current={d === days ? "page" : undefined}
                    className="rounded px-2 py-1 font-medium text-muted aria-[current=page]:bg-line aria-[current=page]:text-ink"
                    scroll={false}
                  >
                    {d === 365 ? "1 an" : `${d} jours`}
                  </Link>
                ))}
              </nav>
            </div>
            {available.length > 1 && (
              <nav aria-label="Carburant" className="mb-3 flex flex-wrap gap-1">
                {available.map((f) => (
                  <Link
                    key={f}
                    href={href(f, days)}
                    aria-current={f === fuel ? "page" : undefined}
                    scroll={false}
                    className="rounded border border-line bg-white px-2.5 py-1.5 text-sm font-medium text-muted aria-[current=page]:border-ink aria-[current=page]:bg-ink aria-[current=page]:text-white"
                  >
                    {FUEL_LABELS[f]}
                  </Link>
                ))}
              </nav>
            )}
            <div className="rounded border border-line bg-white p-3">
              <PriceChart series={[{ id: "price", label: FUEL_LABELS[fuel], color: "var(--color-ink)", points: history }]} />
            </div>
            {first !== undefined && last !== undefined && (
              <p className="mt-2 text-sm text-muted">
                Sur la période : {formatCents(last - first)}, de{" "}
                <Price value={Math.min(...history.map((h) => h.price))} /> à{" "}
                <Price value={Math.max(...history.map((h) => h.price))} />.
              </p>
            )}
          </section>
        </div>

        <aside className="space-y-6">
          {!closed && (
            <section aria-label="Prix actuels" className="rounded bg-ink p-5 text-white">
              <ul className="space-y-3">
                {FUELS.filter((f) => sold.includes(f)).map((f) => {
                  const p = station.currentPrices.find((c) => c.fuel === f)!;
                  return (
                    <li key={f} className="flex items-baseline justify-between gap-3 border-b border-white/15 pb-2 last:border-0">
                      <span className="text-[15px] font-medium">{FUEL_LABELS[f]}</span>
                      {p.price && !p.outOfStock ? (
                        <Price value={p.price} className="text-4xl font-semibold" />
                      ) : (
                        <span className="font-board text-xl font-semibold text-signal">Rupture</span>
                      )}
                    </li>
                  );
                })}
              </ul>
              {current?.updatedAt && (
                <p className="mt-3 text-xs text-white/70">
                  {FUEL_LABELS[fuel]} mis à jour <UpdatedAt iso={current.updatedAt} />
                </p>
              )}
            </section>
          )}

          <section aria-labelledby="hours-title">
            <h2 id="hours-title" className="mb-2 font-semibold">
              Horaires
            </h2>
            {station.openingHours.automate24h && <p className="mb-2 text-sm">Automate 24 h/24</p>}
            {station.openingHours.days.length === 0 ? (
              <p className="text-sm text-muted">Horaires non renseignés par la station.</p>
            ) : (
              <table className="w-full text-sm">
                <tbody>
                  {station.openingHours.days.map((d) => (
                    <tr key={d.day} className="border-b border-line last:border-0">
                      <th scope="row" className="py-1.5 pr-3 text-left font-medium">
                        {d.name}
                      </th>
                      <td className="py-1.5 text-right tabular-nums text-muted">
                        {d.closed ? "Fermé" : d.slots.length ? d.slots.map((s) => `${s.open} - ${s.close}`).join(", ") : "Ouvert"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </section>

          {station.services.length > 0 && (
            <section aria-labelledby="services-title">
              <h2 id="services-title" className="mb-2 font-semibold">
                Services
              </h2>
              <ul className="space-y-1 text-sm text-muted">
                {station.services.map((s) => (
                  <li key={s}>{s}</li>
                ))}
              </ul>
            </section>
          )}
        </aside>
      </div>
    </div>
  );
}
