"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { Price } from "@/components/Price";
import { ALERT_LABELS, accountQuery, authAction, type AlertType } from "@/lib/account";
import { FUEL_LABELS, formatName, type Fuel } from "@/lib/format";

type Me = {
  email: string;
  favorites: { id: string; address: string; city: string; currentPrices: { fuel: Fuel; price: number | null; outOfStock: boolean }[] }[];
  alerts: {
    id: string;
    fuel: Fuel;
    type: AlertType;
    threshold: number | null;
    lastSentAt: string | null;
    station: { id: string; address: string; city: string } | null;
  }[];
};

const ME = `{ me {
  email
  favorites { id address city currentPrices { fuel price outOfStock } }
  alerts { id fuel type threshold lastSentAt station { id address city } }
} }`;

const inputClass =
  "w-full rounded border border-line bg-white px-3 py-2 text-[15px] focus:border-autoroute focus:outline-none";

function AuthForms({ onDone }: { onDone: () => void }) {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await authAction(mode, email, password);
      onDone();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="max-w-sm">
      <div className="mb-4 flex gap-1" role="tablist">
        {(["login", "register"] as const).map((m) => (
          <button
            key={m}
            type="button"
            role="tab"
            aria-selected={mode === m}
            onClick={() => setMode(m)}
            className="rounded px-3 py-1.5 font-medium text-muted aria-selected:bg-ink aria-selected:text-white"
          >
            {m === "login" ? "Se connecter" : "Créer un compte"}
          </button>
        ))}
      </div>
      <form onSubmit={submit} className="space-y-3">
        <label className="block">
          <span className="mb-1 block text-sm font-medium">Email</span>
          <input type="email" required autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} className={inputClass} />
        </label>
        <label className="block">
          <span className="mb-1 block text-sm font-medium">Mot de passe</span>
          <input
            type="password"
            required
            minLength={mode === "register" ? 10 : undefined}
            autoComplete={mode === "register" ? "new-password" : "current-password"}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className={inputClass}
          />
          {mode === "register" && <span className="mt-1 block text-sm text-muted">10 caractères minimum.</span>}
        </label>
        {error && <p className="text-sm text-signal-ink">{error}</p>}
        <button type="submit" disabled={busy} className="rounded bg-ink px-4 py-2 font-medium text-white disabled:opacity-60">
          {mode === "login" ? "Se connecter" : "Créer mon compte"}
        </button>
      </form>
      <p className="mt-6 text-sm leading-relaxed text-muted">
        Un compte sert à garder tes stations favorites et à recevoir des alertes par email. Ton email ne sert à rien
        d&apos;autre, et tu peux supprimer ton compte quand tu veux.
      </p>
    </div>
  );
}

function alertText(a: Me["alerts"][number]): string {
  const fuel = FUEL_LABELS[a.fuel];
  if (a.type === "THRESHOLD" && a.threshold !== null) {
    return `${fuel} sous ${a.threshold.toFixed(3).replace(".", ",")} €`;
  }
  return `${fuel} : ${ALERT_LABELS[a.type].toLowerCase()}`;
}

