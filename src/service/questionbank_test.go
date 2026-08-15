package service

import (
	"context"
	"testing"
	"time"

	"github.com/akorwash/QuizBattle/domain/question"
)

type questionBankStub struct{ items []question.Question }

func (stub questionBankStub) GetByID(context.Context, string) (*question.Question, error) {
	return nil, nil
}
func (stub questionBankStub) GetByIDs(context.Context, []string) (map[string]question.Question, error) {
	return nil, nil
}
func (stub questionBankStub) ListActive(context.Context) ([]question.Question, error) {
	return append([]question.Question(nil), stub.items...), nil
}
func (stub questionBankStub) CountActive(context.Context) (int64, error) {
	return int64(len(stub.items)), nil
}

func TestStarterQuestionsAreDiverseDeterministicAndDistinct(t *testing.T) {
	categories := []string{"science", "mathematics", "geography", "cities", "history", "civics"}
	items := make([]question.Question, 0, 30)
	for categoryIndex, category := range categories {
		for index := 0; index < 5; index++ {
			items = append(items, question.Question{ID: category + "-0000" + string(rune('a'+index)), Category: category, VerifiedAt: time.Now()})
		}
		_ = categoryIndex
	}
	service := NewQuestionBankService(questionBankStub{items: items})
	first, err := service.StarterQuestions(context.Background(), 42, 10)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.StarterQuestions(context.Background(), 42, 10)
	if err != nil {
		t.Fatal(err)
	}
	seenIDs := make(map[string]struct{}, 10)
	seenCategories := make(map[string]struct{})
	for index := range first {
		if first[index].ID != second[index].ID {
			t.Fatal("selection is not deterministic")
		}
		seenIDs[first[index].ID] = struct{}{}
		seenCategories[first[index].Category] = struct{}{}
	}
	if len(seenIDs) != 10 || len(seenCategories) < 5 {
		t.Fatalf("ids=%d categories=%d", len(seenIDs), len(seenCategories))
	}
}
