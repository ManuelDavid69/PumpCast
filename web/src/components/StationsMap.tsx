"use client";

// Carte Leaflet des stations. Chaque station est une étiquette de prix
// plutôt qu'une punaise : c'est l'information que l'utilisateur cherche.
// Chargé seulement dans le navigateur (Leaflet a besoin de window).

import "leaflet/dist/leaflet.css";
import L from "leaflet";
import { useEffect, useMemo, useRef } from "react";
import { MapContainer, Marker, TileLayer, useMap, useMapEvents } from "react-leaflet";
import type { NearbyStation } from "@/graphql/queries";
import { splitPrice, type Fuel } from "@/lib/format";

export type LatLng = { lat: number; lng: number };
// Centre de la carte et rayon qui couvre toute la zone visible
export type MapView = LatLng & { radiusKm: number };

type Props = {
  center: LatLng;
  // Change quand l'utilisateur choisit un nouveau lieu : la carte se cale alors sur les stations trouvées
  placeKey: string;
  // false pendant le chargement : les stations affichées sont encore celles de l'ancien lieu
  ready: boolean;
  stations: NearbyStation[];
  fuel: Fuel;
  cheapestId: string | null;
  activeId: string | null;
  onSelect: (id: string) => void;
  onMoved: (view: MapView) => void;
  // Vue de la carte juste après s'être calée sur les stations : sert de
  // référence pour savoir si l'utilisateur a vraiment bougé ou dézoomé
  onFitted: (view: MapView) => void;
};

function viewOf(map: L.Map): MapView {
  const c = map.getCenter();
  // Du centre au coin de la carte : le cercle de recherche couvre tout ce qu'on voit
  return { lat: c.lat, lng: c.lng, radiusKm: c.distanceTo(map.getBounds().getNorthEast()) / 1000 };
}

function priceIcon(station: NearbyStation, fuel: Fuel, cheapest: boolean, active: boolean) {
  const current = station.currentPrices.find((p) => p.fuel === fuel);
  let label = "rupture";
  if (current?.price && !current.outOfStock) {
    const { main, thousandth } = splitPrice(current.price);
    label = `${main}<small>${thousandth}</small>`;
  }
  return L.divIcon({
    className: "price-tag-wrap",
    html: `<span class="price-tag" data-road="${station.roadType}" data-cheapest="${cheapest}" data-active="${active}">${label}</span>`,
    iconSize: [0, 0],
  });
}

// Cale la carte sur les stations trouvées quand l'utilisateur choisit un
// nouveau lieu (une seule fois par lieu, pour ne pas gêner s'il déplace la carte)
function FitToStations({
  center,
  placeKey,
  ready,
  stations,
  onFitted,
}: {
  center: LatLng;
  placeKey: string;
  ready: boolean;
  stations: NearbyStation[];
  onFitted: (view: MapView) => void;
}) {
  const map = useMap();
  const fitted = useRef<string | null>(null);

  // Leaflet ne voit pas quand son conteneur change de taille (chargement de la
  // page, rotation du téléphone...) : sans ça il ne charge les tuiles que sur
  // une partie de la carte. On le prévient à chaque changement.
  useEffect(() => {
    const observer = new ResizeObserver(() => map.invalidateSize());
    observer.observe(map.getContainer());
    return () => observer.disconnect();
  }, [map]);

  useEffect(() => {
    if (fitted.current === placeKey || !ready) {
      return;
    }
    // Sans animation : la vue est à jour tout de suite pour onFitted
    if (stations.length === 0) {
      map.setView([center.lat, center.lng], 13, { animate: false });
    } else {
      map.invalidateSize();
      const bounds = L.latLngBounds(stations.map((s) => [s.latitude as number, s.longitude as number]));
      map.fitBounds(bounds.extend([center.lat, center.lng]), { padding: [30, 30], maxZoom: 15, animate: false });
    }
    fitted.current = placeKey;
    onFitted(viewOf(map));
  }, [placeKey, ready, stations, center.lat, center.lng, map, onFitted]);
  return null;
}

function MoveWatcher({ onMoved }: { onMoved: (view: MapView) => void }) {
  useMapEvents({
    // moveend part aussi après un zoom
    moveend: (e) => onMoved(viewOf(e.target as L.Map)),
  });
  return null;
}

export default function StationsMap({ center, placeKey, ready, stations, fuel, cheapestId, activeId, onSelect, onMoved, onFitted }: Props) {
  const located = useMemo(
    () => stations.filter((s) => s.latitude !== null && s.longitude !== null),
    [stations],
  );

  return (
    <MapContainer center={[center.lat, center.lng]} zoom={13} className="h-full w-full" scrollWheelZoom>
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <FitToStations center={center} placeKey={placeKey} ready={ready} stations={located} onFitted={onFitted} />
      <MoveWatcher onMoved={onMoved} />
      {located.map((s) => (
        <Marker
          key={s.id}
          position={[s.latitude as number, s.longitude as number]}
          icon={priceIcon(s, fuel, s.id === cheapestId, s.id === activeId)}
          zIndexOffset={s.id === activeId ? 1000 : s.id === cheapestId ? 500 : 0}
          eventHandlers={{ click: () => onSelect(s.id) }}
          title={`${s.address}, ${s.city}`}
        />
      ))}
    </MapContainer>
  );
}