export function AccountClient() {
  const [me, setMe] = useState<Me | null | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);
  const [deletePassword, setDeletePassword] = useState("");

  const load = useCallback(async () => {
    try {
      const data = await accountQuery<{ me: Me | null }>(ME);
      setMe(data.me);
    } catch (err) {
      setError((err as Error).message);
      setMe(null);
    }
  }, []);

  // Premier chargement : l'état est mis à jour dans le .then, pas directement dans l'effet
  useEffect(() => {
    let cancelled = false;
    accountQuery<{ me: Me | null }>(ME)
      .then((data) => !cancelled && setMe(data.me))
      .catch((err: Error) => {
        if (!cancelled) {
          setError(err.message);
          setMe(null);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function run(query: string, variables: Record<string, unknown>) {
    setError(null);
    try {
      await accountQuery(query, variables);
      await load();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  if (me === undefined) {
    return <div className="h-40 animate-pulse rounded bg-line/40" />;
  }
  if (me === null) {
    return <AuthForms onDone={load} />;
  }

  return (
    <div className="space-y-10">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-[15px]">
          Connecté avec <strong>{me.email}</strong>
        </p>
        <button
          type="button"
          onClick={async () => {
            await authAction("logout");
            setMe(null);
          }}
          className="rounded border border-ink px-3 py-1.5 text-sm font-medium hover:bg-ink hover:text-white"
        >
          Se déconnecter
        </button>
      </div>
      {error && <p className="text-sm text-signal-ink">{error}</p>}

      <section aria-labelledby="fav-title">
        <h2 id="fav-title" className="text-xl font-semibold">
          Stations favorites
        </h2>
        {me.favorites.length === 0 ? (
          <p className="mt-2 text-[15px] text-muted">
            Aucune pour l&apos;instant. Ajoute une station depuis sa fiche, avec le bouton « Ajouter aux favoris ».
          </p>
        ) : (
          <ul className="mt-3 divide-y divide-line border-y border-line bg-white">
            {me.favorites.map((s) => (
              <li key={s.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-3 py-3">
                <Link href={`/stations/${s.id}`} className="min-w-0 flex-1 font-medium hover:underline">
                  {formatName(s.address)}, <span className="font-normal text-muted">{formatName(s.city)}</span>
                </Link>
                <span className="flex gap-3 text-sm text-muted">
                  {s.currentPrices
                    .filter((p) => p.price && !p.outOfStock)
                    .slice(0, 3)
                    .map((p) => (
                      <span key={p.fuel}>
                        {FUEL_LABELS[p.fuel]} <Price value={p.price as number} className="font-semibold text-ink" />
                      </span>
                    ))}
                </span>
                <button
                  type="button"
                  onClick={() => run(`mutation($id: ID!) { removeFavorite(stationId: $id) { email } }`, { id: s.id })}
                  className="text-sm text-muted underline hover:text-ink"
                >
                  Retirer
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section aria-labelledby="alerts-title">
        <h2 id="alerts-title" className="text-xl font-semibold">
          Alertes
        </h2>
        {me.alerts.length === 0 ? (
          <p className="mt-2 text-[15px] text-muted">
            Aucune alerte. Sur la fiche d&apos;une station, tu peux demander un email quand le prix passe sous un seuil, ou
            quand le modèle prévoit une baisse ou une hausse.
          </p>
        ) : (
          <ul className="mt-3 divide-y divide-line border-y border-line bg-white">
            {me.alerts.map((a) => (
              <li key={a.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-3 py-3">
                <span className="min-w-0 flex-1">
                  <span className="font-medium">{alertText(a)}</span>
                  {a.station ? (
                    <Link href={`/stations/${a.station.id}`} className="block text-sm text-muted hover:underline">
                      {formatName(a.station.address)}, {formatName(a.station.city)}
                    </Link>
                  ) : (
                    <span className="block text-sm text-muted">Station retirée du flux officiel</span>
                  )}
                </span>
                <span className="text-sm text-muted">
                  {a.lastSentAt ? `Dernier email le ${new Date(a.lastSentAt).toLocaleDateString("fr-FR")}` : "Pas encore d'email"}
                </span>
                <button
                  type="button"
                  onClick={() => run(`mutation($id: ID!) { deleteAlert(id: $id) }`, { id: a.id })}
                  className="text-sm text-muted underline hover:text-ink"
                >
                  Supprimer
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section aria-labelledby="delete-title" className="border-t border-line pt-6">
        <h2 id="delete-title" className="font-semibold">
          Supprimer mon compte
        </h2>
        <p className="mt-1 max-w-prose text-sm text-muted">
          Ton compte, tes favoris et tes alertes sont effacés tout de suite. Ton mot de passe est demandé pour confirmer.
        </p>
        <form
          className="mt-3 flex max-w-md flex-wrap gap-2"
          onSubmit={async (e) => {
            e.preventDefault();
            try {
              await accountQuery(`mutation($p: String!) { deleteAccount(password: $p) }`, { p: deletePassword });
              await authAction("logout");
              setMe(null);
            } catch (err) {
              setError((err as Error).message);
            }
          }}
        >
          <label className="min-w-0 flex-1">
            <span className="sr-only">Mot de passe</span>
            <input
              type="password"
              required
              autoComplete="current-password"
              placeholder="Mot de passe"
              value={deletePassword}
              onChange={(e) => setDeletePassword(e.target.value)}
              className={inputClass}
            />
          </label>
          <button type="submit" className="rounded border border-signal-ink px-3 py-2 text-sm font-medium text-signal-ink">
            Supprimer mon compte
          </button>
        </form>
      </section>
    </div>
  );
}
