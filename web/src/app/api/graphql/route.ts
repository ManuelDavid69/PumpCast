// Relais vers l'API Go pour les requêtes qui ont besoin du compte (favoris,
// alertes) : on ajoute le jeton lu dans le cookie httpOnly. Les requêtes
// publiques (carte, stations) continuent d'aller directement à l'API.

import { NextResponse } from "next/server";
import { callApi, clientIP, getToken, isSameOrigin } from "@/lib/session";

export async function POST(request: Request) {
  if (!isSameOrigin(request)) {
    return NextResponse.json({ errors: [{ message: "requête refusée" }] }, { status: 403 });
  }
  let body: { query?: unknown; variables?: unknown };
  try {
    body = await request.json();
  } catch {
    return NextResponse.json({ errors: [{ message: "requête mal formée" }] }, { status: 400 });
  }
  if (typeof body.query !== "string" || body.query.length > 5000) {
    return NextResponse.json({ errors: [{ message: "requête mal formée" }] }, { status: 400 });
  }
  const variables = typeof body.variables === "object" && body.variables !== null ? (body.variables as Record<string, unknown>) : {};
  const res = await callApi(body.query, variables, { token: await getToken(), clientIP: clientIP(request) });
  return NextResponse.json(res);
}
