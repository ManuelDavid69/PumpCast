// Mise en forme des prix, distances et dates pour l'affichage.

export type Fuel = "GAZOLE" | "SP95" | "SP98" | "E10" | "E85" | "GPLC";

export const FUELS: Fuel[] = ["GAZOLE", "E10", "SP98", "SP95", "E85", "GPLC"];

// Noms affichés sur les pompes
export const FUEL_LABELS: Record<Fuel, string> = {
  GAZOLE: "Gazole",
  SP95: "SP95",
  SP98: "SP98",
  E10: "SP95-E10",
  E85: "E85",
  GPLC: "GPLc",
};

/**
 * Découpe un prix comme sur les panneaux des stations : "2,14" en grand
 * et le millième à part ("9"), affiché plus petit.
 */
export function splitPrice(price: number): { main: string; thousandth: string } {
  const fixed = price.toFixed(3); // "2.149"
  const [euros, decimals] = fixed.split(".");
  return { main: `${euros},${decimals.slice(0, 2)}`, thousandth: decimals.slice(2) };
}

/** Prix complet en texte, par exemple "2,149 €". */
export function formatPrice(price: number): string {
  return `${price.toFixed(3).replace(".", ",")} €`;
}

/** Écart en centimes, signé : "+3,2 c" ou "−1,0 c". */
export function formatCents(delta: number): string {
  const cents = delta * 100;
  const sign = cents > 0.05 ? "+" : cents < -0.05 ? "−" : "";
  return `${sign}${Math.abs(cents).toFixed(1).replace(".", ",")} c`;
}

/** "850 m" sous un kilomètre, "2,4 km" au delà. */
export function formatDistance(km: number): string {
  if (km < 1) {
    return `${Math.round(km * 100) * 10} m`;
  }
  return `${km.toFixed(1).replace(".", ",")} km`;
}

/** "2026-11-04" vers "4 nov." */
export function formatShortDate(isoDay: string): string {
  const d = new Date(`${isoDay}T12:00:00`);
  return d.toLocaleDateString("fr-FR", { day: "numeric", month: "short" });
}

/** Date et heure de mise à jour, relative si c'est récent. */
export function formatUpdatedAt(iso: string, now: Date = new Date()): string {
  const d = new Date(iso);
  const minutes = Math.round((now.getTime() - d.getTime()) / 60000);
  if (minutes < 60) {
    return `il y a ${Math.max(minutes, 1)} min`;
  }
  const sameDay = d.toDateString() === now.toDateString();
  const time = d.toLocaleTimeString("fr-FR", { hour: "2-digit", minute: "2-digit" });
  if (sameDay) {
    return `aujourd'hui à ${time}`;
  }
  return `le ${d.toLocaleDateString("fr-FR", { day: "numeric", month: "long" })}`;
}

/** Les villes du flux sont parfois en majuscules ("VILLEURBANNE"). */
export function formatCity(city: string): string {
  if (city !== city.toUpperCase()) {
    return city;
  }
  return city
    .toLowerCase()
    .replace(/(^|[\s'-])(\p{L})/gu, (_, sep: string, letter: string) => sep + letter.toUpperCase());
}
