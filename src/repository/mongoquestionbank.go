package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/akorwash/QuizBattle/domain/question"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const questionBankCollection = "QuestionBank"

type MongoQuestionBankRepository struct {
	collection *mongo.Collection
}

func NewMongoQuestionBankRepository(database *mongo.Database) *MongoQuestionBankRepository {
	return &MongoQuestionBankRepository{collection: database.Collection(questionBankCollection)}
}

func (repository *MongoQuestionBankRepository) Import(ctx context.Context, questions []question.Question) error {
	if len(questions) == 0 {
		return fmt.Errorf("question bank is empty")
	}
	const batchSize = 500
	for start := 0; start < len(questions); start += batchSize {
		end := min(start+batchSize, len(questions))
		models := make([]mongo.WriteModel, 0, end-start)
		for _, item := range questions[start:end] {
			models = append(models, mongo.NewUpdateOneModel().
				SetFilter(bson.M{"id": item.ID}).
				SetUpdate(bson.M{"$set": item}).
				SetUpsert(true))
		}
		if _, err := repository.collection.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false)); err != nil {
			return fmt.Errorf("import question batch %d: %w", start/batchSize+1, err)
		}
	}
	return nil
}

func (repository *MongoQuestionBankRepository) GetByID(ctx context.Context, id string) (*question.Question, error) {
	var item question.Question
	if err := repository.collection.FindOne(ctx, bson.M{"id": id, "status": question.StatusActive}).Decode(&item); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get question: %w", err)
	}
	return &item, nil
}

func (repository *MongoQuestionBankRepository) GetByIDs(ctx context.Context, ids []string) (map[string]question.Question, error) {
	result := make(map[string]question.Question, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	cursor, err := repository.collection.Find(ctx, bson.M{"id": bson.M{"$in": ids}, "status": question.StatusActive})
	if err != nil {
		return nil, fmt.Errorf("find questions: %w", err)
	}
	defer cursor.Close(ctx)
	var items []question.Question
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("decode questions: %w", err)
	}
	for _, item := range items {
		result[item.ID] = item
	}
	return result, nil
}

func (repository *MongoQuestionBankRepository) ListActive(ctx context.Context) ([]question.Question, error) {
	cursor, err := repository.collection.Find(
		ctx,
		bson.M{"status": question.StatusActive},
		options.Find().SetSort(bson.D{{Key: "category", Value: 1}, {Key: "id", Value: 1}}).SetLimit(5000),
	)
	if err != nil {
		return nil, fmt.Errorf("list active questions: %w", err)
	}
	defer cursor.Close(ctx)
	items := make([]question.Question, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("decode active questions: %w", err)
	}
	return items, nil
}

func (repository *MongoQuestionBankRepository) CountActive(ctx context.Context) (int64, error) {
	count, err := repository.collection.CountDocuments(ctx, bson.M{"status": question.StatusActive})
	if err != nil {
		return 0, fmt.Errorf("count active questions: %w", err)
	}
	return count, nil
}
