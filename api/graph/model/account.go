package model

// User est écrit à la main : favoris et alertes sont des resolvers, lus
// seulement si le client les demande.
type User struct {
	ID          string
	Email       string
	CreatedAt   string
	FavoriteIDs []string
}

// Alert porte directement sa station : les stations de toutes les alertes
// d'un compte sont lues en une seule requête (voir le resolver User.alerts).
type Alert struct {
	ID         string
	Station    *Station
	Fuel       Fuel
	Type       AlertType
	Threshold  *float64
	Active     bool
	LastSentAt *string
	CreatedAt  string
}
