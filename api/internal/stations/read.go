package stations

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Nearby est une station avec sa distance au point recherché.
type Nearby struct {
	Station   `bson:",inline"`
	DistanceM float64 `bson:"distance_m"`
}

// Near renvoie les stations à moins de maxMeters du point, de la plus proche
// à la plus lointaine. $geoNear utilise l'index 2dsphere et calcule la
// distance sur la sphère terrestre.
func (r *Repository) Near(ctx context.Context, lat, lng, maxMeters float64, limit int) ([]Nearby, error) {
	pipeline := bson.A{
		bson.D{{Key: "$geoNear", Value: bson.D{
			{Key: "near", Value: bson.D{{Key: "type", Value: "Point"}, {Key: "coordinates", Value: bson.A{lng, lat}}}},
			{Key: "distanceField", Value: "distance_m"},
			{Key: "maxDistance", Value: maxMeters},
			{Key: "spherical", Value: true},
		}}},
		bson.D{{Key: "$limit", Value: limit}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("stations proches : %w", err)
	}
	var list []Nearby
	if err := cur.All(ctx, &list); err != nil {
		return nil, fmt.Errorf("stations proches : %w", err)
	}
	return list, nil
}

// ByIDs renvoie les stations demandées, indexées par identifiant.
func (r *Repository) ByIDs(ctx context.Context, ids []string) (map[string]Station, error) {
	if len(ids) == 0 {
		return map[string]Station{}, nil
	}
	cur, err := r.coll.Find(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}})
	if err != nil {
		return nil, fmt.Errorf("lecture des stations : %w", err)
	}
	var list []Station
	if err := cur.All(ctx, &list); err != nil {
		return nil, fmt.Errorf("lecture des stations : %w", err)
	}
	m := make(map[string]Station, len(list))
	for _, s := range list {
		m[s.ID] = s
	}
	return m, nil
}

// IDsByDepartment renvoie les identifiants des stations d'un département.
func (r *Repository) IDsByDepartment(ctx context.Context, department string) ([]string, error) {
	cur, err := r.coll.Find(ctx, bson.D{{Key: "department", Value: department}},
		options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("stations du département : %w", err)
	}
	var docs []struct {
		ID string `bson:"_id"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("stations du département : %w", err)
	}
	ids := make([]string, len(docs))
	for i, d := range docs {
		ids[i] = d.ID
	}
	return ids, nil
}
