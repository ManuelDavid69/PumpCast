"use client";

// Recherche d'une ville ou d'un code postal avec le service de géocodage
// officiel (Géoplateforme de l'IGN, gratuit et sans clé).

import { useEffect, useId, useState } from "react";

export type Place = { label: string; lat: number; lng: number };

type Feature = {
  geometry: { coordinates: [number, number] };
  properties: { label: string; postcode?: string; context?: string };
};

const GEOCODER = "https://data.geopf.fr/geocodage/search";

export function CitySearch({ onPick }: { onPick: (place: Place) => void }) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Place[]>([]);
  const [error, setError] = useState<string | null>(null);
  const listId = useId();

  useEffect(() => {
    const q = query.trim();
    if (q.length < 2) {
      return;
    }
    // On attend que l'utilisateur arrête de taper avant d'appeler le service
    const controller = new AbortController();
    const timer = setTimeout(async () => {
      try {
        const url = `${GEOCODER}?q=${encodeURIComponent(q)}&type=municipality&limit=5`;
        const res = await fetch(url, { signal: controller.signal });
        const body = (await res.json()) as { features: Feature[] };
        setResults(
          body.features.map((f) => ({
            label: f.properties.postcode ? `${f.properties.label} (${f.properties.postcode})` : f.properties.label,
            lng: f.geometry.coordinates[0],
            lat: f.geometry.coordinates[1],
          })),
        );
        setError(null);
      } catch (e) {
        if ((e as Error).name !== "AbortError") {
          setError("La recherche de ville ne répond pas, réessaie dans un instant.");
        }
      }
    }, 250);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [query]);

  // Sous 2 caractères on n'affiche rien, sans avoir besoin de vider la liste
  const visible = query.trim().length < 2 ? [] : results;

  function pick(place: Place) {
    onPick(place);
    setQuery("");
    setResults([]);
  }

  return (
    <div className="relative w-full sm:w-72">
      <label htmlFor={`${listId}-input`} className="sr-only">
        Ville ou code postal
      </label>
      <input
        id={`${listId}-input`}
        type="search"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && visible[0]) {
            e.preventDefault();
            pick(visible[0]);
          }
        }}
        placeholder="Ville ou code postal"
        autoComplete="off"
        aria-controls={listId}
        className="w-full rounded border border-line bg-white px-3 py-2 text-[15px] placeholder:text-muted focus:border-autoroute focus:outline-none"
      />
      {(visible.length > 0 || error) && (
        <ul id={listId} className="absolute z-[1000] mt-1 w-full overflow-hidden rounded border border-line bg-white shadow-md">
          {error && <li className="px-3 py-2 text-sm text-muted">{error}</li>}
          {visible.map((r) => (
            <li key={r.label}>
              <button type="button" onClick={() => pick(r)} className="w-full px-3 py-2 text-left text-[15px] hover:bg-paper">
                {r.label}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
