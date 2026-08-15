package question

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Difficulty string

const (
	DifficultyEasy   Difficulty = "easy"
	DifficultyMedium Difficulty = "medium"
	DifficultyHard   Difficulty = "hard"
)

type Status string

const (
	StatusActive  Status = "active"
	StatusDraft   Status = "draft"
	StatusRetired Status = "retired"
)

var (
	ErrInvalidQuestion = errors.New("invalid question")
	idPattern          = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{5,63}$`)
	allowedCategories  = map[string]struct{}{
		"science": {}, "mathematics": {}, "geography": {}, "cities": {},
		"history": {}, "civics": {}, "religion": {}, "technology": {},
		"general-knowledge": {},
	}
)

type Source struct {
	Type    string `bson:"type" json:"type"`
	Title   string `bson:"title" json:"title"`
	URL     string `bson:"url,omitempty" json:"url,omitempty"`
	License string `bson:"license,omitempty" json:"license,omitempty"`
}

type Question struct {
	ID                 string     `bson:"id" json:"id"`
	Category           string     `bson:"category" json:"category"`
	Difficulty         Difficulty `bson:"difficulty" json:"difficulty"`
	Prompt             string     `bson:"prompt" json:"prompt"`
	Options            []string   `bson:"options" json:"options"`
	CorrectOptionIndex int        `bson:"correctOptionIndex" json:"correctOptionIndex"`
	Explanation        string     `bson:"explanation" json:"explanation"`
	Source             Source     `bson:"source" json:"source"`
	VerifiedAt         time.Time  `bson:"verifiedAt" json:"verifiedAt"`
	Language           string     `bson:"language" json:"language"`
	Status             Status     `bson:"status" json:"status"`
	ContentHash        string     `bson:"contentHash" json:"contentHash"`
}

func (item Question) Validate(now time.Time) error {
	if !idPattern.MatchString(item.ID) {
		return invalid("id")
	}
	if _, allowed := allowedCategories[item.Category]; !allowed {
		return invalid("category")
	}
	if item.Difficulty != DifficultyEasy && item.Difficulty != DifficultyMedium && item.Difficulty != DifficultyHard {
		return invalid("difficulty")
	}
	prompt := strings.TrimSpace(item.Prompt)
	if len([]rune(prompt)) < 8 || len([]rune(prompt)) > 240 {
		return invalid("prompt")
	}
	if len(item.Options) != 4 || item.CorrectOptionIndex < 0 || item.CorrectOptionIndex > 3 {
		return invalid("options")
	}
	seen := make(map[string]struct{}, 4)
	for _, option := range item.Options {
		normalized := strings.ToLower(strings.TrimSpace(option))
		length := len([]rune(normalized))
		if length == 0 || length > 120 {
			return invalid("option text")
		}
		if _, duplicate := seen[normalized]; duplicate {
			return invalid("duplicate options")
		}
		seen[normalized] = struct{}{}
	}
	if length := len([]rune(strings.TrimSpace(item.Explanation))); length < 4 || length > 500 {
		return invalid("explanation")
	}
	if item.Language != "ar" {
		return invalid("language")
	}
	if item.Status != StatusActive && item.Status != StatusDraft && item.Status != StatusRetired {
		return invalid("status")
	}
	if item.VerifiedAt.IsZero() || item.VerifiedAt.After(now.Add(24*time.Hour)) {
		return invalid("verifiedAt")
	}
	if err := item.Source.validate(); err != nil {
		return err
	}
	hash, err := item.CalculateContentHash()
	if err != nil {
		return fmt.Errorf("%w: calculate content hash: %v", ErrInvalidQuestion, err)
	}
	if item.ContentHash == "" || !strings.EqualFold(item.ContentHash, hash) {
		return invalid("contentHash")
	}
	return nil
}

func (source Source) validate() error {
	sourceType := strings.TrimSpace(source.Type)
	if sourceType != "open-data" && sourceType != "primary-reference" && sourceType != "primary-registry" && sourceType != "generated" {
		return invalid("source type")
	}
	if strings.TrimSpace(source.Title) == "" || len([]rune(source.Title)) > 160 {
		return invalid("source title")
	}
	if sourceType == "generated" {
		parsed, err := url.Parse(source.URL)
		if err != nil || parsed.Scheme != "urn" || !strings.HasPrefix(parsed.Opaque, "quizbattle:generator:") {
			return invalid("generated source URL")
		}
		return nil
	}
	parsed, err := url.Parse(source.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return invalid("source URL")
	}
	return nil
}

func (item Question) CalculateContentHash() (string, error) {
	// Maps are deliberate: encoding/json sorts string map keys, producing the
	// same compact canonical representation as the offline bank generator.
	payload := map[string]any{
		"id": item.ID, "category": item.Category, "difficulty": item.Difficulty,
		"prompt": item.Prompt, "options": item.Options, "correctOptionIndex": item.CorrectOptionIndex,
		"explanation": item.Explanation,
		"source": map[string]any{
			"type": item.Source.Type, "title": item.Source.Title,
			"url": item.Source.URL, "license": item.Source.License,
		},
		"verifiedAt": item.VerifiedAt.UTC().Format(time.RFC3339),
		"language":   item.Language,
		"status":     item.Status,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func (item *Question) SetContentHash() error {
	hash, err := item.CalculateContentHash()
	if err != nil {
		return err
	}
	item.ContentHash = hash
	return nil
}

func invalid(field string) error {
	return fmt.Errorf("%w: %s", ErrInvalidQuestion, field)
}
