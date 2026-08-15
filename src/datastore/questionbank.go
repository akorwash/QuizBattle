package datastore

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/akorwash/QuizBattle/domain/question"
)

const (
	minimumQuestionBankSize = 1000
	maximumQuestionBankSize = 5000
	maximumQuestionLineSize = 64 << 10
)

// LoadQuestionBank validates the complete JSONL file before any database write.
// A single malformed, duplicated, unsourced, or hash-mismatched item rejects the
// entire import so production never starts with a partially trusted bank.
func LoadQuestionBank(path string, now time.Time) ([]question.Question, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("QUESTION_BANK_PATH is required when seeding")
	}
	// #nosec G304 -- path is startup-only operator configuration, validated as
	// a non-empty QUESTION_BANK_PATH and never derived from an HTTP request.
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open question bank: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, 128<<10)
	items := make([]question.Question, 0, 1600)
	ids := make(map[string]struct{}, 1600)
	prompts := make(map[string]string, 1600)
	lineNumber := 0
	for {
		line, readError := reader.ReadString('\n')
		if len(line) > maximumQuestionLineSize {
			return nil, fmt.Errorf("question bank line %d exceeds %d bytes", lineNumber+1, maximumQuestionLineSize)
		}
		lineNumber++
		line = strings.TrimSpace(line)
		if line != "" {
			var item question.Question
			decoder := json.NewDecoder(strings.NewReader(line))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&item); err != nil {
				return nil, fmt.Errorf("decode question bank line %d: %w", lineNumber, err)
			}
			if decoder.Decode(&struct{}{}) != io.EOF {
				return nil, fmt.Errorf("question bank line %d contains trailing JSON", lineNumber)
			}
			if err := item.Validate(now); err != nil {
				return nil, fmt.Errorf("validate question bank line %d (%s): %w", lineNumber, item.ID, err)
			}
			if _, duplicate := ids[item.ID]; duplicate {
				return nil, fmt.Errorf("duplicate question ID %s", item.ID)
			}
			normalizedPrompt := strings.ToLower(strings.Join(strings.Fields(item.Prompt), " "))
			if existingID, duplicate := prompts[normalizedPrompt]; duplicate {
				return nil, fmt.Errorf("duplicate question prompt in %s and %s", existingID, item.ID)
			}
			ids[item.ID] = struct{}{}
			prompts[normalizedPrompt] = item.ID
			items = append(items, item)
			if len(items) > maximumQuestionBankSize {
				return nil, fmt.Errorf("question bank exceeds %d items", maximumQuestionBankSize)
			}
		}
		if readError != nil {
			if readError == io.EOF {
				break
			}
			return nil, fmt.Errorf("read question bank line %d: %w", lineNumber, readError)
		}
	}
	if len(items) < minimumQuestionBankSize {
		return nil, fmt.Errorf("question bank has %d valid items; need at least %d", len(items), minimumQuestionBankSize)
	}
	return items, nil
}
