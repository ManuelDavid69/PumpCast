"use client";

// Client Apollo pour les composants qui tournent dans le navigateur
// (la carte de l'accueil, qui dépend de la position de l'utilisateur).

import { ApolloClient, HttpLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { useState, type ReactNode } from "react";

function makeClient() {
  return new ApolloClient({
    link: new HttpLink({ uri: process.env.NEXT_PUBLIC_GRAPHQL_URL ?? "http://localhost:8080/graphql" }),
    cache: new InMemoryCache(),
  });
}

export function Providers({ children }: { children: ReactNode }) {
  // Un seul client par onglet, créé au premier rendu
  const [client] = useState(makeClient);
  return <ApolloProvider client={client}>{children}</ApolloProvider>;
}
