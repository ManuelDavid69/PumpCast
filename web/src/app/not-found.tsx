import Link from "next/link";

export default function NotFound() {
  return (
    <div className="mx-auto max-w-7xl px-4 py-16">
      <h1 className="font-board text-3xl font-semibold">Page introuvable</h1>
      <p className="mt-2 max-w-prose text-[15px] text-muted">
        Cette station ou cette page n&apos;existe pas, ou plus. Les stations fermées disparaissent du flux officiel.
      </p>
      <Link href="/" className="mt-6 inline-block rounded bg-ink px-4 py-2 font-medium text-white">
        Revenir à la carte
      </Link>
    </div>
  );
}
