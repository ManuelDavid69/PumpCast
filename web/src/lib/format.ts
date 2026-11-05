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

// Codes européens affichés sur les pistolets depuis 2018. On ne les montre que
// quand ils apportent quelque chose (pas "E10 E10").
export const FUEL_CODES: Partial<Record<Fuel, string>> = {
  GAZOLE: "B7",
  SP95: "E5",
  SP98: "E5",
  GPLC: "LPG",
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

/** Écart en centimes sans signe, quand la phrase dit déjà "moins cher" ou "plus cher". */
export function formatCentsAbs(delta: number): string {
  return `${Math.abs(delta * 100).toFixed(1).replace(".", ",")} c`;
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

/** "2025-11-01" vers "1er novembre 2025". */
export function formatLongDate(isoDay: string): string {
  const d = new Date(`${isoDay}T12:00:00`);
  const text = d.toLocaleDateString("fr-FR", { day: "numeric", month: "long", year: "numeric" });
  return d.getDate() === 1 ? text.replace(/^1 /, "1er ") : text;
}

// Heure de Paris quelle que soit la machine qui fait le rendu : les serveurs
// de Vercel sont en UTC, sans ça une mise à jour de 13 h 37 s'affichait 12:37
const PARIS = "Europe/Paris";

/** Date et heure de mise à jour, relative si c'est récent. */
export function formatUpdatedAt(iso: string, now: Date = new Date()): string {
  const d = new Date(iso);
  const minutes = Math.round((now.getTime() - d.getTime()) / 60000);
  if (minutes < 60) {
    return `il y a ${Math.max(minutes, 1)} min`;
  }
  const dayOf = (x: Date) => x.toLocaleDateString("fr-FR", { timeZone: PARIS });
  const time = d.toLocaleTimeString("fr-FR", { hour: "2-digit", minute: "2-digit", timeZone: PARIS });
  if (dayOf(d) === dayOf(now)) {
    return `aujourd'hui à ${time}`;
  }
  const dayNumber = d.toLocaleDateString("fr-FR", { day: "numeric", timeZone: PARIS });
  const month = d.toLocaleDateString("fr-FR", { month: "long", timeZone: PARIS });
  return `le ${dayNumber === "1" ? "1er" : dayNumber} ${month}`;
}

/**
 * Les villes et adresses du flux sont parfois tout en majuscules
 * ("VILLEURBANNE", "86 AVENUE JEAN MERMOZ") : on les remet en casse normale.
 * Un texte déjà en minuscules est laissé tel quel.
 */
export function formatName(text: string): string {
  if (text !== text.toUpperCase()) {
    return text;
  }
  return text
    .toLowerCase()
    .replace(/(^|[\s'-])(\p{L})/gu, (_, sep: string, letter: string) => sep + letter.toUpperCase());
}
