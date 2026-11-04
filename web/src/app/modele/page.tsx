// Page de transparence : ce que vaut vraiment le modèle, chiffres à l'appui.
// Les phrases sont construites à partir des résultats, pour ne jamais
// annoncer mieux que ce que le test a montré.

import type { Metadata } from "next";
import { Suspense } from "react";
import type { ModelRun } from "@/graphql/queries";
import { getModelRuns } from "@/lib/api";
import { formatLongDate } from "@/lib/format";

export const metadata: Metadata = {
  title: "Le modèle de prévision",
  description: "Comment PumpCast prévoit les prix, et ce que donne le modèle face à « le prix ne bouge pas ».",
};

// Noms des variables tels qu'on les expliquerait à quelqu'un
const FEATURE_LABELS: Record<string, string> = {
  nat_trend1: "Variation des prix en France la veille",
  nat_trend3: "Variation des prix en France sur 3 jours",
  nat_trend7: "Variation des prix en France sur 7 jours",
  change_freq_60: "Fréquence habituelle des changements de la station",
  expected_changes: "Temps écoulé comparé au rythme de la station",
  days_since_change: "Jours depuis le dernier changement",
  weekday: "Jour de la semaine",
  d1: "Variation de la station la veille",
  d3: "Variation de la station sur 3 jours",
  d7: "Variation de la station sur 7 jours",
  dep_gap: "Écart avec la moyenne du département",
  dep_trend7: "Variation du département sur 7 jours",
  dep_moved_since_change: "Mouvement du département depuis le dernier changement",
  nat_moved_since_change: "Mouvement national depuis le dernier changement",
  brent_d7: "Variation du pétrole sur 7 jours",
  brent_d14: "Variation du pétrole sur 14 jours",
  brent_d21: "Variation du pétrole sur 21 jours",
  eurusd_d14: "Variation de l'euro face au dollar",
  margin_gap: "Marge pompe moins pétrole comparée à d'habitude",
  gap_ma7: "Écart avec la moyenne de la station sur 7 jours",
  gap_ma30: "Écart avec la moyenne de la station sur 30 jours",
  price: "Prix actuel",
  fuel: "Carburant",
  department: "Département",
  autoroute: "Station d'autoroute",
  holiday_next7: "Jours fériés dans la semaine",
  vacation_a: "Vacances scolaires en cours",
  vacation_a_start_in7: "Départ en vacances dans la semaine",
};

// Espace insécable avant l'unité : "1,576 c" ne doit pas être coupé en fin de ligne
const KIND_NAMES = { WAIT: "Attends", FILL_NOW: "Fais le plein", STABLE: "Prix stable" } as const;

function ruleCell(count: number, accuracy: number | null, gain: number | null): string {
  if (count === 0 || accuracy === null) {
    return "jamais donné";
  }
  const parts = [`${count.toLocaleString("fr-FR")} fois`, `${Math.round(accuracy * 100)} % justes`];
  if (gain !== null) {
    parts.push(`${gain >= 0 ? "+" : "−"}${Math.abs(gain).toFixed(1).replace(".", ",")} c/L`);
  }
  return parts.join(", ");
}

const cents = (v: number, digits = 3) => `${v.toFixed(digits).replace(".", ",")} c`;
const percent = (v: number) => `${Math.round(v * 100)} %`;

export default function ModelPage() {
  return (
    <div className="mx-auto max-w-4xl px-4 py-8">
      <Suspense fallback={<div className="h-96 animate-pulse rounded bg-line/40" />}>
        <ModelView />
      </Suspense>
    </div>
  );
}

async function ModelView() {
  const runs = await getModelRuns();
  const run = runs[0];
  if (!run) {
    return (
      <>
        <h1 className="font-board text-4xl font-semibold">Le modèle de prévision</h1>
        <p className="mt-3 text-[15px] text-muted">Le modèle n&apos;a pas encore été entraîné.</p>
      </>
    );
  }
  return <Report run={run} history={runs} />;
}

