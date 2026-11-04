# Déploiement

Tout tourne sur des offres gratuites. Limites vérifiées en novembre 2026 :

| Service | Rôle | Limite gratuite qui compte pour PumpCast |
|---|---|---|
| MongoDB Atlas (Free) | Base de données | 512 Mo (données + index), 100 opérations par seconde |
| Upstash Redis | Cache, rate limiting | 500 000 commandes par mois, 10 Go de transfert |
| Render (Free) | API Go | S'endort après 15 min sans visite, environ 1 min pour se réveiller |
| Vercel (Hobby) | Front Next.js | Usage non commercial |
| GitHub Actions | Ingestion, prévisions, entraînement | Gratuit pour un dépôt public |
| Brevo | Emails d'alerte | 300 emails par jour |

La limite d'Upstash a changé le code : les prix actuels sont rangés dans un seul hash Redis, l'ingestion coûte 4 commandes au lieu de 9 000 (voir `api/internal/prices/current.go`).

## 1. MongoDB Atlas

1. Créer un cluster gratuit, région Paris ou Francfort.
2. Database Access : créer un utilisateur avec un mot de passe généré.
3. Network Access : autoriser `0.0.0.0/0`. GitHub Actions et Render n'ont pas d'IP fixe, la protection repose donc sur le mot de passe.
4. Copier l'URI `mongodb+srv://...`.
5. Envoyer les données locales (stations, historique, modèle) vers Atlas, une seule fois :

```bash
ATLAS_URI='mongodb+srv://...' ./scripts/push_to_atlas.sh
```

## 2. Upstash

Créer une base Redis, région `eu-west-1`, et copier l'URL `rediss://...` (avec deux s : connexion chiffrée).

## 3. Render (API)

New > Blueprint, choisir le dépôt : `render.yaml` décrit le service. Render demande ensuite les valeurs à remplir :

| Variable | Valeur |
|---|---|
| `MONGO_URI` | l'URI Atlas |
| `REDIS_URL` | l'URL Upstash |
| `INTERNAL_API_KEY` | une longue chaîne aléatoire (`openssl rand -base64 32`), la même que sur Vercel |
| `CORS_ORIGINS` | l'adresse du site sur Vercel, par exemple `https://pumpcast.vercel.app` |

`JWT_SECRET` est généré par Render.

## 4. Vercel (front)

Importer le dépôt avec `web` comme dossier racine, puis ajouter :

| Variable | Valeur |
|---|---|
| `NEXT_PUBLIC_GRAPHQL_URL` | `https://<service>.onrender.com/graphql` |
| `GRAPHQL_URL` | la même adresse |
| `INTERNAL_API_KEY` | la même valeur que sur Render |

## 5. Brevo

Vérifier une adresse d'envoi et créer une clé API. Sans nom de domaine à soi, les emails envoyés depuis une adresse Gmail risquent d'arriver en spam.

## 6. GitHub Actions

Settings > Secrets and variables > Actions.

Secrets :

| Nom | Valeur |
|---|---|
| `MONGO_URI` | l'URI Atlas |
| `REDIS_URL` | l'URL Upstash |
| `BREVO_API_KEY` | la clé Brevo |

Variables (pas secrètes) :

| Nom | Valeur |
|---|---|
| `SITE_URL` | l'adresse du site sur Vercel |
| `ALERT_FROM_EMAIL` | l'adresse d'envoi vérifiée dans Brevo |

Ensuite, onglet Actions : lancer une fois « Entraînement du modèle » à la main (il dépose les modèles que « Prévisions du jour » réutilise chaque matin). L'ingestion démarre toute seule toutes les heures.

À savoir : GitHub met en pause les workflows planifiés d'un dépôt public sans activité depuis 60 jours. Un commit ou un lancement manuel les relance.
