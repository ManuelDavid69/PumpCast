import Link from "next/link";
import { Price } from "@/components/Price";
import type { Prediction } from "@/graphql/queries";
import { FUEL_LABELS, formatShortDate, type Fuel } from "@/lib/format";

// Le conseil de la fiche station. Quand le modèle n'est pas assez fiable,
// on le dit au lieu d'afficher une fausse certitude.

const TITLES = {
  FILL_NOW: "Fais le plein maintenant",
  WAIT: "Attends, baisse probable",
  STABLE: "Prix stable",
} as const;

function confidenceLabel(c: number): string {
  if (c >= 0.85) return "confiance élevée";
  if (c >= 0.7) return "confiance moyenne";
  return "confiance faible";
}

function weekday(isoDay: string): string {
  return new Date(`${isoDay}T12:00:00`).toLocaleDateString("fr-FR", { weekday: "long" });
}

export function PredictionPanel({ prediction, fuel }: { prediction: Prediction | null; fuel: Fuel }) {
  if (!prediction) {
    return (
      <p className="mt-2 max-w-prose text-[15px] leading-relaxed text-muted">
        Pas de prévision pour cette station : elle n&apos;a pas assez d&apos;historique, ou elle n&apos;est pas en
        Auvergne-Rhône-Alpes (le modèle ne tourne que sur cette région pour l&apos;instant).
      </p>
    );
  }

  const rec = prediction.recommendation;
  // Le jour visé par le conseil, calculé par le script de prédiction
  const target = prediction.adviceTarget;

  return (
    <div className="mt-2">
      {rec ? (
        <>
          <p className={`font-board text-3xl font-semibold ${rec === "FILL_NOW" ? "text-signal-ink" : rec === "WAIT" ? "text-route" : ""}`}>
            {TITLES[rec]}
          </p>
          {target && rec !== "STABLE" && (
            <p className="mt-1 text-[15px]">
              Le {FUEL_LABELS[fuel]} est à <Price value={prediction.basePrice} /> et devrait{" "}
              {rec === "WAIT" ? "descendre" : "monter"} vers <Price value={target.price} /> d&apos;ici {weekday(target.date)}{" "}
              (entre <Price value={Math.min(target.low, target.high)} /> et <Price value={Math.max(target.low, target.high)} />, {confidenceLabel(prediction.confidence)}).
            </p>
          )}
          {rec === "STABLE" && (
            <p className="mt-1 text-[15px]">
              Pas de variation de plus de 2 centimes attendue sur 3 jours, tu peux faire le plein quand ça t&apos;arrange.
            </p>
          )}
        </>
      ) : (
        <>
          <p className="font-board text-2xl font-semibold">Pas de conseil aujourd&apos;hui</p>
          <p className="mt-1 max-w-prose text-[15px] leading-relaxed text-muted">
            PumpCast ne donne un conseil que quand ce type de conseil a vraiment fait gagner de l&apos;argent sur les
            mois passés. Ici le modèle n&apos;a rien d&apos;assez sûr à dire, alors il ne dit rien.{" "}
            <Link href="/modele#conseils" className="text-ink underline">
              Comment c&apos;est décidé
            </Link>
          </p>
        </>
      )}

      {prediction.reasons.length > 0 && (
        // Ce que montrent les données en ce moment, pas forcément la raison du conseil
        <p className="mt-3 text-sm font-medium">En ce moment</p>
      )}
      {prediction.reasons.length > 0 && (
        <ul className="mt-1 list-disc space-y-1 pl-5 text-[15px] text-muted">
          {prediction.reasons.map((r) => (
            <li key={r}>{r}</li>
          ))}
        </ul>
      )}

      {prediction.horizon.length > 0 && (
        <p className="mt-3 text-sm text-muted">
          Prévision calculée le {formatShortDate(prediction.baseDate)}, la courbe va jusqu&apos;au{" "}
          {formatShortDate(prediction.horizon[prediction.horizon.length - 1].date)}
        </p>
      )}
    </div>
  );
}
