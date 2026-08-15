package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const sessionRevocationCollection = "SessionRevocation"

const sessionOperationTimeout = 2 * time.Second

type MongoSessionRepository struct {
	collection *mongo.Collection
}

type sessionRevocationDocument struct {
	TokenID   string    `bson:"tokenid"`
	ExpiresAt time.Time `bson:"expiresat"`
}

func NewMongoSessionRepository(database *mongo.Database) *MongoSessionRepository {
	return &MongoSessionRepository{collection: database.Collection(sessionRevocationCollection)}
}

func (repository *MongoSessionRepository) SaveSessionRevocation(tokenID string, expiresAt time.Time) error {
	if tokenID == "" || expiresAt.IsZero() {
		return fmt.Errorf("save session revocation: invalid token metadata")
	}
	ctx, cancel := context.WithTimeout(context.Background(), sessionOperationTimeout)
	defer cancel()
	_, err := repository.collection.UpdateOne(
		ctx,
		bson.M{"tokenid": tokenID},
		bson.M{"$set": sessionRevocationDocument{TokenID: tokenID, ExpiresAt: expiresAt.UTC()}},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("save session revocation: %w", err)
	}
	return nil
}

func (repository *MongoSessionRepository) IsSessionRevoked(tokenID string) (bool, error) {
	if tokenID == "" {
		return true, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), sessionOperationTimeout)
	defer cancel()
	var document sessionRevocationDocument
	err := repository.collection.FindOne(ctx, bson.M{
		"tokenid":   tokenID,
		"expiresat": bson.M{"$gt": time.Now().UTC()},
	}).Decode(&document)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check session revocation: %w", err)
	}
	return true, nil
}
