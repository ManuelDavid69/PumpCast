import { describe, expect, it } from "vitest";
import {
  formatCents,
  formatCentsAbs,
  formatDistance,
  formatLongDate,
  formatName,
  formatPrice,
  formatUpdatedAt,
  splitPrice,
} from "./format";

describe("splitPrice", () => {
  // Comme sur les totems : les centimes en grand, le millième à part
  it.each([
    [2.149, { main: "2,14", thousandth: "9" }],
    [1.5, { main: "1,50", thousandth: "0" }],
    [0.927, { main: "0,92", thousandth: "7" }],
  ])("%s", (price, expected) => {
    expect(splitPrice(price)).toEqual(expected);
  });
});

describe("formatPrice", () => {
  it("affiche trois décimales avec une virgule", () => {
    expect(formatPrice(2.149)).toBe("2,149 €");
  });
});

describe("formatCents", () => {
  it.each([
    [0.032, "+3,2 c"],
    [-0.01, "−1,0 c"], // signe moins typographique U+2212
    [0.0001, "0,0 c"], // pas de signe pour un écart nul
  ])("%s -> %s", (delta, expected) => {
    expect(formatCents(delta)).toBe(expected);
  });

  it("formatCentsAbs n'a jamais de signe", () => {
    expect(formatCentsAbs(-0.108)).toBe("10,8 c");
    expect(formatCentsAbs(0.05)).toBe("5,0 c");
  });
});

describe("formatDistance", () => {
  it.each([
    [0.248, "250 m"],
    [0.85, "850 m"],
    [2.43, "2,4 km"],
  ])("%s km -> %s", (km, expected) => {
    expect(formatDistance(km)).toBe(expected);
  });
});

describe("formatName", () => {
  it.each([
    ["VILLEURBANNE", "Villeurbanne"],
    ["86 AVENUE JEAN MERMOZ", "86 Avenue Jean Mermoz"],
    ["SAINT-DENIS-LÈS-BOURG", "Saint-Denis-Lès-Bourg"],
    ["ROUTE D'HEYRIEUX", "Route D'Heyrieux"],
    // Déjà en casse normale : on ne touche à rien
    ["Loire-sur-Rhône", "Loire-sur-Rhône"],
  ])("%s", (raw, expected) => {
    expect(formatName(raw)).toBe(expected);
  });
});

describe("formatUpdatedAt", () => {
  const now = new Date("2026-11-04T12:00:00+01:00");

  it("donne les minutes quand c'est récent", () => {
    expect(formatUpdatedAt("2026-11-04T11:55:00+01:00", now)).toBe("il y a 5 min");
  });

  it("n'affiche jamais 0 minute", () => {
    expect(formatUpdatedAt("2026-11-04T11:59:50+01:00", now)).toBe("il y a 1 min");
  });

  it("donne l'heure pour plus tôt dans la journée", () => {
    expect(formatUpdatedAt("2026-11-04T08:30:00+01:00", now)).toMatch(/^aujourd'hui à /);
  });

  it("donne la date pour un autre jour", () => {
    expect(formatUpdatedAt("2026-11-01T08:30:00+01:00", now)).toBe("le 1er novembre");
  });

  it("raisonne en heure de Paris même si le serveur est en UTC", () => {
    // 0 h 50 à Paris le 6, mais encore le 5 en UTC : la mise à jour de la
    // veille à 13 h 37 n'est pas "aujourd'hui"
    const afterMidnight = new Date("2026-11-06T00:50:00+01:00");
    expect(formatUpdatedAt("2026-11-05T13:37:00+01:00", afterMidnight)).toBe("le 5 novembre");
    expect(formatUpdatedAt("2026-11-04T08:30:00+01:00", now)).toBe("aujourd'hui à 08:30");
  });
});

describe("formatLongDate", () => {
  it("écrit le jour en entier avec l'année", () => {
    expect(formatLongDate("2026-11-04")).toBe("4 novembre 2026");
  });

  it("écrit 1er pour le premier du mois", () => {
    expect(formatLongDate("2025-11-01")).toBe("1er novembre 2025");
  });
});
