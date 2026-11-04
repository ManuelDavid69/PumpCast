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

Lancer l'API (route `/health` pour l'instant) :

```bash
go run ./cmd/server
```

Tests (le test sur la time series a besoin du MongoDB local, il est ignoré sinon) :

```bash
MONGO_TEST_URI=mongodb://localhost:27017 go test ./...
docker compose --profile ml run --rm --no-deps ml python -m pytest -q
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
