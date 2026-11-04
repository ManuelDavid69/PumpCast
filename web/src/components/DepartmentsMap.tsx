"use client";

// Carte de chaleur des prix moyens par département. Plus c'est foncé, plus
// c'est cher. Les contours viennent de public/departements.geojson.

import "leaflet/dist/leaflet.css";
import type { Feature, FeatureCollection, Geometry } from "geojson";
import type { Layer, PathOptions } from "leaflet";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { GeoJSON, MapContainer } from "react-leaflet";
import type { DepartmentAverage } from "@/graphql/queries";
import { formatPrice } from "@/lib/format";

type Props = { averages: DepartmentAverage[]; classes: number[]; colors: string[] };
type DepProps = { code: string; nom: string };

export function colorFor(price: number, classes: number[], colors: string[]): string {
  const i = classes.findIndex((limit) => price <= limit);
  return colors[i === -1 ? colors.length - 1 : i];
}

export default function DepartmentsMap({ averages, classes, colors }: Props) {
  const router = useRouter();
  const [shapes, setShapes] = useState<FeatureCollection<Geometry, DepProps> | null>(null);
  const byCode = new Map(averages.map((a) => [a.department, a]));

  useEffect(() => {
    let cancelled = false;
    fetch("/departements.geojson")
      .then((r) => r.json())
      .then((data: FeatureCollection<Geometry, DepProps>) => {
        if (!cancelled) setShapes(data);
      })
      .catch(() => {
        // Sans contours la liste à côté de la carte reste utilisable
      });
    return () => {
      cancelled = true;
    };
  }, []);

  function style(feature?: Feature<Geometry, DepProps>): PathOptions {
    const avg = feature ? byCode.get(feature.properties.code) : undefined;
    return {
      weight: 0.6,
      color: "#ffffff",
      fillOpacity: 1,
      fillColor: avg ? colorFor(avg.avgPrice, classes, colors) : "#e5e7eb",
    };
  }

  function onEach(feature: Feature<Geometry, DepProps>, layer: Layer) {
    const avg = byCode.get(feature.properties.code);
    const text = avg
      ? `${feature.properties.nom} : ${formatPrice(avg.avgPrice)} (${avg.stationCount} stations)`
      : `${feature.properties.nom} : pas de données`;
    layer.bindTooltip(text, { sticky: true });
    layer.on("click", () => router.push(`/departements/${feature.properties.code}`));
  }

  return (
    <MapContainer
      center={[46.6, 2.4]}
      zoom={5.5}
      minZoom={5}
      maxZoom={8}
      zoomSnap={0.25}
      className="departments-map h-full w-full"
      attributionControl={false}
    >
      {shapes && (
        // key : on redessine si les moyennes changent (autre carburant)
        <GeoJSON key={averages.map((a) => a.avgPrice).join()} data={shapes} style={style} onEachFeature={onEach} />
      )}
    </MapContainer>
  );
}
