// Inscription, connexion et déconnexion : /api/auth/register, /login, /logout.
// Le jeton renvoyé par l'API Go n'est jamais donné au navigateur, il est
// rangé directement dans le cookie httpOnly.

import { NextResponse } from "next/server";
import { callApi, clearToken, clientIP, isSameOrigin, setToken } from "@/lib/session";

const MUTATIONS = {
  register: `mutation($email: String!, $password: String!) { register(email: $email, password: $password) { token user { email } } }`,
  login: `mutation($email: String!, $password: String!) { login(email: $email, password: $password) { token user { email } } }`,
} as const;

type AuthData = { token: string; user: { email: string } };

export async function POST(request: Request, ctx: RouteContext<"/api/auth/[action]">) {
  if (!isSameOrigin(request)) {
    return NextResponse.json({ error: "requête refusée" }, { status: 403 });
  }
  const { action } = await ctx.params;

  if (action === "logout") {
    await clearToken();
    return NextResponse.json({ ok: true });
  }
  if (action !== "register" && action !== "login") {
    return NextResponse.json({ error: "action inconnue" }, { status: 404 });
  }

  let body: { email?: unknown; password?: unknown };
  try {
    body = await request.json();
  } catch {
    return NextResponse.json({ error: "requête mal formée" }, { status: 400 });
  }
  if (typeof body.email !== "string" || typeof body.password !== "string") {
    return NextResponse.json({ error: "email et mot de passe obligatoires" }, { status: 400 });
  }

  const res = await callApi(MUTATIONS[action], { email: body.email, password: body.password }, { clientIP: clientIP(request) });
  if (res.errors?.length) {
    return NextResponse.json({ error: res.errors[0].message }, { status: 400 });
  }
  const payload = (res.data as Record<string, AuthData>)[action];
  await setToken(payload.token);
  return NextResponse.json({ ok: true, email: payload.user.email });
}
