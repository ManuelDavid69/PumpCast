package graph

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/ManuelDavid69/pumpcast/api/graph/generated"
	"github.com/ManuelDavid69/pumpcast/api/internal/apperr"
	"github.com/ManuelDavid69/pumpcast/api/internal/auth"
	"github.com/ManuelDavid69/pumpcast/api/internal/dataloader"
)

// Chaque champ demandé coûte 1 point. Au delà de cette limite la requête est
// refusée avant d'être exécutée, pour éviter les requêtes énormes imbriquées.
const complexityLimit = 500

// presentError décide ce que le client voit d'une erreur. Les erreurs
// publiques (apperr) et les erreurs de requête GraphQL (champ inconnu...)
// gardent leur message. Les autres (panne de MongoDB, de Redis...) sont
// écrites dans les logs et remplacées par un message neutre.
func presentError(ctx context.Context, err error) *gqlerror.Error {
	out := graphql.DefaultErrorPresenter(ctx, err)
	var public *apperr.Public
	if errors.As(err, &public) {
		out.Message = public.Msg
		return out
	}
	var gqlErr *gqlerror.Error
	if errors.As(err, &gqlErr) && gqlErr.Unwrap() == nil {
		return out
	}
	log.Printf("erreur interne sur %v : %v", out.Path, err)
	out.Message = "erreur interne, réessaie dans un moment"
	return out
}

// NewHandler construit le handler HTTP de l'endpoint /graphql.
// Utilisé par le serveur et par les tests, pour tester exactement la même chose.
func NewHandler(res *Resolver, src dataloader.Sources) http.Handler {
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: res}))
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.Use(extension.Introspection{})
	srv.Use(extension.FixedComplexityLimit(complexityLimit))
	srv.SetErrorPresenter(presentError)
	return auth.Middleware(res.Tokens)(dataloader.Middleware(src, srv))
}
