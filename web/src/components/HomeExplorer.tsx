"use client";

import { gql } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { useMemo, useState } from "react";
import { CitySearch, type Place } from "@/components/CitySearch";
import { FuelPicker } from "@/components/FuelPicker";
import { Price } from "@/components/Price";
import type { LatLng, MapView } from "@/components/StationsMap";
import { STATIONS_NEAR, type NearbyStation } from "@/graphql/queries";
import { useCompareSelection, MAX_COMPARED } from "@/lib/compare";
import { FUEL_LABELS, formatName, formatDistance, formatUpdatedAt, type Fuel } from "@/lib/format";

// Leaflet a besoin de window : la carte n'est rendue que dans le navigateur
const StationsMap = dynamic(() => import("@/components/StationsMap"), {
  ssr: false,
  loading: () => <div className="h-full w-full animate-pulse bg-line/40" />,
});

const QUERY = gql(STATIONS_NEAR);
const LYON: Place = { label: "Lyon", lat: 45.764, lng: 4.8357 };
// Autour d'une ville, 5 km suffisent en ville dense. Quand on dézoome puis
// "Chercher dans cette zone", le rayon suit la carte (borné par l'API à 50 km)
const DEFAULT_RADIUS_KM = 5;
const MAX_RADIUS_KM = 30;

type Data = { stationsNear: NearbyStation[] };
type Sort = "PRICE" | "DISTANCE";

// Distance approchée, suffisante pour quelques kilomètres
function distanceKm(a: LatLng, b: LatLng) {
  const dLat = (a.lat - b.lat) * 111;
  const dLng = (a.lng - b.lng) * 111 * Math.cos((a.lat * Math.PI) / 180);
  return Math.hypot(dLat, dLng);
}

function priceFor(station: NearbyStation, fuel: Fuel) {
  return station.currentPrices.find((p) => p.fuel === fuel);
}

