import type { Metadata } from "next";
import Link from "next/link";
import { Barlow, Barlow_Condensed } from "next/font/google";
import { Providers } from "@/lib/apollo";
import "./globals.css";

// Barlow s'inspire des panneaux routiers, la version condensée sert aux prix
const barlow = Barlow({ subsets: ["latin"], weight: ["400", "500", "600"], variable: "--font-barlow" });
const barlowCondensed = Barlow_Condensed({
  subsets: ["latin"],
  weight: ["500", "600", "700"],
  variable: "--font-barlow-condensed",
});

export const metadata: Metadata = {
  title: { default: "PumpCast, faire le plein au bon moment", template: "%s | PumpCast" },
  description:
    "Prix des carburants station par station, historique et prévision sur 7 jours pour savoir s'il vaut mieux faire le plein maintenant ou attendre.",
};

const NAV = [
  { href: "/", label: "Carte" },
  { href: "/compare", label: "Comparer" },
  { href: "/departements", label: "Départements" },
  { href: "/modele", label: "Modèle" },
  { href: "/a-propos", label: "À propos" },
];

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="fr" className={`${barlow.variable} ${barlowCondensed.variable}`}>
      <body className="flex min-h-dvh flex-col">
        <header className="border-b border-line bg-paper">
          <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-x-4 gap-y-1 px-4 py-3">
            <Link href="/" className="font-board text-2xl font-bold tracking-tight">
              PumpCast
            </Link>
            <nav className="-mx-2 flex flex-wrap gap-0.5 text-sm font-medium sm:mx-0 sm:gap-1 sm:text-[15px]">
              {NAV.map((item) => (
                <Link key={item.href} href={item.href} className="rounded px-2 py-1.5 text-muted hover:bg-line/50 hover:text-ink sm:px-3">
                  {item.label}
                </Link>
              ))}
            </nav>
          </div>
        </header>

        <Providers>
          <main className="flex-1">{children}</main>
        </Providers>

        <footer className="border-t border-line text-sm text-muted">
          <div className="mx-auto flex max-w-7xl flex-col gap-2 px-4 py-5 leading-relaxed sm:flex-row sm:justify-between">
            <p>
              Un projet perso de Manuel David, étudiant à CPE Lyon.{" "}
              <a href="https://github.com/ManuelDavid69/PumpCast" className="underline hover:text-ink">
                Le code est sur GitHub
              </a>
              .
            </p>
            <p>
              Prix : prix-carburants.gouv.fr (Licence Ouverte).{" "}
              <Link href="/a-propos#sources" className="underline hover:text-ink">
                Toutes les sources
              </Link>
            </p>
          </div>
        </footer>
      </body>
    </html>
  );
}
