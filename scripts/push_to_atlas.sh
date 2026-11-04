#!/usr/bin/env bash
# Copie la base locale vers MongoDB Atlas, une seule fois au déploiement.
#
# Plutôt que de relancer l'import des archives sur Atlas (l'offre gratuite
# limite à 100 opérations par seconde), on copie les données déjà calculées
# et vérifiées en local : stations, historique des prix, moyennes par
# département, marché, calendrier, prévisions et scores du modèle.
# Les comptes et les alertes de test ne sont pas copiés.
#
# Usage, depuis la racine du dépôt, avec docker compose lancé :
#   ATLAS_URI='mongodb+srv://...' ./scripts/push_to_atlas.sh
set -euo pipefail

if [ -z "${ATLAS_URI:-}" ]; then
  echo "Il faut définir ATLAS_URI (l'URI de connexion donnée par Atlas)" >&2
  exit 1
fi

DUMP=/tmp/pumpcast-dump
docker exec pumpcast-mongo rm -rf "$DUMP"

echo "1/2 Export de la base locale..."
docker exec pumpcast-mongo mongodump --quiet --db pumpcast --out "$DUMP" \
  --excludeCollection users --excludeCollection alerts

echo "2/2 Envoi vers Atlas (quelques minutes)..."
# --drop : remplace les collections si le script est relancé.
# L'URI est passée par une variable d'environnement pour ne pas apparaître
# dans la liste des processus.
docker exec -e ATLAS_URI="$ATLAS_URI" pumpcast-mongo sh -c \
  'mongorestore --quiet --drop --uri "$ATLAS_URI" --nsInclude "pumpcast.*" "'"$DUMP"'"'

docker exec pumpcast-mongo rm -rf "$DUMP"
echo "Terminé. Les index manquants sont recréés au démarrage de l'API."
