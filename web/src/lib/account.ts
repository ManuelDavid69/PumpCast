"use client";

// Appels liés au compte depuis le navigateur. Tout passe par les routes du
// serveur Next (/api/...), qui ajoutent le jeton gardé dans le cookie.

export class AccountError extends Error {}

export async function accountQuery<T>(query: string, variables: Record<string, unknown> = {}): Promise<T> {
  const res = await fetch("/api/graphql", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query, variables }),
  });
  const body = (await res.json()) as { data?: T; errors?: { message: string }[] };
  if (body.errors?.length) {
    throw new AccountError(body.errors[0].message);
  }
  if (!res.ok || !body.data) {
    throw new AccountError("le serveur ne répond pas, réessaie dans un moment");
  }
  return body.data;
}

export async function authAction(action: "login" | "register" | "logout", email = "", password = ""): Promise<void> {
  const res = await fetch(`/api/auth/${action}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    throw new AccountError(body.error ?? "le serveur ne répond pas, réessaie dans un moment");
  }
}

export const ALERT_LABELS = {
  THRESHOLD: "Prix sous un seuil",
  PREDICTED_DROP: "Baisse prévue",
  PREDICTED_RISE: "Hausse prévue",
} as const;

export type AlertType = keyof typeof ALERT_LABELS;
