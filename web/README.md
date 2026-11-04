# PumpCast, front

Next.js (App Router), TypeScript, Tailwind CSS, Apollo Client, Leaflet et Recharts.

```bash
cp .env.example .env.local
npm install
npm run dev      # http://localhost:3000, l'API Go doit tourner sur le port 8080
npm test         # tests des fonctions de mise en forme
npm run build
```

Les pages stations, comparaison et départements sont rendues côté serveur avec un cache de quelques minutes. La carte de l'accueil est rendue dans le navigateur car elle dépend de la position de l'utilisateur.
