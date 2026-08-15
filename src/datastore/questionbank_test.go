package datastore

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCommittedQuestionBankIsValid(t *testing.T) {
	path := filepath.Join("..", "..", "data", "question-bank", "questions.ar.jsonl")
	items, err := LoadQuestionBank(path, time.Date(2026, 8, 15, 23, 59, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 1000 {
		t.Fatalf("expected at least 1000 questions, got %d", len(items))
	}
	categories := make(map[string]int)
	for _, item := range items {
		categories[item.Category]++
	}
	if len(categories) < 8 || categories["science"] < 100 || categories["mathematics"] < 100 || categories["religion"] < 50 {
		t.Fatalf("question bank lacks required category depth: %#v", categories)
	}
}
