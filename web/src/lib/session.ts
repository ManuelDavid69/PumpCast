// Session côté serveur Next.js.
//
// Le jeton de connexion (JWT) est rangé dans un cookie httpOnly : le
// JavaScript de la page ne peut pas le lire, donc un script injecté (XSS) ne
// peut pas le voler. Le navigateur l'envoie à notre serveur Next, qui le
// transmet à l'API Go dans l'en-tête Authorization.

import "server-only";
import { cookies } from "next/headers";

export const SESSION_COOKIE = "pumpcast_session";
const SEVEN_DAYS = 7 * 24 * 60 * 60;

const API_URL = process.env.GRAPHQL_URL ?? process.env.NEXT_PUBLIC_GRAPHQL_URL ?? "http://localhost:8080/graphql";

export async function getToken(): Promise<string | null> {
  const store = await cookies();
  return store.get(SESSION_COOKIE)?.value ?? null;
}

export async function setToken(token: string): Promise<void> {
  const store = await cookies();
  store.set(SESSION_COOKIE, token, {
    httpOnly: true,
    // Lax : le cookie n'est pas envoyé sur un POST venant d'un autre site (CSRF)
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: SEVEN_DAYS,
  });
}

export async function clearToken(): Promise<void> {
  const store = await cookies();
  store.delete(SESSION_COOKIE);
}

/**
 * Les requêtes qui changent quelque chose (POST) ne sont acceptées que si
 * elles viennent de notre propre site : en-tête Origin égal à notre origine.
 */
export function isSameOrigin(request: Request): boolean {
  const origin = request.headers.get("origin");
  const host = request.headers.get("x-forwarded-host") ?? request.headers.get("host");
  if (!origin || !host) {
    return false;
  }
  try {
    return new URL(origin).host === host;
  } catch {
    return false;
  }
}

/** IP du visiteur, transmise à l'API pour le rate limiting. */
export function clientIP(request: Request): string {
  const forwarded = request.headers.get("x-forwarded-for");
  return forwarded?.split(",")[0]?.trim() || request.headers.get("x-real-ip") || "local";
}

type GraphQLBody = { data?: unknown; errors?: { message: string }[] };

/** Appelle l'API Go avec le jeton de l'utilisateur et l'IP du visiteur. */
export async function callApi(
  query: string,
  variables: Record<string, unknown>,
  opts: { token?: string | null; clientIP?: string },
): Promise<GraphQLBody> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (opts.token) {
    headers.Authorization = `Bearer ${opts.token}`;
  }
  if (process.env.INTERNAL_API_KEY) {
    headers["X-Internal-Key"] = process.env.INTERNAL_API_KEY;
    if (opts.clientIP) {
      headers["X-Client-IP"] = opts.clientIP;
    }
  }
  const res = await fetch(API_URL, { method: "POST", headers, body: JSON.stringify({ query, variables }), cache: "no-store" });
  return (await res.json()) as GraphQLBody;
}
