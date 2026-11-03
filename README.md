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

Lancer l'API (route `/health` pour l'instant) :

```bash
go run ./cmd/server
```

Tests (le test sur la time series a besoin du MongoDB local, il est ignoré sinon) :

```bash
MONGO_TEST_URI=mongodb://localhost:27017 go test ./...
```

## Sources des données

- Prix des carburants en France, flux instantané (prix-carburants.gouv.fr / data.economie.gouv.fr), Licence Ouverte 2.0
