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

## Sources des données

- Prix des carburants en France, flux instantané (prix-carburants.gouv.fr / data.economie.gouv.fr), Licence Ouverte 2.0
