// Comparaison de stations, rendue côté serveur. Les stations viennent de
// l'URL (?ids=a,b,c&fuel=GAZOLE), choisies en cochant "Comparer" sur la carte.

import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { PriceChart } from "@/components/PriceChart";
import { Price } from "@/components/Price";
import { getCompared } from "@/lib/api";
import { FUEL_LABELS, FUELS, formatCents, formatName, type Fuel } from "@/lib/format";

export const metadata: Metadata = { title: "Comparer des stations" };

// Couleurs des courbes : distinctes entre elles et lisibles sur fond clair
const COLORS = ["#1b2430", "#1d4f91", "#1e7b4a", "#c28410", "#b5473a"];
const PERIODS = [30, 90, 365];

export default function ComparePage(props: PageProps<"/compare">) {
  return (
    <div className="mx-auto max-w-7xl px-4 py-6">
      <h1 className="font-board text-3xl font-semibold sm:text-4xl">Comparer des stations</h1>
      <Suspense fallback={<div className="mt-6 h-80 animate-pulse rounded bg-line/40" />}>
        <CompareView {...props} />
      </Suspense>
    </div>
  );
}

async function CompareView({ searchParams }: PageProps<"/compare">) {
  const query = await searchParams;
  const ids = typeof query.ids === "string" ? query.ids.split(",").filter(Boolean).slice(0, 5) : [];
  const fuelParam = typeof query.fuel === "string" ? (query.fuel.toUpperCase() as Fuel) : "GAZOLE";
  const fuel = FUELS.includes(fuelParam) ? fuelParam : "GAZOLE";
  const daysParam = Number(query.jours);
  const days = PERIODS.includes(daysParam) ? daysParam : 90;

  if (ids.length === 0) {
    return (
      <p className="mt-4 max-w-prose text-[15px] leading-relaxed text-muted">
        Coche « Comparer » sur les stations de la{" "}
        <Link href="/" className="font-medium text-ink underline">
          carte
        </Link>{" "}
        (jusqu&apos;à 5), puis reviens ici pour voir leurs prix sur un même graphique.
      </p>
    );
  }

  const stations = await getCompared(ids, fuel, days);
  const href = (f: Fuel, d: number) => `/compare?ids=${ids.join(",")}&fuel=${f}&jours=${d}`;
  const series = stations.map((s, i) => ({
    id: s.id,
    label: `${formatName(s.address)}, ${formatName(s.city)}`,
    color: COLORS[i % COLORS.length],
    points: s.priceHistory,
  }));

  return (
    <>
      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <nav aria-label="Carburant" className="flex flex-wrap gap-1">
          {FUELS.map((f) => (
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
        <nav aria-label="Période" className="flex gap-1 text-sm">
          {PERIODS.map((d) => (
            <Link
              key={d}
              href={href(fuel, d)}
              aria-current={d === days ? "page" : undefined}
              scroll={false}
              className="rounded px-2 py-1 font-medium text-muted aria-[current=page]:bg-line aria-[current=page]:text-ink"
            >
              {d === 365 ? "1 an" : `${d} jours`}
            </Link>
          ))}
        </nav>
      </div>

      <div className="mt-4 rounded border border-line bg-white p-3">
        <PriceChart series={series} height={360} />
      </div>

      <table className="mt-6 w-full text-left text-[15px]">
        <caption className="sr-only">Prix actuels et évolution sur la période</caption>
        <thead className="border-b border-line text-sm text-muted">
          <tr>
            <th scope="col" className="py-2 font-medium">Station</th>
            <th scope="col" className="py-2 text-right font-medium">Prix actuel</th>
            <th scope="col" className="hidden py-2 text-right font-medium sm:table-cell">Sur la période</th>
          </tr>
        </thead>
        <tbody>
          {stations.map((s, i) => {
            const current = s.currentPrices.find((p) => p.fuel === fuel);
            const first = s.priceHistory[0]?.price;
            const last = s.priceHistory.at(-1)?.price;
            return (
              <tr key={s.id} className="border-b border-line">
                <td className="py-2.5">
                  <span className="mr-2 inline-block size-3 rounded-full align-middle" style={{ background: COLORS[i % COLORS.length] }} />
                  <Link href={`/stations/${s.id}`} className="font-medium hover:underline">
                    {formatName(s.address)}
                  </Link>
                  <span className="text-muted">, {formatName(s.city)}</span>
                </td>
                <td className="py-2.5 text-right">
                  {current?.price && !current.outOfStock ? (
                    <Price value={current.price} className="text-xl font-semibold" />
                  ) : (
                    <span className="text-sm text-muted">{current?.outOfStock ? "Rupture" : "Non vendu"}</span>
                  )}
                </td>
                <td className="hidden py-2.5 text-right tabular-nums text-muted sm:table-cell">
                  {first !== undefined && last !== undefined ? formatCents(last - first) : "-"}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
      {stations.length < ids.length && (
        <p className="mt-3 text-sm text-muted">Certaines stations demandées n&apos;existent plus et ont été retirées.</p>
      )}
    </>
  );
}
