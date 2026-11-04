"use client";

import { FUEL_LABELS, FUELS, type Fuel } from "@/lib/format";

// Choix du carburant, sous forme de boutons radio pour rester accessible au clavier
export function FuelPicker({ value, onChange, name = "fuel" }: { value: Fuel; onChange: (f: Fuel) => void; name?: string }) {
  return (
    <fieldset className="flex flex-wrap gap-1">
      <legend className="sr-only">Carburant</legend>
      {FUELS.map((fuel) => (
        <label
          key={fuel}
          className="cursor-pointer rounded border border-line bg-white px-2.5 py-1.5 text-sm font-medium text-muted has-[:checked]:border-ink has-[:checked]:bg-ink has-[:checked]:text-white has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-autoroute"
        >
          <input
            type="radio"
            name={name}
            value={fuel}
            checked={value === fuel}
            onChange={() => onChange(fuel)}
            className="sr-only"
          />
          {FUEL_LABELS[fuel]}
        </label>
      ))}
    </fieldset>
  );
}
