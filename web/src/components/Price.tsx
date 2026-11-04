import { formatPrice, splitPrice } from "@/lib/format";

// Prix affiché comme sur un totem de station : "2,14" puis le millième en petit.
// Le texte complet reste lisible par les lecteurs d'écran.
export function Price({ value, className = "" }: { value: number; className?: string }) {
  const { main, thousandth } = splitPrice(value);
  return (
    <span className={`font-board tabular-nums ${className}`} aria-label={formatPrice(value)}>
      <span aria-hidden="true">
        {main}
        <sup className="ml-px text-[0.55em] align-[0.6em]">{thousandth}</sup>
      </span>
    </span>
  );
}
