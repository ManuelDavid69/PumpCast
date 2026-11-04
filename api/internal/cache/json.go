package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Préfixe des clés de statistiques. L'ingestion les supprime toutes après
// avoir recalculé les moyennes, pour que l'API ne serve pas d'anciens chiffres.
const StatsPrefix = "stats:"

// Les statistiques changent seulement à chaque ingestion : une heure de cache
// suffit, et l'ingestion vide le cache de toute façon.
const StatsTTL = time.Hour

// GetJSON lit une valeur en cache. found vaut false si la clé n'existe pas.
func GetJSON(ctx context.Context, rdb *redis.Client, key string, dest any) (found bool, err error) {
	data, err := rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, fmt.Errorf("cache illisible pour %s : %w", key, err)
	}
	return true, nil
}

// SetJSON met une valeur en cache pour la durée ttl.
func SetJSON(ctx context.Context, rdb *redis.Client, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, key, data, ttl).Err()
}

// DeletePrefix supprime toutes les clés qui commencent par prefix.
// SCAN parcourt les clés par paquets sans bloquer Redis (contrairement à KEYS).
func DeletePrefix(ctx context.Context, rdb *redis.Client, prefix string) (int, error) {
	var deleted int
	iter := rdb.Scan(ctx, 0, prefix+"*", 500).Iterator()
	var batch []string
	for iter.Next(ctx) {
		batch = append(batch, iter.Val())
		if len(batch) == 500 {
			if err := rdb.Del(ctx, batch...).Err(); err != nil {
				return deleted, err
			}
			deleted += len(batch)
			batch = batch[:0]
		}
	}
	if err := iter.Err(); err != nil {
		return deleted, err
	}
	if len(batch) > 0 {
		if err := rdb.Del(ctx, batch...).Err(); err != nil {
			return deleted, err
		}
		deleted += len(batch)
	}
	return deleted, nil
}
