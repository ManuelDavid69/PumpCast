// Package users gère les comptes et leurs stations favorites.
package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const CollectionName = "users"

// Un compte ne peut pas avoir plus de favoris que ça (pour limiter les abus)
const MaxFavorites = 50

var (
	ErrEmailTaken      = errors.New("un compte existe déjà avec cet email")
	ErrNotFound        = errors.New("compte introuvable")
	ErrTooManyFavorite = fmt.Errorf("%d stations favorites au maximum", MaxFavorites)
)

type User struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	Email        string        `bson:"email"`
	PasswordHash string        `bson:"password_hash"`
	Favorites    []string      `bson:"favorites"`
	CreatedAt    time.Time     `bson:"created_at"`
}

type Repository struct {
	coll *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection(CollectionName)}
}

// Create ajoute un compte. L'unicité de l'email est garantie par un index
// unique dans MongoDB (voir db.EnsureSchema) : deux inscriptions en même
// temps avec le même email ne peuvent pas passer toutes les deux.
func (r *Repository) Create(ctx context.Context, email, passwordHash string) (*User, error) {
	u := &User{Email: email, PasswordHash: passwordHash, Favorites: []string{}, CreatedAt: time.Now().UTC()}
	res, err := r.coll.InsertOne(ctx, u)
	if mongo.IsDuplicateKeyError(err) {
		return nil, ErrEmailTaken
	}
	if err != nil {
		return nil, fmt.Errorf("création du compte : %w", err)
	}
	u.ID = res.InsertedID.(bson.ObjectID)
	return u, nil
}

func (r *Repository) find(ctx context.Context, filter bson.D) (*User, error) {
	var u User
	err := r.coll.FindOne(ctx, filter).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lecture du compte : %w", err)
	}
	return &u, nil
}

func (r *Repository) ByEmail(ctx context.Context, email string) (*User, error) {
	return r.find(ctx, bson.D{{Key: "email", Value: email}})
}

func (r *Repository) ByID(ctx context.Context, id string) (*User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, ErrNotFound
	}
	return r.find(ctx, bson.D{{Key: "_id", Value: oid}})
}

// AddFavorite ajoute une station aux favoris ($addToSet : pas de doublon).
// Le filtre sur la taille du tableau empêche de dépasser la limite même si
// deux requêtes arrivent en même temps.
func (r *Repository) AddFavorite(ctx context.Context, userID, stationID string) (*User, error) {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, ErrNotFound
	}
	filter := bson.D{
		{Key: "_id", Value: oid},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "favorites", Value: stationID}}, // déjà favori : rien ne change
			bson.D{{Key: "$expr", Value: bson.D{{Key: "$lt", Value: bson.A{bson.D{{Key: "$size", Value: "$favorites"}}, MaxFavorites}}}}},
		}},
	}
	var u User
	err = r.coll.FindOneAndUpdate(ctx, filter,
		bson.D{{Key: "$addToSet", Value: bson.D{{Key: "favorites", Value: stationID}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		if _, findErr := r.ByID(ctx, userID); findErr != nil {
			return nil, findErr
		}
		return nil, ErrTooManyFavorite
	}
	if err != nil {
		return nil, fmt.Errorf("ajout du favori : %w", err)
	}
	return &u, nil
}

func (r *Repository) RemoveFavorite(ctx context.Context, userID, stationID string) (*User, error) {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, ErrNotFound
	}
	var u User
	err = r.coll.FindOneAndUpdate(ctx, bson.D{{Key: "_id", Value: oid}},
		bson.D{{Key: "$pull", Value: bson.D{{Key: "favorites", Value: stationID}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("retrait du favori : %w", err)
	}
	return &u, nil
}

// Delete supprime le compte (ses alertes sont supprimées par l'appelant).
func (r *Repository) Delete(ctx context.Context, userID string) error {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return ErrNotFound
	}
	if _, err := r.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}}); err != nil {
		return fmt.Errorf("suppression du compte : %w", err)
	}
	return nil
}

// ByIDs sert à la vérification des alertes (envoi des emails).
func (r *Repository) ByIDs(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]User, error) {
	out := map[bson.ObjectID]User{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := r.coll.Find(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}},
		options.Find().SetProjection(bson.D{{Key: "email", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("lecture des comptes : %w", err)
	}
	var list []User
	if err := cur.All(ctx, &list); err != nil {
		return nil, fmt.Errorf("lecture des comptes : %w", err)
	}
	for _, u := range list {
		out[u.ID] = u
	}
	return out, nil
}
