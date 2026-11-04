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
];

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="fr" className={`${barlow.variable} ${barlowCondensed.variable}`}>
      <body className="flex min-h-dvh flex-col">
        <header className="border-b border-line bg-paper">
          <div className="mx-auto flex max-w-7xl items-center justify-between gap-2 px-4 py-3">
            <Link href="/" className="font-board text-2xl font-bold tracking-tight">
              PumpCast
            </Link>
            <nav className="flex gap-0.5 text-sm font-medium sm:gap-1 sm:text-[15px]">
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
          <div className="mx-auto max-w-7xl px-4 py-5 leading-relaxed">
            Données : prix des carburants (prix-carburants.gouv.fr, Licence Ouverte 2.0), Brent (FRED), EUR/USD (BCE),
            calendrier scolaire (data.education.gouv.fr), contours des départements (IGN Admin Express), fond de carte
            © les contributeurs OpenStreetMap. L&apos;enseigne des stations n&apos;est pas dans les données officielles.
          </div>
        </footer>
      </body>
    </html>
  );
}
