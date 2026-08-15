package datastore

import (
	"context"
	"fmt"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// SeedQuestions adds the minimal development question set without modifying
// existing records. It must be explicitly enabled through configuration.
func SeedQuestions(ctx context.Context, database *mongo.Database) error {
	questions := []entites.Question{
		{ID: 1, Header: "اين صنعت أول كسوة للكعبة؟", Answers: []entites.Answer{
			{ID: 1, Text: "تونس"},
			{ID: 2, Text: "مصر", IsCorrect: true},
			{ID: 3, Text: "السعودية"},
			{ID: 4, Text: "العراق"},
		}},
		{ID: 2, Header: "في اي مدينة يتواجد سوق عكاظ؟", Answers: []entites.Answer{
			{ID: 1, Text: "الطائف", IsCorrect: true},
			{ID: 2, Text: "الدمام"},
			{ID: 3, Text: "الخبر"},
			{ID: 4, Text: "الرياض"},
		}},
		{ID: 4, Header: "ماهو اطول جسر بحري في العالم؟", Answers: []entites.Answer{
			{ID: 1, Text: "جسر السلطان سليم الأول"},
			{ID: 2, Text: "جسر هايوان كوينغداو"},
			{ID: 3, Text: "جسر دانيانغ-كونشان"},
			{ID: 4, Text: "جسر الملك فهد", IsCorrect: true},
		}},
	}
	collection := database.Collection("Question")
	for _, question := range questions {
		_, err := collection.UpdateOne(
			ctx,
			bson.M{"id": question.ID},
			bson.M{"$setOnInsert": question},
			options.UpdateOne().SetUpsert(true),
		)
		if err != nil {
			return fmt.Errorf("seed question %d: %w", question.ID, err)
		}
	}
	return nil
}
