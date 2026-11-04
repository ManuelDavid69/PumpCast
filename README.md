# PumpCast

Appli web qui prédit l'évolution du prix des carburants station par station, pour savoir s'il vaut mieux faire le plein maintenant ou attendre quelques jours.

Projet perso en cours de développement (Go, GraphQL, MongoDB, Redis, Next.js, scikit-learn).

## Lancer en local

Prérequis : Docker et Go 1.27+.

```bash
cp .env.example .env
docker compose up -d
```

Ça démarre MongoDB (port 27017) et Redis (port 6379).

Récupérer les prix du jour (à relancer quand on veut, ça ne crée pas de doublons) :

```bash
cd api
go run ./cmd/ingest
```

Importer l'historique depuis les archives annuelles (environ 2 min, les zips sont gardés dans `data/archives/`) :

```bash
go run ./cmd/importhistory -from 2023 -to 2026
```

Les archives ne donnent que les changements de prix. L'import reconstruit un prix par jour (le dernier prix connu), arrête de reporter un prix resté 30 jours sans mise à jour et saute les jours de rupture. 2022 n'est pas importée par défaut : la remise carburant de l'État d'avril à décembre crée des sauts de prix artificiels.

Récupérer les données externes (Brent, EUR/USD, jours fériés, vacances scolaires) dans l'image Python du ML :

```bash
docker compose --profile ml run --rm ml
```

Ouvrir le notebook d'exploration (Jupyter Lab sur http://localhost:8888, le lien avec le token s'affiche dans le terminal) :

```bash
docker compose --profile ml up notebook
```

Lancer l'API :

```bash
go run ./cmd/server
```

L'API GraphQL répond sur `http://localhost:8080/graphql` et une page pour tester les requêtes est sur `http://localhost:8080/playground`. Exemple, les 5 stations les moins chères en gazole autour de Villeurbanne avec leur historique :

```graphql
{
  stationsNear(lat: 45.7712, lng: 4.8835, radiusKm: 5, fuel: GAZOLE, sort: PRICE, limit: 5) {
    address
    city
    distanceKm
    currentPrices { fuel price outOfStock }
    priceHistory(fuel: GAZOLE, days: 30) { date price }
  }
}
```

Lancer le front (Next.js, sur http://localhost:3000) :

```bash
cd web
cp .env.example .env.local
npm install
npm run dev
```

Tests (les tests d'intégration ont besoin du MongoDB et du Redis locaux, ils sont ignorés sinon) :

```bash
MONGO_TEST_URI=mongodb://localhost:27017 REDIS_TEST_URL=redis://localhost:6379/15 go test ./...
docker compose --profile ml run --rm --no-deps ml python -m pytest -q
cd web && npm test
```

## Sources des données

- Prix des carburants en France, flux instantané et archives annuelles (prix-carburants.gouv.fr / data.economie.gouv.fr), Licence Ouverte 2.0
- Prix du Brent : FRED, Federal Reserve Bank of St. Louis (série DCOILBRENTEU)
- Taux de change EUR/USD : Banque centrale européenne
- Jours fériés : calendrier.api.gouv.fr
- Vacances scolaires : data.education.gouv.fr (calendrier scolaire)

## Pourquoi l'historique est limité à Auvergne-Rhône-Alpes

L'hébergement gratuit de MongoDB (Atlas M0) est limité à 512 Mo. Les prix actuels sont gardés pour toute la France (dans Redis), mais l'historique jour par jour ne l'est que pour les départements de `HISTORY_DEPARTMENTS` (AURA par défaut, environ 2 000 stations).

Mesuré en local : de 2023 à novembre 2026, ça fait 5,7 millions de relevés, 87 Mo de données et 48 Mo réellement sur le disque grâce à la compression des collections time series. On a donc de la marge pour ajouter des années ou des départements plus tard.

## Quelques choix sur l'API

- **GraphQL** parce que chaque page a besoin de données très différentes (prix actuels sur l'accueil, historique et prédiction sur la fiche station, plusieurs courbes pour la comparaison). Le client demande juste les champs qu'il affiche, en une requête.
- **Dataloaders** pour éviter le problème N+1 : pour 20 stations avec leur historique, on fait une requête MongoDB avec les 20 identifiants au lieu de 20 requêtes. Un test compte les requêtes envoyées à MongoDB pour le vérifier.
- **Cache Redis** : prix actuels (mis à jour à chaque ingestion) et statistiques par département (une heure, vidé par l'ingestion).
- **Rate limiting** : 120 requêtes par minute et par IP, compteur dans Redis. L'IP vient de la connexion, ou de la dernière entrée de `X-Forwarded-For` derrière le proxy de Render (les autres entrées peuvent être inventées par le client).
- **Limite de complexité** : une requête GraphQL trop lourde est refusée avant d'être exécutée, et les paramètres sont bornés (rayon 50 km, 100 stations, 365 jours d'historique, 5 stations à comparer).

## Quelques choix sur le front

- **Rendu côté serveur** pour les fiches stations, la comparaison et les départements : la page arrive déjà remplie, rapide à afficher et lisible par Google. Les données sont gardées en cache quelques minutes (`'use cache'` de Next.js), les prix ne changent pas plus vite dans le flux officiel.
- **Apollo Client** seulement pour la carte de l'accueil, qui dépend de la position de l'utilisateur et relance des requêtes quand on change de ville ou de carburant.
- **Courbes en escalier** : un prix ne bouge pas entre deux changements, une courbe lissée inventerait des prix intermédiaires qui n'ont jamais existé.
- **Recherche de ville** avec le service de géocodage officiel de l'IGN (Géoplateforme), gratuit et sans clé.

