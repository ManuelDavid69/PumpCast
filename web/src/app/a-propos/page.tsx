import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "À propos",
  description: "Pourquoi PumpCast existe, d'où viennent les données et ce que l'appli ne sait pas faire.",
};

// Page écrite à la main, à la première personne : c'est un projet perso,
// autant le dire simplement.
export default function AboutPage() {
  return (
    <article className="mx-auto max-w-2xl px-4 py-8 text-[16px] leading-relaxed">
      <h1 className="font-board text-4xl font-semibold">À propos</h1>

      <p className="mt-4">
        Je m&apos;appelle Manuel, je suis en 4e année d&apos;école d&apos;ingénieur à CPE Lyon (Informatique et Réseaux
        de Communication), en alternance. PumpCast est un projet perso que j&apos;ai fait pour apprendre des outils que je
        n&apos;avais jamais utilisés : Go, GraphQL, MongoDB, Redis, Next.js et le machine learning avec scikit-learn.
      </p>
      <p className="mt-3">
        L&apos;idée de départ est une question que tout le monde se pose devant une pompe : je fais le plein maintenant,
        ou j&apos;attends quelques jours&nbsp;? Les applis de prix montrent le prix du jour, pas ce qu&apos;il va devenir.
        J&apos;ai voulu voir jusqu&apos;où on pouvait aller avec les données publiques.
      </p>

      <h2 className="mt-8 text-xl font-semibold">Ce que j&apos;ai appris en fouillant les données</h2>
      <ul className="mt-2 list-disc space-y-2 pl-5">
        <li>
          Une station change son prix environ un jour sur quatre, souvent de 1 ou 2 centimes. Presque jamais le dimanche,
          et le vendredi est le jour des baisses.
        </li>
        <li>
          Le « fais le plein le lundi, c&apos;est moins cher » ne se vérifie pas : l&apos;écart entre les jours de la
          semaine est d&apos;environ 0,1 centime, soit 5 centimes sur un plein.
        </li>
        <li>
          La pompe suit le pétrole avec du retard : la moitié d&apos;une hausse du Brent arrive en 5 jours, presque tout
          en deux semaines. Les hausses passent un peu plus vite que les baisses.
        </li>
        <li>Sur autoroute, le gazole coûte en moyenne 17 centimes de plus.</li>
        <li>
          Je m&apos;attendais à voir les prix monter avant les vacances scolaires. Dans les données, ce n&apos;est pas le
          cas, même sur autoroute.
        </li>
      </ul>
      <p className="mt-3">
        Le modèle de prévision part de tout ça.{" "}
        <Link href="/modele" className="underline">
          Ses résultats sont détaillés ici
        </Link>
        , y compris là où il ne fait pas mieux que « le prix ne bouge pas ».
      </p>

      <h2 className="mt-8 text-xl font-semibold">Ce que PumpCast ne sait pas faire</h2>
      <ul className="mt-2 list-disc space-y-2 pl-5">
        <li>
          Les prix actuels couvrent toute la France, mais l&apos;historique et les prévisions seulement
          l&apos;Auvergne-Rhône-Alpes. L&apos;hébergement gratuit de la base de données est limité à 512 Mo, il a fallu
          choisir.
        </li>
        <li>
          Le nom des enseignes (Total, Leclerc...) n&apos;est pas dans les données officielles. Je ne l&apos;ai pas
          deviné, PumpCast affiche l&apos;adresse.
        </li>
        <li>
          Les prix sont déclarés par les stations elles-mêmes. Certaines ne les mettent pas à jour, d&apos;autres font
          des fautes de frappe (un gazole à 8,82 € dans les archives). Les valeurs absurdes sont retirées, mais il peut
          en rester.
        </li>
        <li>L&apos;outre-mer n&apos;est pas dans le flux officiel des prix.</li>
      </ul>

      <h2 id="sources" className="mt-8 text-xl font-semibold">
        D&apos;où viennent les données
      </h2>
      <ul className="mt-2 list-disc space-y-1 pl-5">
        <li>Prix des carburants, flux instantané et archives annuelles : prix-carburants.gouv.fr (Licence Ouverte 2.0)</li>
        <li>Cours du Brent : FRED, Federal Reserve Bank of St. Louis</li>
        <li>Taux de change euro / dollar : Banque centrale européenne</li>
        <li>Jours fériés : calendrier.api.gouv.fr</li>
        <li>Vacances scolaires : data.education.gouv.fr</li>
        <li>Recherche de ville : Géoplateforme de l&apos;IGN</li>
        <li>Contours des départements : IGN Admin Express (Licence Ouverte)</li>
        <li>Fond de carte : © les contributeurs OpenStreetMap</li>
      </ul>

      <h2 className="mt-8 text-xl font-semibold">Le code</h2>
      <p className="mt-2">
        Tout est sur{" "}
        <a href="https://github.com/ManuelDavid69/PumpCast" className="underline">
          GitHub
        </a>
        , avec un README qui explique les choix techniques : API en Go avec GraphQL, MongoDB, Redis, front en Next.js,
        modèle en Python. Si tu vois une erreur ou un prix bizarre, tu peux ouvrir une issue.
      </p>
    </article>
  );
}
