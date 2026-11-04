// Package alerts gère les alertes des utilisateurs et leur envoi par email.
package alerts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const CollectionName = "alerts"

// Nombre maximum d'alertes par compte
const MaxPerUser = 20

// Types d'alerte
const (
	TypeThreshold     = "threshold"      // le prix passe sous un seuil
	TypePredictedDrop = "predicted_drop" // le modèle conseille d'attendre
	TypePredictedRise = "predicted_rise" // le modèle conseille de faire le plein
)

var (
	ErrNotFound     = errors.New("alerte introuvable")
	ErrTooManyAlert = fmt.Errorf("%d alertes au maximum", MaxPerUser)
)

type Alert struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	UserID    bson.ObjectID `bson:"user_id"`
	StationID string        `bson:"station_id"`
	Fuel      string        `bson:"fuel"`
	Type      string        `bson:"type"`
	Threshold *float64      `bson:"threshold,omitempty"`
	Active    bool          `bson:"active"`
	// Triggered vaut true tant que la condition reste vraie : on n'envoie un
	// email qu'au moment où elle devient vraie, pas à chaque vérification
	Triggered  bool       `bson:"triggered"`
	LastSentAt *time.Time `bson:"last_sent_at,omitempty"`
	CreatedAt  time.Time  `bson:"created_at"`
}

type Repository struct {
	coll *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection(CollectionName)}
}

// Create ajoute une alerte, dans la limite de MaxPerUser par compte.
func (r *Repository) Create(ctx context.Context, a Alert) (*Alert, error) {
	n, err := r.coll.CountDocuments(ctx, bson.D{{Key: "user_id", Value: a.UserID}})
	if err != nil {
		return nil, fmt.Errorf("comptage des alertes : %w", err)
	}
	if n >= MaxPerUser {
		return nil, ErrTooManyAlert
	}
	a.Active = true
	a.CreatedAt = time.Now().UTC()
	res, err := r.coll.InsertOne(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("création de l'alerte : %w", err)
	}
	a.ID = res.InsertedID.(bson.ObjectID)
	return &a, nil
}

func (r *Repository) ByUser(ctx context.Context, userID bson.ObjectID) ([]Alert, error) {
	cur, err := r.coll.Find(ctx, bson.D{{Key: "user_id", Value: userID}},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("lecture des alertes : %w", err)
	}
	list := []Alert{}
	if err := cur.All(ctx, &list); err != nil {
		return nil, fmt.Errorf("lecture des alertes : %w", err)
	}
	return list, nil
}

// Delete supprime une alerte. Le filtre sur user_id empêche de supprimer
// l'alerte de quelqu'un d'autre en devinant son identifiant.
func (r *Repository) Delete(ctx context.Context, userID bson.ObjectID, alertID string) error {
	oid, err := bson.ObjectIDFromHex(alertID)
	if err != nil {
		return ErrNotFound
	}
	res, err := r.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}, {Key: "user_id", Value: userID}})
	if err != nil {
		return fmt.Errorf("suppression de l'alerte : %w", err)
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteByUser(ctx context.Context, userID bson.ObjectID) error {
	if _, err := r.coll.DeleteMany(ctx, bson.D{{Key: "user_id", Value: userID}}); err != nil {
		return fmt.Errorf("suppression des alertes : %w", err)
	}
	return nil
}

// Active renvoie toutes les alertes actives, pour la vérification.
func (r *Repository) Active(ctx context.Context) ([]Alert, error) {
	cur, err := r.coll.Find(ctx, bson.D{{Key: "active", Value: true}})
	if err != nil {
		return nil, fmt.Errorf("lecture des alertes actives : %w", err)
	}
	list := []Alert{}
	if err := cur.All(ctx, &list); err != nil {
		return nil, fmt.Errorf("lecture des alertes actives : %w", err)
	}
	return list, nil
}

// SetState enregistre le nouvel état de l'alerte après vérification.
func (r *Repository) SetState(ctx context.Context, id bson.ObjectID, triggered bool, sentAt *time.Time) error {
	set := bson.D{{Key: "triggered", Value: triggered}}
	if sentAt != nil {
		set = append(set, bson.E{Key: "last_sent_at", Value: *sentAt})
	}
	if _, err := r.coll.UpdateByID(ctx, id, bson.D{{Key: "$set", Value: set}}); err != nil {
		return fmt.Errorf("mise à jour de l'alerte : %w", err)
	}
	return nil
}
