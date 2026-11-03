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

// CurrentKey est la clé Redis des prix actuels d'une station.
func CurrentKey(stationID string) string {
	return "prices:current:" + stationID
}

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

// SaveAll écrit les prix actuels de toutes les stations.
// Le pipeline envoie toutes les commandes d'un coup au lieu d'attendre
// la réponse de Redis entre chaque station.
func (c *CurrentCache) SaveAll(ctx context.Context, byStation map[string][]Current) error {
	const batchSize = 1000

	pipe := c.rdb.Pipeline()
	n := 0
	for id, list := range byStation {
		data, err := json.Marshal(list)
		if err != nil {
			return fmt.Errorf("encodage des prix de %s : %w", id, err)
		}
		pipe.Set(ctx, CurrentKey(id), data, currentTTL)
		n++
		if n%batchSize == 0 {
			if _, err := pipe.Exec(ctx); err != nil {
				return fmt.Errorf("écriture redis : %w", err)
			}
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("écriture redis : %w", err)
	}
	return nil
}

// Get lit les prix actuels d'une station. Renvoie nil si rien en cache.
func (c *CurrentCache) Get(ctx context.Context, stationID string) ([]Current, error) {
	data, err := c.rdb.Get(ctx, CurrentKey(stationID)).Bytes()
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
