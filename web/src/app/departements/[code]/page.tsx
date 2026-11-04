// Statistiques d'un département : moyenne, écart, station la moins chère,
// évolution sur 30 jours.

import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Suspense } from "react";
import { PriceChart } from "@/components/PriceChart";
import { Price } from "@/components/Price";
import { getDepartmentStats } from "@/lib/api";
import { DEPARTMENT_NAMES, departmentName } from "@/lib/departments";
import { FUEL_LABELS, FUELS, formatCents, formatName, formatShortDate, type Fuel } from "@/lib/format";

export async function generateMetadata({ params }: PageProps<"/departements/[code]">): Promise<Metadata> {
  const { code } = await params;
  return { title: `Prix des carburants : ${departmentName(code.toUpperCase())}` };
}

export default function DepartmentPage(props: PageProps<"/departements/[code]">) {
  return (
    <div className="mx-auto max-w-7xl px-4 py-6">
      <Suspense fallback={<div className="h-96 animate-pulse rounded bg-line/40" />}>
        <DepartmentView {...props} />
      </Suspense>
    </div>
  );
}

async function DepartmentView({ params, searchParams }: PageProps<"/departements/[code]">) {
  const code = (await params).code.toUpperCase();
  if (!DEPARTMENT_NAMES[code]) {
    notFound();
  }
  const query = await searchParams;
  const wanted = typeof query.carburant === "string" ? (query.carburant.toUpperCase() as Fuel) : "GAZOLE";
  const fuel = FUELS.includes(wanted) ? wanted : "GAZOLE";
  const stats = await getDepartmentStats(code, fuel);
  const trend = stats?.trend30d ?? [];
  const cheapestPrice = stats?.cheapestStation?.currentPrices.find((p) => p.fuel === fuel)?.price;

  return (
    <>
      <p className="mb-1 text-sm text-muted">
        <Link href={`/departements?carburant=${fuel}`} className="hover:underline">
          Tous les départements
        </Link>
      </p>
      <h1 className="font-board text-3xl font-semibold sm:text-4xl">
        {departmentName(code)} <span className="text-muted">({code})</span>
      </h1>
      <nav aria-label="Carburant" className="mt-3 flex flex-wrap gap-1">
        {FUELS.map((f) => (
          <Link
            key={f}
            href={`/departements/${code}?carburant=${f}`}
            aria-current={f === fuel ? "page" : undefined}
            scroll={false}
            className="rounded border border-line bg-white px-2.5 py-1.5 text-sm font-medium text-muted aria-[current=page]:border-ink aria-[current=page]:bg-ink aria-[current=page]:text-white"
          >
            {FUEL_LABELS[f]}
          </Link>
        ))}
      </nav>

      {!stats ? (
        <p className="mt-6 text-[15px] text-muted">Aucune station de ce département ne vend du {FUEL_LABELS[fuel]}.</p>
      ) : (
        <div className="mt-6 grid gap-6 lg:grid-cols-[20rem_minmax(0,1fr)]">
          <section aria-label="Chiffres du jour" className="rounded bg-ink p-5 text-white">
            <p className="text-[15px]">Prix moyen le {formatShortDate(stats.date)}</p>
            <Price value={stats.avgPrice} className="mt-2 block text-5xl font-semibold leading-none" />
            <p className="mt-1 text-sm text-white/70">sur {stats.stationCount} stations</p>
            <dl className="mt-4 space-y-1 text-[15px]">
              <div className="flex justify-between">
                <dt className="text-white/70">Le moins cher</dt>
                <dd>
                  <Price value={stats.minPrice} />
                </dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-white/70">Le plus cher</dt>
                <dd>
                  <Price value={stats.maxPrice} />
                </dd>
              </div>
              {trend.length > 1 && (
                <div className="flex justify-between">
                  <dt className="text-white/70">Sur 30 jours</dt>
                  <dd className="tabular-nums">{formatCents(trend[trend.length - 1].price - trend[0].price)}</dd>
                </div>
              )}
            </dl>
            {stats.cheapestStation && (
              <p className="mt-4 border-t border-white/15 pt-3 text-[15px]">
                Station la moins chère :{" "}
                <Link href={`/stations/${stats.cheapestStation.id}?carburant=${fuel}`} className="font-medium underline">
                  {formatName(stats.cheapestStation.address)}, {formatName(stats.cheapestStation.city)}
                </Link>
                {cheapestPrice && (
                  <>
                    {" "}
                    à <Price value={cheapestPrice} />
                  </>
                )}
              </p>
            )}
          </section>

          <section aria-labelledby="trend-title">
            <h2 id="trend-title" className="mb-3 text-lg font-semibold">
              Prix moyen sur les 30 derniers jours
            </h2>
            <div className="rounded border border-line bg-white p-3">
              <PriceChart series={[{ id: "avg", label: "Prix moyen", color: "var(--color-autoroute)", points: trend }]} />
            </div>
          </section>
        </div>
      )}
    </>
  );
}