export function HomeExplorer() {
  const [place, setPlace] = useState<Place>(LYON);
  const [fuel, setFuel] = useState<Fuel>("GAZOLE");
  const [sort, setSort] = useState<Sort>("PRICE");
  const [radiusKm, setRadiusKm] = useState(DEFAULT_RADIUS_KM);
  const [moved, setMoved] = useState<MapView | null>(null);
  const [fittedView, setFittedView] = useState<MapView | null>(null);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [geoError, setGeoError] = useState<string | null>(null);
  const compare = useCompareSelection();

  const { data, previousData, loading, error } = useQuery<Data>(QUERY, {
    variables: { lat: place.lat, lng: place.lng, radiusKm, fuel, sort, limit: 100 },
  });
  // Pendant le chargement on garde les stations précédentes au lieu de vider la carte
  const current = (data ?? previousData)?.stationsNear;
  const stations = useMemo(() => current ?? [], [current]);

  // La moins chère des stations affichées, mise en vert sur la carte
  const cheapest = useMemo(() => {
    let best: { station: NearbyStation; price: number } | null = null;
    for (const s of stations) {
      const p = priceFor(s, fuel);
      if (p?.price && !p.outOfStock && (!best || p.price < best.price)) {
        best = { station: s, price: p.price };
      }
    }
    return best;
  }, [stations, fuel]);

  function locate() {
    if (!navigator.geolocation) {
      setGeoError("Ton navigateur ne donne pas accès à la position. Cherche une ville à la place.");
      return;
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setGeoError(null);
        setMoved(null);
        setRadiusKm(DEFAULT_RADIUS_KM);
        setPlace({ label: "ta position", lat: pos.coords.latitude, lng: pos.coords.longitude });
      },
      () => setGeoError("Position refusée ou introuvable. Cherche une ville à la place."),
      { timeout: 10000 },
    );
  }

  // Le bouton "chercher ici" n'apparaît que si, depuis que la carte s'est calée
  // sur les stations, on l'a déplacée de plus de 1,5 km ou nettement dézoomée
  const reference = fittedView ?? { ...place, radiusKm };
  const showSearchHere =
    moved !== null && (distanceKm(moved, reference) > 1.5 || moved.radiusKm > 1.5 * reference.radiusKm);

  return (
    <div className="mx-auto max-w-7xl px-4 py-5">
      <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <h1 className="font-board text-3xl font-semibold leading-tight sm:text-4xl">
          {cheapest ? (
            <>
              {FUEL_LABELS[fuel]} le moins cher autour de {place.label} :{" "}
              <Price value={cheapest.price} /> à {formatName(cheapest.station.city)}
            </>
          ) : (
            <>Prix du {FUEL_LABELS[fuel].toLowerCase()} autour de {place.label}</>
          )}
        </h1>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <CitySearch
            onPick={(p) => {
              setMoved(null);
              setRadiusKm(DEFAULT_RADIUS_KM);
              setPlace(p);
            }}
          />
          <button
            type="button"
            onClick={locate}
            className="rounded border border-ink px-3 py-2 text-[15px] font-medium hover:bg-ink hover:text-white"
          >
            Autour de moi
          </button>
        </div>
      </div>

      <p className="-mt-2 mb-4 max-w-prose text-[15px] text-muted">
        Les prix viennent du flux officiel, mis à jour par les stations toute la journée. Clique sur une station pour
        voir son historique et la prévision des prochains jours (Auvergne-Rhône-Alpes seulement pour l&apos;instant).
      </p>

      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <FuelPicker value={fuel} onChange={setFuel} />
        <div className="flex items-center gap-2 text-sm">
          <span className="text-muted">Trier par</span>
          {(["PRICE", "DISTANCE"] as Sort[]).map((s) => (
            <button
              key={s}
              type="button"
              aria-pressed={sort === s}
              onClick={() => setSort(s)}
              className="rounded px-2 py-1 font-medium text-muted aria-pressed:bg-line aria-pressed:text-ink"
            >
              {s === "PRICE" ? "prix" : "distance"}
            </button>
          ))}
        </div>
      </div>
      {geoError && <p className="mb-3 text-sm text-signal-ink">{geoError}</p>}

      {/* grid-cols-1 = minmax(0, 1fr) : sans ça, sur mobile, la colonne
          s'élargit à la taille des adresses et la page défile de côté */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        <div className="relative h-[45vh] overflow-hidden rounded border border-line lg:h-[70vh]">
          <StationsMap
            center={place}
            placeKey={`${place.lat},${place.lng}`}
            ready={!loading && data !== undefined}
            stations={stations}
            fuel={fuel}
            cheapestId={cheapest?.station.id ?? null}
            activeId={activeId}
            onSelect={(id) => {
              setActiveId(id);
              document.getElementById(`station-${id}`)?.scrollIntoView({ block: "nearest", behavior: "smooth" });
            }}
            onMoved={setMoved}
            onFitted={setFittedView}
          />
          {showSearchHere && (
            <button
              type="button"
              onClick={() => {
                setPlace({ label: "cette zone", lat: moved.lat, lng: moved.lng });
                setRadiusKm(Math.min(Math.max(Math.round(moved.radiusKm), 2), MAX_RADIUS_KM));
                setMoved(null);
              }}
              className="absolute left-1/2 top-3 z-[500] -translate-x-1/2 rounded bg-ink px-3 py-2 text-sm font-medium text-white shadow"
            >
              Chercher dans cette zone
            </button>
          )}
        </div>

        <section aria-label="Stations" className="lg:h-[70vh] lg:overflow-y-auto">
          {error && (
            <p className="rounded border border-line bg-white p-4 text-[15px]">
              Le serveur ne répond pas. Il est hébergé gratuitement et s&apos;endort après
              15 minutes sans visite : recharge la page dans une minute, le temps qu&apos;il
              se réveille.
            </p>
          )}
          {!error && !loading && stations.length === 0 && (
            <p className="p-4 text-[15px] text-muted">
              Aucune station ne vend du {FUEL_LABELS[fuel]} à moins de {radiusKm} km. Essaie un autre carburant ou
              une autre ville.
            </p>
          )}
          <ul className="divide-y divide-line border-y border-line bg-white" aria-busy={loading}>
            {stations.map((s) => {
              const p = priceFor(s, fuel);
              const selected = compare.ids.includes(s.id);
              return (
                <li
                  key={s.id}
                  id={`station-${s.id}`}
                  onMouseEnter={() => setActiveId(s.id)}
                  className={`flex items-center gap-4 px-3 py-3 ${activeId === s.id ? "bg-paper" : ""}`}
                >
                  <div className="w-20 shrink-0 text-right">
                    {p?.price && !p.outOfStock ? (
                      <Price value={p.price} className={`text-2xl font-semibold ${s.id === cheapest?.station.id ? "text-route" : ""}`} />
                    ) : (
                      <span className="text-sm font-medium text-signal-ink">Rupture</span>
                    )}
                  </div>
                  <div className="min-w-0 flex-1">
                    <Link href={`/stations/${s.id}`} className="block truncate font-medium hover:underline">
                      {formatName(s.address)}
                    </Link>
                    <p className="text-sm text-muted">
                      {formatName(s.city)}
                      {s.distanceKm !== null && <>, à {formatDistance(s.distanceKm)}</>}
                      {s.roadType === "A" && <span className="ml-2 rounded-sm bg-autoroute px-1.5 py-px text-xs font-medium text-white">Autoroute</span>}
                    </p>
                    {p?.updatedAt && <p className="text-xs text-muted">Prix mis à jour {formatUpdatedAt(p.updatedAt)}</p>}
                  </div>
                  <label className="flex shrink-0 items-center gap-1.5 text-sm text-muted">
                    <input
                      type="checkbox"
                      checked={selected}
                      disabled={!selected && compare.full}
                      onChange={() => compare.toggle(s.id)}
                      className="size-4 accent-ink"
                    />
                    Comparer
                  </label>
                </li>
              );
            })}
          </ul>
        </section>
      </div>

      {compare.ids.length > 0 && (
        <div className="sticky bottom-3 z-[600] mt-4 flex justify-center">
          <Link
            href={`/compare?ids=${compare.ids.join(",")}&fuel=${fuel}`}
            className="rounded bg-ink px-4 py-2.5 font-medium text-white shadow-lg"
          >
            Comparer {compare.ids.length} station{compare.ids.length > 1 ? "s" : ""}
            {compare.full && ` (maximum ${MAX_COMPARED})`}
          </Link>
        </div>
      )}
    </div>
  );
}
