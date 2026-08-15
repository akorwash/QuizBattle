package repository

import (
	"errors"
	"fmt"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoQuestionRepository struct {
	collection *mongo.Collection
}

func NewMongoQuestionRepository(database *mongo.Database) *MongoQuestionRepository {
	// Keep the original collection name so existing installations do not
	// silently start reading from an empty collection after the driver upgrade.
	return &MongoQuestionRepository{collection: database.Collection("Question")}
}

func (repository *MongoQuestionRepository) GetQuestionByID(id int) (*entites.Question, error) {
	ctx, cancel := operationContext()
	defer cancel()
	var question entites.Question
	if err := repository.collection.FindOne(ctx, bson.M{"id": id}).Decode(&question); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find question: %w", err)
	}
	return &question, nil
}
