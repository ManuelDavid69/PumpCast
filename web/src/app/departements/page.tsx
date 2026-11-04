// Carte de chaleur des prix moyens par département (toute la France).

import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { DepartmentsMapLoader } from "@/components/DepartmentsMapLoader";
import { Price } from "@/components/Price";
import { getDepartmentAverages } from "@/lib/api";
import { departmentName } from "@/lib/departments";
import { FUEL_LABELS, FUELS, type Fuel } from "@/lib/format";

export const metadata: Metadata = { title: "Prix moyens par département" };

// Du moins cher (clair) au plus cher (foncé), dans le bleu des panneaux
const COLORS = ["#e3ebf5", "#b6cae4", "#7d9fcb", "#46719f", "#1d4f91"];

// Bornes des 5 classes : chaque classe contient environ un cinquième des départements
function quintiles(values: number[]): number[] {
  const sorted = [...values].sort((a, b) => a - b);
  return [0.2, 0.4, 0.6, 0.8, 1].map((q) => sorted[Math.min(sorted.length - 1, Math.ceil(q * sorted.length) - 1)]);
}

export default function DepartmentsPage(props: PageProps<"/departements">) {
  return (
    <div className="mx-auto max-w-7xl px-4 py-6">
      <Suspense fallback={<div className="h-[70vh] animate-pulse rounded bg-line/40" />}>
        <DepartmentsView {...props} />
      </Suspense>
    </div>
  );
}

async function DepartmentsView({ searchParams }: PageProps<"/departements">) {
  const query = await searchParams;
  const wanted = typeof query.carburant === "string" ? (query.carburant.toUpperCase() as Fuel) : "GAZOLE";
  const fuel = FUELS.includes(wanted) ? wanted : "GAZOLE";
  const averages = await getDepartmentAverages(fuel);
  const sorted = [...averages].sort((a, b) => a.avgPrice - b.avgPrice);
  const classes = averages.length ? quintiles(averages.map((a) => a.avgPrice)) : [];

  return (
    <>
      <h1 className="font-board text-3xl font-semibold sm:text-4xl">Prix moyen du {FUEL_LABELS[fuel]} par département</h1>
      <nav aria-label="Carburant" className="mt-3 flex flex-wrap gap-1">
        {FUELS.map((f) => (
          <Link
            key={f}
            href={`/departements?carburant=${f}`}
            aria-current={f === fuel ? "page" : undefined}
            className="rounded border border-line bg-white px-2.5 py-1.5 text-sm font-medium text-muted aria-[current=page]:border-ink aria-[current=page]:bg-ink aria-[current=page]:text-white"
          >
            {FUEL_LABELS[f]}
          </Link>
        ))}
      </nav>

      {averages.length === 0 ? (
        <p className="mt-6 text-[15px] text-muted">Pas encore de moyennes pour ce carburant.</p>
      ) : (
        <div className="mt-4 grid gap-6 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
          <div>
            <div className="h-[60vh] overflow-hidden rounded border border-line lg:h-[70vh]">
              <DepartmentsMapLoader averages={averages} classes={classes} colors={COLORS} />
            </div>
            <ol className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm text-muted" aria-label="Légende">
              {classes.map((limit, i) => (
                <li key={`${i}-${limit}`} className="flex items-center gap-1.5">
                  <span className="inline-block h-3 w-5 rounded-sm" style={{ background: COLORS[i] }} />
                  jusqu&apos;à {limit.toFixed(3).replace(".", ",")} €
                </li>
              ))}
            </ol>
            <p className="mt-1 text-sm text-muted">
              Moyenne du dernier jour connu. L&apos;outre-mer n&apos;est pas dans le flux officiel des prix.
            </p>
          </div>

          <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-1">
            {[
              { title: "Les moins chers", list: sorted.slice(0, 5) },
              { title: "Les plus chers", list: sorted.slice(-5).reverse() },
            ].map((block) => (
              <section key={block.title} aria-label={block.title}>
                <h2 className="mb-2 font-semibold">{block.title}</h2>
                <ol className="divide-y divide-line border-y border-line bg-white">
                  {block.list.map((d) => (
                    <li key={d.department}>
                      <Link
                        href={`/departements/${d.department}?carburant=${fuel}`}
                        className="flex items-baseline justify-between gap-3 px-3 py-2 hover:bg-paper"
                      >
                        <span>
                          {departmentName(d.department)} <span className="text-sm text-muted">({d.department})</span>
                        </span>
                        <Price value={d.avgPrice} className="text-xl font-semibold" />
                      </Link>
                    </li>
                  ))}
                </ol>
              </section>
            ))}
          </div>
        </div>
      )}
    </>
  );
}
