package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/akorwash/QuizBattle/domain/question"
)

type QuestionBankRepository interface {
	GetByID(ctx context.Context, id string) (*question.Question, error)
	GetByIDs(ctx context.Context, ids []string) (map[string]question.Question, error)
	ListActive(ctx context.Context) ([]question.Question, error)
	CountActive(ctx context.Context) (int64, error)
}

type QuestionBankService struct {
	repository QuestionBankRepository
}

func NewQuestionBankService(repository QuestionBankRepository) *QuestionBankService {
	return &QuestionBankService{repository: repository}
}

func (service *QuestionBankService) Get(ctx context.Context, id string) (*question.Question, error) {
	return service.repository.GetByID(ctx, id)
}

func (service *QuestionBankService) GetMany(ctx context.Context, ids []string) (map[string]question.Question, error) {
	return service.repository.GetByIDs(ctx, ids)
}

func (service *QuestionBankService) Count(ctx context.Context) (int64, error) {
	return service.repository.CountActive(ctx)
}

// TieBreakQuestions returns a deterministic pool of fresh questions for one
// match. Questions already present in committed decks are excluded so a
// revealed regular-round answer can never be reused during sudden death.
func (service *QuestionBankService) TieBreakQuestions(ctx context.Context, matchID int64, excludedIDs []string, count int) ([]question.Question, error) {
	if matchID <= 0 || count <= 0 || count > 50 {
		return nil, fmt.Errorf("invalid tie-break selection")
	}
	items, err := service.repository.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	excluded := make(map[string]struct{}, len(excludedIDs))
	for _, id := range excludedIDs {
		excluded[id] = struct{}{}
	}
	available := make([]question.Question, 0, len(items))
	for _, item := range items {
		if _, found := excluded[item.ID]; !found {
			available = append(available, item)
		}
	}
	if len(available) < count {
		return nil, fmt.Errorf("question bank has %d unused questions; need %d for tie-breaks", len(available), count)
	}
	sort.Slice(available, func(left, right int) bool {
		leftRank := stableRank(matchID, "tie-break:"+available[left].ID)
		rightRank := stableRank(matchID, "tie-break:"+available[right].ID)
		if leftRank == rightRank {
			return available[left].ID < available[right].ID
		}
		return leftRank < rightRank
	})
	return append([]question.Question(nil), available[:count]...), nil
}

// StarterQuestions deterministically selects distinct questions for a user,
// preferring category diversity. Determinism makes starter creation safely
// retryable while the per-user hash gives different collections.
func (service *QuestionBankService) StarterQuestions(ctx context.Context, userID int64, count int) ([]question.Question, error) {
	if userID <= 0 || count <= 0 || count > 50 {
		return nil, fmt.Errorf("invalid starter selection")
	}
	items, err := service.repository.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	if len(items) < count {
		return nil, fmt.Errorf("question bank has %d active questions; need %d", len(items), count)
	}
	byCategory := make(map[string][]question.Question)
	for _, item := range items {
		byCategory[item.Category] = append(byCategory[item.Category], item)
	}
	categories := make([]string, 0, len(byCategory))
	for category := range byCategory {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	if len(categories) < 5 {
		return nil, fmt.Errorf("question bank must contain at least five active categories")
	}

	selected := make([]question.Question, 0, count)
	seen := make(map[string]struct{}, count)
	for _, category := range categories {
		if len(selected) >= count {
			break
		}
		bucket := byCategory[category]
		index := stableRank(userID, category) % uint64(len(bucket))
		selected = append(selected, bucket[index])
		seen[bucket[index].ID] = struct{}{}
	}

	sort.Slice(items, func(left, right int) bool {
		leftRank := stableRank(userID, items[left].ID)
		rightRank := stableRank(userID, items[right].ID)
		if leftRank == rightRank {
			return items[left].ID < items[right].ID
		}
		return leftRank < rightRank
	})
	for _, item := range items {
		if len(selected) >= count {
			break
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}
		selected = append(selected, item)
		seen[item.ID] = struct{}{}
	}
	return selected, nil
}

func stableRank(userID int64, value string) uint64 {
	digest := sha256.Sum256([]byte(fmt.Sprintf("quizbattle:%d:%s", userID, value)))
	return binary.BigEndian.Uint64(digest[:8])
}
