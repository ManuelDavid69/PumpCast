package graph

import (
	"strings"
	"testing"

	"github.com/99designs/gqlgen/client"
)

type authResp struct {
	Token string
	User  struct{ ID, Email string }
}

func bearer(token string) client.Option {
	return client.AddHeader("Authorization", "Bearer "+token)
}

// register crée un compte et renvoie son jeton
func register(t *testing.T, env *testEnv, email string) string {
	t.Helper()
	var r struct{ Register authResp }
	env.client.MustPost(`mutation($e: String!, $p: String!) { register(email: $e, password: $p) { token user { id email } } }`,
		&r, client.Var("e", email), client.Var("p", "un-mot-de-passe-solide"))
	if r.Register.Token == "" {
		t.Fatal("pas de jeton à l'inscription")
	}
	return r.Register.Token
}

func TestRegisterAndLogin(t *testing.T) {
	env := setup(t)
	token := register(t, env, "  Manu@Example.FR ")

	// L'email est enregistré en minuscules, sans espaces
	var me struct{ Me *struct{ Email string } }
	env.client.MustPost(`{ me { email } }`, &me, bearer(token))
	if me.Me == nil || me.Me.Email != "manu@example.fr" {
		t.Fatalf("me = %+v", me.Me)
	}

	// Sans jeton, ou avec un jeton bidon : me vaut null, sans erreur
	me.Me = nil
	env.client.MustPost(`{ me { email } }`, &me)
	env.client.MustPost(`{ me { email } }`, &me, bearer("n.importe.quoi"))
	if me.Me != nil {
		t.Error("me doit être null sans jeton valide")
	}

	// Même email une 2e fois (avec une casse différente)
	var dup struct{ Register authResp }
	err := env.client.Post(`mutation { register(email: "manu@example.fr", password: "un-autre-mot-de-passe") { token } }`, &dup)
	if err == nil || !strings.Contains(err.Error(), "existe déjà") {
		t.Errorf("doublon d'email : %v", err)
	}

	var login struct{ Login authResp }
	env.client.MustPost(`mutation { login(email: "MANU@example.fr", password: "un-mot-de-passe-solide") { token } }`, &login)
	if login.Login.Token == "" {
		t.Error("la connexion doit renvoyer un jeton")
	}

	// Mauvais mot de passe et compte inconnu : exactement la même erreur
	errWrong := env.client.Post(`mutation { login(email: "manu@example.fr", password: "pas-le-bon-mot") { token } }`, &login)
	errUnknown := env.client.Post(`mutation { login(email: "inconnu@example.fr", password: "pas-le-bon-mot") { token } }`, &login)
	if errWrong == nil || errUnknown == nil || errWrong.Error() != errUnknown.Error() {
		t.Errorf("les deux erreurs doivent être identiques : %v / %v", errWrong, errUnknown)
	}
}

func TestRegisterValidation(t *testing.T) {
	env := setup(t)
	var r struct{ Register authResp }
	for _, tt := range []struct{ email, password, want string }{
		{"pas-un-email", "un-mot-de-passe-solide", "email invalide"},
		{"a@b.fr", "court", "au moins 10 caractères"},
	} {
		err := env.client.Post(`mutation($e: String!, $p: String!) { register(email: $e, password: $p) { token } }`,
			&r, client.Var("e", tt.email), client.Var("p", tt.password))
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("register(%q) : %v, attendu %q", tt.email, err, tt.want)
		}
	}
}

func TestFavoritesAndAlerts(t *testing.T) {
	env := setup(t)
	token := register(t, env, "manu@example.fr")

	// Pas connecté : refusé
	var fav struct {
		AddFavorite struct{ Favorites []struct{ ID string } }
	}
	if err := env.client.Post(`mutation { addFavorite(stationId: "A") { email } }`, &fav); err == nil || !strings.Contains(err.Error(), "connecté") {
		t.Errorf("favori sans compte : %v", err)
	}

	env.client.MustPost(`mutation { addFavorite(stationId: "B") { favorites { id } } }`, &fav, bearer(token))
	env.client.MustPost(`mutation { addFavorite(stationId: "A") { favorites { id } } }`, &fav, bearer(token))
	env.client.MustPost(`mutation { addFavorite(stationId: "A") { favorites { id } } }`, &fav, bearer(token)) // doublon ignoré
	if got := ids2(fav.AddFavorite.Favorites); got != "B,A" {
		t.Errorf("favoris = %s, attendu B,A", got)
	}
	if err := env.client.Post(`mutation { addFavorite(stationId: "inexistante") { email } }`, &fav, bearer(token)); err == nil {
		t.Error("une station inconnue doit être refusée")
	}

	// Alerte de seuil : le seuil est obligatoire et doit être un prix plausible
	var created struct{ CreateAlert struct{ ID string } }
	err := env.client.Post(`mutation { createAlert(input: {stationId: "A", fuel: GAZOLE, type: THRESHOLD, threshold: 12}) { id } }`, &created, bearer(token))
	if err == nil || !strings.Contains(err.Error(), "seuil") {
		t.Errorf("seuil absurde : %v", err)
	}
	env.client.MustPost(`mutation { createAlert(input: {stationId: "A", fuel: GAZOLE, type: THRESHOLD, threshold: 1.75}) { id } }`, &created, bearer(token))

	var me struct {
		Me struct {
			Alerts []struct {
				ID        string
				Type      string
				Threshold *float64
				Station   struct{ ID string }
			}
		}
	}
	env.client.MustPost(`{ me { alerts { id type threshold station { id } } } }`, &me, bearer(token))
	if len(me.Me.Alerts) != 1 || me.Me.Alerts[0].Station.ID != "A" || *me.Me.Alerts[0].Threshold != 1.75 {
		t.Fatalf("alertes = %+v", me.Me.Alerts)
	}

	// Un autre compte ne peut pas supprimer cette alerte en connaissant son id
	other := register(t, env, "autre@example.fr")
	var del struct{ DeleteAlert bool }
	if err := env.client.Post(`mutation($id: ID!) { deleteAlert(id: $id) }`, &del, client.Var("id", created.CreateAlert.ID), bearer(other)); err == nil {
		t.Error("supprimer l'alerte d'un autre compte doit échouer")
	}
	env.client.MustPost(`mutation($id: ID!) { deleteAlert(id: $id) }`, &del, client.Var("id", created.CreateAlert.ID), bearer(token))
	if !del.DeleteAlert {
		t.Error("le propriétaire doit pouvoir supprimer son alerte")
	}
}

func TestDeleteAccount(t *testing.T) {
	env := setup(t)
	token := register(t, env, "manu@example.fr")
	var r struct{ DeleteAccount bool }
	if err := env.client.Post(`mutation { deleteAccount(password: "pas-le-bon") }`, &r, bearer(token)); err == nil {
		t.Error("le mot de passe doit être vérifié")
	}
	env.client.MustPost(`mutation { deleteAccount(password: "un-mot-de-passe-solide") }`, &r, bearer(token))
	// Le jeton est toujours valide mais le compte n'existe plus
	var me struct{ Me *struct{ Email string } }
	env.client.MustPost(`{ me { email } }`, &me, bearer(token))
	if me.Me != nil {
		t.Error("me doit être null après suppression du compte")
	}
}

func ids2(list []struct{ ID string }) string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = s.ID
	}
	return strings.Join(out, ",")
}
