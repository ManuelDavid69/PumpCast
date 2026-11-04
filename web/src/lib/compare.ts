"use client";

// Stations choisies pour la comparaison, gardées dans le navigateur
// pour retrouver la sélection en revenant sur la carte.
//
// localStorage est une source extérieure à React : useSyncExternalStore
// est le hook prévu pour la lire et être prévenu quand elle change.

import { useCallback, useMemo, useSyncExternalStore } from "react";

export const MAX_COMPARED = 5;
const KEY = "pumpcast:compare";
const EVENT = "pumpcast:compare-change";

function readRaw(): string {
  try {
    return window.localStorage.getItem(KEY) ?? "[]";
  } catch {
    // Navigation privée ou stockage bloqué : la sélection ne sera juste pas gardée
    return "[]";
  }
}

function parse(raw: string): string[] {
  try {
    const ids = JSON.parse(raw) as unknown;
    return Array.isArray(ids) ? ids.filter((x): x is string => typeof x === "string").slice(0, MAX_COMPARED) : [];
  } catch {
    return [];
  }
}

function subscribe(onChange: () => void) {
  // "storage" : changement depuis un autre onglet, EVENT : depuis cet onglet
  window.addEventListener("storage", onChange);
  window.addEventListener(EVENT, onChange);
  return () => {
    window.removeEventListener("storage", onChange);
    window.removeEventListener(EVENT, onChange);
  };
}

export function useCompareSelection() {
  const raw = useSyncExternalStore(subscribe, readRaw, () => "[]");
  const ids = useMemo(() => parse(raw), [raw]);

  const toggle = useCallback((id: string) => {
    const prev = parse(readRaw());
    const next = prev.includes(id)
      ? prev.filter((x) => x !== id)
      : prev.length < MAX_COMPARED
        ? [...prev, id]
        : prev;
    try {
      window.localStorage.setItem(KEY, JSON.stringify(next));
    } catch {
      return;
    }
    window.dispatchEvent(new Event(EVENT));
  }, []);

  return { ids, toggle, full: ids.length >= MAX_COMPARED };
}
