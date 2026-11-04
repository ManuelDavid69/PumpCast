// Appels GraphQL côté serveur (pages rendues par Next.js).
//
// On utilise un simple fetch plutôt qu'Apollo côté serveur : les fonctions
// qui l'appellent sont marquées 'use cache', c'est le mécanisme de cache de
// Next.js, et elles n'ont pas besoin du cache normalisé d'Apollo.

const SERVER_URL =
  process.env.GRAPHQL_URL ?? process.env.NEXT_PUBLIC_GRAPHQL_URL ?? "http://localhost:8080/graphql";

type GraphQLResponse<T> = {
  data?: T;
  errors?: { message: string }[];
};

export async function gqlFetch<T>(query: string, variables: Record<string, unknown> = {}): Promise<T> {
  const res = await fetch(SERVER_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query, variables }),
  });
  const body = (await res.json()) as GraphQLResponse<T>;
  if (body.errors?.length) {
    throw new Error(body.errors.map((e) => e.message).join(", "));
  }
  if (!res.ok || !body.data) {
    throw new Error(`l'API a répondu ${res.status}`);
  }
  return body.data;
}
