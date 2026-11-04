// Package graph contient les resolvers GraphQL : les fonctions qui vont
// chercher les données pour chaque champ du schéma.
package graph

// Resolver regroupe les dépendances partagées par tous les resolvers
// (l'équivalent des services injectés dans un contrôleur Spring).
type Resolver struct{}
