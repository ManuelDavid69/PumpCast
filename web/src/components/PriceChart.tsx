"use client";

// Courbe des prix, une ligne par station. Sert à la fiche station (une
// seule courbe, avec la prévision en pointillé) et à la comparaison (jusqu'à 5).

import {
  Area,
  CartesianGrid,
  ComposedChart,
  Legend,
  Line,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { PricePoint } from "@/graphql/queries";
import { formatPrice, formatShortDate } from "@/lib/format";

export type Series = { id: string; label: string; color: string; points: PricePoint[] };

export type Forecast = {
  // Point de départ (dernier prix connu) pour que le pointillé prolonge la courbe
  start: PricePoint;
  points: { date: string; price: number; low: number; high: number }[];
};

type Row = Record<string, number | string | [number, number]>;

// Une ligne par date, une colonne par station, comme l'attend Recharts
function toRows(series: Series[], forecast?: Forecast) {
  const byDate = new Map<string, Row>();
  const row = (date: string) => {
    const r = byDate.get(date) ?? { date };
    byDate.set(date, r);
    return r;
  };
  for (const s of series) {
    for (const p of s.points) {
      row(p.date)[s.id] = p.price;
    }
  }
  if (forecast) {
    const start = row(forecast.start.date);
    start.forecast = forecast.start.price;
    start.band = [forecast.start.price, forecast.start.price];
    for (const p of forecast.points) {
      const r = row(p.date);
      r.forecast = p.price;
      r.band = [p.low, p.high];
    }
  }
  return [...byDate.values()].sort((a, b) => String(a.date).localeCompare(String(b.date)));
}

export function PriceChart({
  series,
  forecast,
  height = 300,
}: {
  series: Series[];
  forecast?: Forecast;
  height?: number;
}) {
  const rows = toRows(series, forecast);
  if (rows.length === 0) {
    return (
      <p className="flex items-center justify-center rounded border border-dashed border-line text-[15px] text-muted" style={{ height }}>
        Pas d&apos;historique pour ce carburant sur cette période.
      </p>
    );
  }
  const labelOf = (id: string) =>
    id === "forecast" ? "Prévision" : id === "band" ? "Fourchette" : (series.find((s) => s.id === id)?.label ?? id);

  return (
    <div style={{ height }} role="img" aria-label={`Évolution du prix sur ${rows.length} jours`}>
      <ResponsiveContainer width="100%" height="100%">
        <ComposedChart data={rows} margin={{ top: 8, right: 12, bottom: 0, left: 0 }}>
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
            formatter={(value, name) => {
              if (Array.isArray(value)) {
                return [`${formatPrice(Number(value[0]))} à ${formatPrice(Number(value[1]))}`, labelOf(String(name))];
              }
              return [formatPrice(Number(value)), labelOf(String(name))];
            }}
            contentStyle={{ border: "1px solid var(--color-line)", borderRadius: 4, fontSize: 13 }}
          />
          {series.length > 1 && <Legend formatter={(id) => labelOf(String(id))} wrapperStyle={{ fontSize: 13 }} />}
          {forecast && (
            <>
              <ReferenceLine x={forecast.start.date} stroke="var(--color-line)" />
              <Area dataKey="band" stroke="none" fill="var(--color-signal)" fillOpacity={0.18} isAnimationActive={false} />
              <Line
                dataKey="forecast"
                stroke="var(--color-ink)"
                strokeWidth={2}
                strokeDasharray="5 4"
                dot={false}
                isAnimationActive={false}
              />
            </>
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
        </ComposedChart>
      </ResponsiveContainer>
    </div>
  );
}
