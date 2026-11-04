package prices

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Les prix actuels expirent au bout de 24 h. Si l'ingestion plante pendant
// une journée, on préfère ne plus rien afficher que des prix faux.
const currentTTL = 24 * time.Hour

// Tous les prix actuels sont dans un seul hash Redis (un champ par station).
//
// Première version : une clé par station, donc environ 9 000 commandes SET à
// chaque ingestion. L'offre gratuite d'Upstash est limitée à 500 000
// commandes par mois : avec une ingestion par heure, on l'aurait dépassée 13
// fois. Avec un hash, l'écriture complète tient en 4 commandes et la lecture
// de N stations en une seule (HMGET).
const (
	CurrentKey     = "prices:current"
	currentTempKey = "prices:current:next"
)

// Current est le prix actuel d'un carburant, tel que l'API le servira.
type Current struct {
	Fuel       string    `json:"fuel"`
	Price      float64   `json:"price"`
	UpdatedAt  time.Time `json:"updated_at"`
	OutOfStock bool      `json:"out_of_stock"`
}

type CurrentCache struct {
	rdb *redis.Client
}

func NewCurrentCache(rdb *redis.Client) *CurrentCache {
	return &CurrentCache{rdb: rdb}
}

// SaveAll remplace tous les prix actuels.
//
// On écrit dans un hash temporaire puis on le renomme : le remplacement est
// atomique. Une requête ne voit jamais un cache à moitié écrit, et une
// station sortie du flux disparaît tout de suite au lieu de rester 24 h.
func (c *CurrentCache) SaveAll(ctx context.Context, byStation map[string][]Current) error {
	if len(byStation) == 0 {
		return nil
	}
	fields := make(map[string]any, len(byStation))
	for id, list := range byStation {
		data, err := json.Marshal(list)
		if err != nil {
			return fmt.Errorf("encodage des prix de %s : %w", id, err)
		}
		fields[id] = data
	}

	// TxPipeline : les 4 commandes partent ensemble dans une transaction (MULTI/EXEC)
	_, err := c.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.Del(ctx, currentTempKey)
		p.HSet(ctx, currentTempKey, fields)
		p.Expire(ctx, currentTempKey, currentTTL)
		p.Rename(ctx, currentTempKey, CurrentKey)
		return nil
	})
	if err != nil {
		return fmt.Errorf("écriture redis : %w", err)
	}
	return nil
}

// Get lit les prix actuels d'une station. Renvoie nil si rien en cache.
func (c *CurrentCache) Get(ctx context.Context, stationID string) ([]Current, error) {
	data, err := c.rdb.HGet(ctx, CurrentKey, stationID).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var list []Current
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("prix en cache illisibles pour %s : %w", stationID, err)
	}
	return list, nil
}

// GetMany lit les prix actuels de plusieurs stations en une seule commande
// HMGET. Une station absente du cache n'apparaît pas dans le résultat.
func (c *CurrentCache) GetMany(ctx context.Context, ids []string) (map[string][]Current, error) {
	out := make(map[string][]Current, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	values, err := c.rdb.HMGet(ctx, CurrentKey, ids...).Result()
	if err != nil {
		return nil, fmt.Errorf("lecture des prix actuels : %w", err)
	}
	for i, v := range values {
		raw, ok := v.(string)
		if !ok {
			continue // nil : pas en cache
		}
		var list []Current
		if err := json.Unmarshal([]byte(raw), &list); err != nil {
			return nil, fmt.Errorf("prix en cache illisibles pour %s : %w", ids[i], err)
		}
		out[ids[i]] = list
	}
	return out, nil
}
