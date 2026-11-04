package graph

import (
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"

	"github.com/ManuelDavid69/pumpcast/api/graph/generated"
	"github.com/ManuelDavid69/pumpcast/api/internal/dataloader"
)

// Chaque champ demandé coûte 1 point. Au delà de cette limite la requête est
// refusée avant d'être exécutée, pour éviter les requêtes énormes imbriquées.
const complexityLimit = 500

// NewHandler construit le handler HTTP de l'endpoint /graphql.
// Utilisé par le serveur et par les tests, pour tester exactement la même chose.
func NewHandler(res *Resolver, src dataloader.Sources) http.Handler {
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: res}))
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.Use(extension.Introspection{})
	srv.Use(extension.FixedComplexityLimit(complexityLimit))
	return dataloader.Middleware(src, srv)
}
