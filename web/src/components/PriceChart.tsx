"use client";

// Courbe des prix, une ligne par station. Sert à la fiche station (une
// seule courbe) et à la comparaison (jusqu'à 5). La prévision viendra se
// placer en pointillé dans la continuité de la courbe (phase 5).

import { CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { PricePoint } from "@/graphql/queries";
import { formatPrice, formatShortDate } from "@/lib/format";

export type Series = { id: string; label: string; color: string; points: PricePoint[] };

// Une ligne par date, une colonne par station, comme l'attend Recharts
function toRows(series: Series[]) {
  const byDate = new Map<string, Record<string, number | string>>();
  for (const s of series) {
    for (const p of s.points) {
      const row = byDate.get(p.date) ?? { date: p.date };
      row[s.id] = p.price;
      byDate.set(p.date, row);
    }
  }
  return [...byDate.values()].sort((a, b) => String(a.date).localeCompare(String(b.date)));
}

export function PriceChart({ series, height = 300 }: { series: Series[]; height?: number }) {
  const rows = toRows(series);
  if (rows.length === 0) {
    return (
      <p className="flex items-center justify-center rounded border border-dashed border-line text-[15px] text-muted" style={{ height }}>
        Pas d&apos;historique pour ce carburant sur cette période.
      </p>
    );
  }

  return (
    <div style={{ height }} role="img" aria-label={`Évolution du prix sur ${rows.length} jours`}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={rows} margin={{ top: 8, right: 12, bottom: 0, left: 0 }}>
          <CartesianGrid stroke="var(--color-line)" vertical={false} />
          <XAxis
            dataKey="date"
            tickFormatter={formatShortDate}
            tick={{ fontSize: 12, fill: "var(--color-muted)" }}
            tickLine={false}
            axisLine={{ stroke: "var(--color-line)" }}
            minTickGap={40}
          />
          <YAxis
            domain={["auto", "auto"]}
            tickFormatter={(v: number) => v.toFixed(2).replace(".", ",")}
            tick={{ fontSize: 12, fill: "var(--color-muted)" }}
            tickLine={false}
            axisLine={false}
            width={44}
          />
          <Tooltip
            labelFormatter={(d) => formatShortDate(String(d))}
            formatter={(value, name) => [formatPrice(Number(value)), series.find((s) => s.id === name)?.label ?? name]}
            contentStyle={{ border: "1px solid var(--color-line)", borderRadius: 4, fontSize: 13 }}
          />
          {series.length > 1 && (
            <Legend formatter={(id) => series.find((s) => s.id === id)?.label ?? id} wrapperStyle={{ fontSize: 13 }} />
          )}
          {series.map((s) => (
            <Line
              key={s.id}
              dataKey={s.id}
              type="stepAfter"
              stroke={s.color}
              strokeWidth={2}
              dot={false}
              connectNulls={false}
              isAnimationActive={false}
            />
          ))}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
