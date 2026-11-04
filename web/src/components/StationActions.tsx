"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { ALERT_LABELS, accountQuery, type AlertType } from "@/lib/account";
import { FUEL_LABELS, type Fuel } from "@/lib/format";

type Props = {
  stationId: string;
  fuels: Fuel[];
  defaultFuel: Fuel;
  currentPrice: number | null;
  // Les alertes "baisse / hausse prévue" n'ont de sens que si la station a des prévisions
  hasForecast: boolean;
};

// Favori et alerte, sur la fiche station
export function StationActions({ stationId, fuels, defaultFuel, currentPrice, hasForecast }: Props) {
  const [state, setState] = useState<"loading" | "anonymous" | "ready">("loading");
  const [favorite, setFavorite] = useState(false);
  const [fuel, setFuel] = useState<Fuel>(defaultFuel);
  const [type, setType] = useState<AlertType>("THRESHOLD");
  // Seuil proposé : 3 centimes sous le prix actuel
  const [threshold, setThreshold] = useState(currentPrice ? (currentPrice - 0.03).toFixed(3) : "");
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    accountQuery<{ me: { favorites: { id: string }[] } | null }>(`{ me { favorites { id } } }`)
      .then((d) => {
        if (cancelled) return;
        setState(d.me ? "ready" : "anonymous");
        setFavorite(Boolean(d.me?.favorites.some((f) => f.id === stationId)));
      })
      .catch(() => !cancelled && setState("anonymous"));
    return () => {
      cancelled = true;
    };
  }, [stationId]);

  if (state === "loading") {
    return null;
  }
  if (state === "anonymous") {
    return (
      <p className="text-sm text-muted">
        <Link href="/compte" className="font-medium text-ink underline">
          Connecte-toi
        </Link>{" "}
        pour garder cette station en favori ou recevoir une alerte par email.
      </p>
    );
  }

  async function toggleFavorite() {
    setMessage(null);
    const mutation = favorite ? "removeFavorite" : "addFavorite";
    try {
      await accountQuery(`mutation($id: ID!) { ${mutation}(stationId: $id) { email } }`, { id: stationId });
      setFavorite(!favorite);
    } catch (err) {
      setMessage((err as Error).message);
    }
  }

  async function createAlert(e: React.FormEvent) {
    e.preventDefault();
    setMessage(null);
    const input: Record<string, unknown> = { stationId, fuel, type };
    if (type === "THRESHOLD") {
      input.threshold = Number(threshold.replace(",", "."));
    }
    try {
      await accountQuery(`mutation($input: AlertInput!) { createAlert(input: $input) { id } }`, { input });
      setMessage("Alerte créée. Tu recevras un email au moment où elle se déclenche.");
    } catch (err) {
      setMessage((err as Error).message);
    }
  }

  const types = (Object.keys(ALERT_LABELS) as AlertType[]).filter((t) => t === "THRESHOLD" || hasForecast);
  const field = "rounded border border-line bg-white px-2 py-1.5 text-sm";

  return (
    <div className="space-y-3">
      <button
        type="button"
        onClick={toggleFavorite}
        aria-pressed={favorite}
        className="rounded border border-ink px-3 py-1.5 text-sm font-medium aria-pressed:bg-ink aria-pressed:text-white"
      >
        {favorite ? "Dans tes favoris" : "Ajouter aux favoris"}
      </button>

      <form onSubmit={createAlert} className="flex flex-wrap items-end gap-2">
        <label className="text-sm">
          <span className="mb-1 block text-muted">M&apos;envoyer un email</span>
          <select value={type} onChange={(e) => setType(e.target.value as AlertType)} className={field}>
            {types.map((t) => (
              <option key={t} value={t}>
                {ALERT_LABELS[t]}
              </option>
            ))}
          </select>
        </label>
        <label className="text-sm">
          <span className="mb-1 block text-muted">Carburant</span>
          <select value={fuel} onChange={(e) => setFuel(e.target.value as Fuel)} className={field}>
            {fuels.map((f) => (
              <option key={f} value={f}>
                {FUEL_LABELS[f]}
              </option>
            ))}
          </select>
        </label>
        {type === "THRESHOLD" && (
          <label className="text-sm">
            <span className="mb-1 block text-muted">Seuil en €</span>
            <input
              inputMode="decimal"
              value={threshold}
              onChange={(e) => setThreshold(e.target.value)}
              className={`${field} w-24 tabular-nums`}
              required
            />
          </label>
        )}
        <button type="submit" className="rounded bg-ink px-3 py-1.5 text-sm font-medium text-white">
          Créer l&apos;alerte
        </button>
      </form>
      {message && <p className="text-sm text-muted">{message}</p>}
    </div>
  );
}