function Report({ run, history }: { run: ModelRun; history: ModelRun[] }) {
  const h3 = run.horizons.find((h) => h.horizon === 3);
  const best = [...run.horizons].sort((a, b) => a.maeModel / a.maeBaseline - b.maeModel / b.maeBaseline)[0];
  const rec = run.recommendations;
  const crit = run.recommendationCriteria;
  const enabledNames = run.enabledRecommendations.map((k) => `« ${KIND_NAMES[k]} »`);
  const maxImportance = Math.max(...run.featureImportance.map((f) => f.maeIncrease), 1e-9);

  return (
    <article className="text-[16px] leading-relaxed">
      <h1 className="font-board text-4xl font-semibold leading-tight">Le modèle de prévision</h1>
      <p className="mt-3 max-w-prose text-muted">
        Version {run.modelVersion}, entraînée le {new Date(run.trainedAt).toLocaleDateString("fr-FR")}. Testée sur des
        jours qu&apos;elle n&apos;avait jamais vus, du {formatLongDate(run.testStart)} au {formatLongDate(run.testEnd)}.
      </p>

      <section className="mt-8">
        <h2 className="text-xl font-semibold">Le résultat, sans arrondir</h2>
        {h3 && (
          <p className="mt-2 max-w-prose">
            {h3.beatsBaseline ? (
              <>
                À 3 jours, le modèle se trompe en moyenne de {cents(h3.maeModel)} par litre, contre {cents(h3.maeBaseline)}{" "}
                si on suppose simplement que le prix ne bouge pas.
              </>
            ) : (
              <>
                À 3 jours, le modèle ne fait pas mieux que de supposer que le prix ne bouge pas : {cents(h3.maeModel)}{" "}
                d&apos;erreur moyenne contre {cents(h3.maeBaseline)}.
              </>
            )}{" "}
            Ça paraît peu, mais c&apos;est la vraie difficulté : la plupart des jours, une station ne change pas son prix,
            et « rien ne bouge » est une prévision très dure à battre.
          </p>
        )}
        <p className="mt-2 max-w-prose">
          La courbe de prévision des fiches stations n&apos;affiche que les jours où le modèle fait mieux que cette
          prévision naïve
          {run.allowedHorizons.length > 0 ? <> (J+{run.allowedHorizons.join(", J+")})</> : <> (aucun pour l&apos;instant)</>}.
          {best && best.maeModel < best.maeBaseline && (
            <> Le meilleur résultat est à J+{best.horizon} : {percent(1 - best.maeModel / best.maeBaseline)} d&apos;erreur en moins.</>
          )}
        </p>
        <p className="mt-2 max-w-prose">
          Pour les conseils, la bonne question n&apos;est pas l&apos;erreur moyenne mais : est-ce que les suivre fait
          gagner de l&apos;argent ?{" "}
          {enabledNames.length > 0 ? (
            <>Aujourd&apos;hui l&apos;appli donne les conseils {enabledNames.join(", ")}.</>
          ) : (
            <>Aujourd&apos;hui aucun type de conseil n&apos;a passé le test, l&apos;appli n&apos;en donne donc pas.</>
          )}{" "}
          <a href="#conseils" className="underline">
            Le détail est plus bas
          </a>
          .
        </p>
      </section>

      <section className="mt-8" aria-labelledby="table-title">
        <h2 id="table-title" className="text-xl font-semibold">
          Erreur moyenne par horizon
        </h2>
        <p className="mt-1 text-sm text-muted">En centimes par litre, plus c&apos;est bas mieux c&apos;est.</p>
        <div className="mt-3 overflow-x-auto">
          <table className="w-full min-w-[34rem] text-left text-[15px] tabular-nums">
            <thead className="border-b border-ink text-sm">
              <tr>
                <th scope="col" className="py-2 pr-3 font-medium">Dans</th>
                <th scope="col" className="py-2 pr-3 text-right font-medium">Le prix ne bouge pas</th>
                <th scope="col" className="py-2 pr-3 text-right font-medium">Régression linéaire</th>
                <th scope="col" className="py-2 pr-3 text-right font-medium">Modèle retenu</th>
                <th scope="col" className="py-2 pr-3 text-right font-medium">Fourchette juste</th>
                <th scope="col" className="py-2 text-right font-medium">Sur la courbe</th>
              </tr>
            </thead>
            <tbody>
              {run.horizons.map((h) => (
                <tr key={h.horizon} className="border-b border-line">
                  <th scope="row" className="py-2 pr-3 text-left font-medium">
                    {h.horizon} jour{h.horizon > 1 ? "s" : ""}
                  </th>
                  <td className="py-2 pr-3 text-right">{cents(h.maeBaseline)}</td>
                  <td className="py-2 pr-3 text-right text-muted">{cents(h.maeRidge)}</td>
                  <td className={`py-2 pr-3 text-right font-semibold ${h.beatsBaseline ? "text-route" : ""}`}>{cents(h.maeModel)}</td>
                  <td className="py-2 pr-3 text-right">{percent(h.intervalCoverage)}</td>
                  <td className="py-2 text-right">{h.beatsBaseline ? "oui" : "non"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="mt-2 max-w-prose text-sm text-muted">
          « Fourchette juste » : part des cas où le vrai prix est tombé dans la fourchette affichée. Elle est prévue pour
          tomber juste 8 fois sur 10.
        </p>
      </section>

      {h3 && h3.periods.length > 0 && (
        <section className="mt-8">
          <h2 className="text-xl font-semibold">Une année de test, deux ambiances</h2>
          <p className="mt-2 max-w-prose">
            L&apos;hiver 2025 était calme. À partir de mars 2026 le pétrole s&apos;est envolé et les prix ont beaucoup
            bougé. Les deux périodes ne se ressemblent pas, alors voici les chiffres séparés (à 3 jours) :
          </p>
          <ul className="mt-2 space-y-1">
            {h3.periods.map((p) => (
              <li key={p.name}>
                {p.name} : {cents(p.maeModel)} pour le modèle, {cents(p.maeBaseline)} pour « le prix ne bouge pas ».
              </li>
            ))}
          </ul>
        </section>
      )}

      <section id="conseils" className="mt-8 scroll-mt-6">
        <h2 className="text-xl font-semibold">Quand l&apos;appli donne un conseil</h2>
        <p className="mt-2 max-w-prose">
          Chaque type de conseil est rejoué sur des milliers de situations passées : si quelqu&apos;un l&apos;avait suivi,
          aurait-il payé moins cher ? Un conseil n&apos;est activé que s&apos;il passe deux épreuves.
        </p>
        <ul className="mt-2 max-w-prose list-disc space-y-1 pl-5">
          <li>
            Sur la validation (mai à octobre 2025, la période qui sert à faire les choix) : donné au moins{" "}
            {crit.minCountValidation} fois, juste dans au moins {percent(crit.minAccuracyValidation)} des cas, et pour
            « attends » et « fais le plein », un gain moyen positif.
          </li>
          <li>
            Sur le test (des jours jamais utilisés pour décider) : il doit le confirmer, avec au moins {crit.minCountTest}{" "}
            cas, {percent(crit.minAccuracyTest)} de conseils justes et toujours un gain positif.
          </li>
        </ul>
        <div className="mt-4 overflow-x-auto">
          <table className="w-full min-w-[36rem] text-left text-[15px] tabular-nums">
            <thead className="border-b border-ink text-sm">
              <tr>
                <th scope="col" className="py-2 pr-3 font-medium">Conseil</th>
                <th scope="col" className="py-2 pr-3 text-right font-medium">Validation</th>
                <th scope="col" className="py-2 pr-3 text-right font-medium">Test</th>
                <th scope="col" className="py-2 text-right font-medium">Activé</th>
              </tr>
            </thead>
            <tbody>
              {run.recommendationRules.map((r) => (
                <tr key={r.kind} className="border-b border-line align-top">
                  <th scope="row" className="py-2 pr-3 text-left font-medium">
                    {KIND_NAMES[r.kind]}
                  </th>
                  <td className={`py-2 pr-3 text-right ${r.passesValidation ? "" : "text-muted"}`}>
                    {ruleCell(r.validationCount, r.validationAccuracy, r.validationGainCents)}
                  </td>
                  <td className={`py-2 pr-3 text-right ${r.passesTest ? "" : "text-muted"}`}>
                    {ruleCell(r.testCount, r.testAccuracy, r.testGainCents)}
                  </td>
                  <td className={`py-2 text-right font-semibold ${r.enabled ? "text-route" : ""}`}>
                    {r.enabled ? "oui" : r.passesValidation ? "non, pas confirmé au test" : "non"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="mt-2 max-w-prose text-sm text-muted">
          Nombre de fois où le conseil aurait été donné, part de conseils justes, et gain moyen par litre en le suivant.
          Le modèle ne conseille d&apos;agir que quand il est sûr de lui, c&apos;est pour ça que « attends » et « fais le
          plein » sont rares.
        </p>

        <h3 className="mt-6 font-semibold">Et si on avait suivi les conseils activés ?</h3>
        {rec.accuracy === null ? (
          <p className="mt-2 max-w-prose">
            Sur la période de test, aucun conseil activé n&apos;aurait été donné. Pas de chiffre à afficher, donc.
          </p>
        ) : (
          <ul className="mt-2 space-y-1">
            <li>Conseils justes : {percent(rec.accuracy)}.</li>
            {rec.waitCount > 0 && rec.waitGainCents !== null && (
              <li>
                « Attends » donné {rec.waitCount.toLocaleString("fr-FR")} fois, juste dans {percent(rec.waitAccuracy ?? 0)} des
                cas, {cents(rec.waitGainCents, 1)} gagnés par litre en moyenne.
              </li>
            )}
            {rec.fillNowCount > 0 && rec.fillNowGainCents !== null && (
              <li>
                « Fais le plein » donné {rec.fillNowCount.toLocaleString("fr-FR")} fois, juste dans{" "}
                {percent(rec.fillNowAccuracy ?? 0)} des cas, {cents(rec.fillNowGainCents, 1)} gagnés par litre en moyenne.
              </li>
            )}
          </ul>
        )}
      </section>

      {run.featureImportance.length > 0 && (
        <section className="mt-8">
          <h2 className="text-xl font-semibold">Ce qui compte pour le modèle</h2>
          <p className="mt-2 max-w-prose">
            Pour chaque information, on la brouille et on regarde de combien l&apos;erreur augmente (à 3 jours). Plus la
            barre est longue, plus le modèle s&apos;en sert.
          </p>
          <ol className="mt-3 space-y-2">
            {run.featureImportance.slice(0, 8).map((f) => (
              <li key={f.feature} className="grid grid-cols-[minmax(0,1fr)_8rem] items-center gap-3 text-[15px]">
                <span>{FEATURE_LABELS[f.feature] ?? f.feature}</span>
                <span className="h-2.5 rounded-sm bg-autoroute" style={{ width: `${Math.max(4, (f.maeIncrease / maxImportance) * 100)}%` }} />
              </li>
            ))}
          </ol>
        </section>
      )}

      <section className="mt-8">
        <h2 className="text-xl font-semibold">Comment c&apos;est fait</h2>
        <ul className="mt-2 max-w-prose list-disc space-y-2 pl-5">
          <li>
            Un seul modèle pour toutes les stations d&apos;Auvergne-Rhône-Alpes et tous les carburants, un par horizon
            (de 1 à 7 jours). C&apos;est un gradient boosting de scikit-learn, comparé à une régression linéaire et à
            « le prix ne bouge pas ».
          </li>
          <li>
            Il apprend sur les prix depuis le {formatLongDate(run.trainStart)} et il est testé sur l&apos;année suivante,
            jamais sur des jours tirés au hasard : sinon il aurait « vu » le futur et les scores seraient faux.
          </li>
          <li>
            Le cours du pétrole est publié avec une semaine de retard environ. Le modèle n&apos;utilise que le cours qui
            était connu le jour de la prévision, même si ça le rend moins bon.
          </li>
          <li>
            Les relevés aberrants (un gazole à 8,82 €, une station 40 centimes au-dessus de son département) sont retirés
            avec la même règle à l&apos;entraînement et au test.
          </li>
          <li>
            La fourchette vient de deux modèles qui visent les 10 % les plus bas et les 10 % les plus hauts, puis elle est
            recalée sur des données que ces modèles n&apos;ont pas vues.
          </li>
        </ul>
      </section>

      {history.length > 1 && (
        <section className="mt-8">
          <h2 className="text-xl font-semibold">Versions précédentes</h2>
          <ul className="mt-2 space-y-1 text-[15px] tabular-nums">
            {history.map((r) => {
              const s = r.horizons.find((h) => h.horizon === 3);
              return (
                <li key={r.modelVersion}>
                  {r.modelVersion} : {s ? `${cents(s.maeModel)} à 3 jours (contre ${cents(s.maeBaseline)})` : "pas de score"}
                </li>
              );
            })}
          </ul>
        </section>
      )}
    </article>
  );
}
