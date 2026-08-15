package question

import (
	"errors"
	"testing"
	"time"
)

func TestQuestionValidationAndHash(t *testing.T) {
	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	item := validQuestion(now)
	if err := item.SetContentHash(); err != nil {
		t.Fatal(err)
	}
	if len(item.ContentHash) != 64 {
		t.Fatalf("unexpected hash %q", item.ContentHash)
	}
	if err := item.Validate(now); err != nil {
		t.Fatal(err)
	}
	item.Options[0] = "إجابة معدلة"
	if err := item.Validate(now); !errors.Is(err, ErrInvalidQuestion) {
		t.Fatalf("tampered item accepted: %v", err)
	}
}

func TestQuestionRejectsUnsafeOrAmbiguousContent(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name   string
		mutate func(*Question)
	}{
		{"duplicate options", func(item *Question) { item.Options[2] = item.Options[1] }},
		{"wrong language", func(item *Question) { item.Language = "en" }},
		{"future verification", func(item *Question) { item.VerifiedAt = now.Add(48 * time.Hour) }},
		{"insecure source", func(item *Question) { item.Source.URL = "http://example.com" }},
		{"unknown category", func(item *Question) { item.Category = "celebrity-gossip" }},
		{"bad correct index", func(item *Question) { item.CorrectOptionIndex = 4 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := validQuestion(now)
			test.mutate(&item)
			_ = item.SetContentHash()
			if err := item.Validate(now); !errors.Is(err, ErrInvalidQuestion) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func validQuestion(now time.Time) Question {
	return Question{
		ID: "science-water-001", Category: "science", Difficulty: DifficultyEasy,
		Prompt:  "ما الصيغة الكيميائية للماء؟",
		Options: []string{"H₂O", "CO₂", "O₂", "NaCl"}, CorrectOptionIndex: 0,
		Explanation: "يتكون جزيء الماء من ذرتي هيدروجين وذرة أكسجين.",
		Source:      Source{Type: "primary-reference", Title: "NIST Chemistry WebBook", URL: "https://webbook.nist.gov/", License: "Public data"},
		VerifiedAt:  now, Language: "ar", Status: StatusActive,
	}
}
