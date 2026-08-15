package repository

import (
	"context"
	"errors"
	"fmt"

	avatardomain "github.com/akorwash/QuizBattle/domain/avatar"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const userAvatarCollection = "UserAvatar"

type MongoAvatarRepository struct {
	collection *mongo.Collection
}

func NewMongoAvatarRepository(database *mongo.Database) *MongoAvatarRepository {
	return &MongoAvatarRepository{collection: database.Collection(userAvatarCollection)}
}

// Save atomically creates or replaces the single avatar owned by a user.
func (repository *MongoAvatarRepository) Save(ctx context.Context, avatar *avatardomain.Image) error {
	if repository == nil || repository.collection == nil || avatar == nil || avatar.ValidateStored() != nil {
		return fmt.Errorf("save avatar: %w", avatardomain.ErrInvalidImage)
	}
	operation, cancel := boundedAvatarContext(ctx)
	defer cancel()
	_, err := repository.collection.UpdateOne(
		operation,
		bson.M{"userId": avatar.UserID},
		bson.M{"$set": bson.M{
			"contentType": avatar.ContentType,
			"data":        avatar.Data,
			"etag":        avatar.ETag,
			"width":       avatar.Width,
			"height":      avatar.Height,
			"byteSize":    avatar.ByteSize,
			"updatedAt":   avatar.UpdatedAt,
		}},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrConflict
		}
		return fmt.Errorf("save avatar: %w", err)
	}
	return nil
}

func (repository *MongoAvatarRepository) GetByUserID(ctx context.Context, userID int64) (*avatardomain.Image, error) {
	if repository == nil || repository.collection == nil || userID <= 0 {
		return nil, ErrNotFound
	}
	operation, cancel := boundedAvatarContext(ctx)
	defer cancel()
	var avatar avatardomain.Image
	if err := repository.collection.FindOne(operation, bson.M{"userId": userID}).Decode(&avatar); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get avatar: %w", err)
	}
	if err := avatar.ValidateStored(); err != nil {
		return nil, fmt.Errorf("get avatar: %w", err)
	}
	return &avatar, nil
}

// DeleteByUserID is intentionally idempotent. Removing an absent avatar has
// the same externally visible result as removing an existing one.
func (repository *MongoAvatarRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	if repository == nil || repository.collection == nil || userID <= 0 {
		return ErrNotFound
	}
	operation, cancel := boundedAvatarContext(ctx)
	defer cancel()
	if _, err := repository.collection.DeleteOne(operation, bson.M{"userId": userID}); err != nil {
		return fmt.Errorf("delete avatar: %w", err)
	}
	return nil
}

func boundedAvatarContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, operationTimeout)
}
