// Requêtes GraphQL du front et types des réponses.
// Chaque page ne demande que les champs qu'elle affiche.

import type { Fuel } from "@/lib/format";

export type CurrentPrice = {
  fuel: Fuel;
  price: number | null;
  updatedAt: string | null;
  outOfStock: boolean;
};

export type PricePoint = { date: string; price: number };

export type NearbyStation = {
  id: string;
  address: string;
  city: string;
  latitude: number | null;
  longitude: number | null;
  roadType: string;
  distanceKm: number | null;
  currentPrices: CurrentPrice[];
};

// Accueil : stations autour d'un point, sans historique (inutile sur la carte)
export const STATIONS_NEAR = /* GraphQL */ `
  query StationsNear($lat: Float!, $lng: Float!, $radiusKm: Float, $fuel: Fuel, $sort: StationSort, $limit: Int) {
    stationsNear(lat: $lat, lng: $lng, radiusKm: $radiusKm, fuel: $fuel, sort: $sort, limit: $limit) {
      id
      address
      city
      latitude
      longitude
      roadType
      distanceKm
      currentPrices { fuel price updatedAt outOfStock }
    }
  }
`;

export type StationDetail = {
  id: string;
  address: string;
  city: string;
  postalCode: string;
  department: string;
  latitude: number | null;
  longitude: number | null;
  roadType: string;
  services: string[];
  openingHours: {
    automate24h: boolean;
    days: { day: number; name: string; closed: boolean; slots: { open: string; close: string }[] }[];
  };
  currentPrices: CurrentPrice[];
};

// Fiche station : tout sauf l'historique, demandé à part pour le carburant choisi
export const STATION = /* GraphQL */ `
  query Station($id: ID!) {
    station(id: $id) {
      id
      address
      city
      postalCode
      department
      latitude
      longitude
      roadType
      services
      openingHours { automate24h days { day name closed slots { open close } } }
      currentPrices { fuel price updatedAt outOfStock }
    }
  }
`;

export const STATION_HISTORY = /* GraphQL */ `
  query StationHistory($id: ID!, $fuel: Fuel!, $days: Int) {
    station(id: $id) {
      id
      priceHistory(fuel: $fuel, days: $days) { date price }
      prediction(fuel: $fuel) {
        baseDate
        basePrice
        generatedAt
        recommendation
        confidence
        reasons
        horizon { date price low high }
      }
    }
  }
`;

export type Prediction = {
  baseDate: string;
  basePrice: number;
  generatedAt: string;
  // null : pas assez fiable aujourd'hui, l'appli ne conseille rien
  recommendation: "FILL_NOW" | "WAIT" | "STABLE" | null;
  confidence: number;
  reasons: string[];
  horizon: { date: string; price: number; low: number; high: number }[];
};

export type ComparedStation = {
  id: string;
  address: string;
  city: string;
  roadType: string;
  currentPrices: CurrentPrice[];
  priceHistory: PricePoint[];
};

// Comparaison : plusieurs stations et leurs courbes en une seule requête
export const COMPARE = /* GraphQL */ `
  query Compare($ids: [ID!]!, $fuel: Fuel!, $days: Int) {
    compareStations(ids: $ids) {
      id
      address
      city
      roadType
      currentPrices { fuel price updatedAt outOfStock }
      priceHistory(fuel: $fuel, days: $days) { date price }
    }
  }
`;

export type DepartmentAverage = { department: string; avgPrice: number; stationCount: number };

export const DEPARTMENT_AVERAGES = /* GraphQL */ `
  query DepartmentAverages($fuel: Fuel!) {
    departmentAverages(fuel: $fuel) { department avgPrice stationCount }
  }
`;

export type DepartmentStats = {
  department: string;
  date: string;
  avgPrice: number;
  minPrice: number;
  maxPrice: number;
  stationCount: number;
  cheapestStation: { id: string; address: string; city: string; currentPrices: CurrentPrice[] } | null;
  trend30d: PricePoint[];
};

export const DEPARTMENT_STATS = /* GraphQL */ `
  query DepartmentStats($department: String!, $fuel: Fuel!) {
    departmentStats(department: $department, fuel: $fuel) {
      department
      date
      avgPrice
      minPrice
      maxPrice
      stationCount
      cheapestStation { id address city currentPrices { fuel price updatedAt outOfStock } }
      trend30d { date price }
    }
  }
`;

export type HorizonScore = {
  horizon: number;
  maeBaseline: number;
  maeRidge: number;
  maeModel: number;
  intervalCoverage: number;
  beatsBaseline: boolean;
  periods: { name: string; maeBaseline: number; maeModel: number }[];
};

export type ModelRun = {
  modelVersion: string;
  trainedAt: string;
  trainStart: string;
  trainEnd: string;
  testStart: string;
  testEnd: string;
  allowedHorizons: number[];
  horizons: HorizonScore[];
  recommendations: {
    accuracy: number | null;
    waitCount: number;
    fillNowCount: number;
    stableCount: number;
    noneCount: number;
    waitAccuracy: number | null;
    fillNowAccuracy: number | null;
    stableAccuracy: number | null;
    waitGainCents: number | null;
    fillNowGainCents: number | null;
  };
  featureImportance: { feature: string; maeIncrease: number }[];
};

// Page "Comment marche le modèle" : le dernier entraînement en détail, les
// précédents pour l'historique des versions
export const MODEL_RUNS = /* GraphQL */ `
  query ModelRuns {
    modelRuns(limit: 10) {
      modelVersion
      trainedAt
      trainStart
      trainEnd
      testStart
      testEnd
      allowedHorizons
      horizons {
        horizon
        maeBaseline
        maeRidge
        maeModel
        intervalCoverage
        beatsBaseline
        periods { name maeBaseline maeModel }
      }
      recommendations {
        accuracy
        waitCount
        fillNowCount
        stableCount
        noneCount
        waitAccuracy
        fillNowAccuracy
        stableAccuracy
        waitGainCents
        fillNowGainCents
      }
      featureImportance { feature maeIncrease }
    }
  }
`;

