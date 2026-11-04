// Données lues par les pages rendues côté serveur.
//
// 'use cache' garde le résultat quelques minutes (profil "minutes") :
// les prix changent au plus toutes les 10 minutes dans le flux officiel,
// inutile de rappeler l'API à chaque visite.

import { cacheLife } from "next/cache";
import {
  COMPARE,
  DEPARTMENT_AVERAGES,
  DEPARTMENT_STATS,
  STATION,
  STATION_HISTORY,
  type ComparedStation,
  type DepartmentAverage,
  type DepartmentStats,
  type PricePoint,
  type Prediction,
  type StationDetail,
} from "@/graphql/queries";
import type { Fuel } from "@/lib/format";
import { gqlFetch } from "@/lib/graphql-server";

export async function getStation(id: string): Promise<StationDetail | null> {
  "use cache";
  cacheLife("minutes");
  const data = await gqlFetch<{ station: StationDetail | null }>(STATION, { id });
  return data.station;
}

export async function getStationHistory(
  id: string,
  fuel: Fuel,
  days: number,
): Promise<{ history: PricePoint[]; prediction: Prediction | null }> {
  "use cache";
  cacheLife("minutes");
  const data = await gqlFetch<{ station: { priceHistory: PricePoint[]; prediction: Prediction | null } | null }>(
    STATION_HISTORY,
    { id, fuel, days },
  );
  return { history: data.station?.priceHistory ?? [], prediction: data.station?.prediction ?? null };
}

export async function getDepartmentStats(department: string, fuel: Fuel): Promise<DepartmentStats | null> {
  "use cache";
  cacheLife("minutes");
  const data = await gqlFetch<{ departmentStats: DepartmentStats | null }>(DEPARTMENT_STATS, { department, fuel });
  return data.departmentStats;
}

export async function getDepartmentAverages(fuel: Fuel): Promise<DepartmentAverage[]> {
  "use cache";
  cacheLife("minutes");
  const data = await gqlFetch<{ departmentAverages: DepartmentAverage[] }>(DEPARTMENT_AVERAGES, { fuel });
  return data.departmentAverages;
}

export async function getCompared(ids: string[], fuel: Fuel, days: number): Promise<ComparedStation[]> {
  "use cache";
  cacheLife("minutes");
  const data = await gqlFetch<{ compareStations: ComparedStation[] }>(COMPARE, { ids, fuel, days });
  return data.compareStations;
}
