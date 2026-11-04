"use client";

import { formatUpdatedAt } from "@/lib/format";

// "il y a 5 min" dépend de l'heure actuelle : calculé dans le navigateur,
// sinon la valeur serait figée dans la page mise en cache côté serveur.
// suppressHydrationWarning : la minute peut changer entre le rendu serveur et le navigateur.
export function UpdatedAt({ iso }: { iso: string }) {
  return <span suppressHydrationWarning>{formatUpdatedAt(iso)}</span>;
}
