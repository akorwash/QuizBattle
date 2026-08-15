package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"

	matchdomain "github.com/akorwash/QuizBattle/domain/match"
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

// BotDeckQuestions selects a reproducible, category-diverse virtual deck. The
// cards exist only inside the match aggregate; they are never minted into the
// economy or exposed through the market.
func (service *QuestionBankService) BotDeckQuestions(ctx context.Context, matchID int64, strategy matchdomain.BotStrategy, count int) ([]question.Question, error) {
	if matchID <= 0 || count <= 0 || count > 50 {
		return nil, fmt.Errorf("invalid bot deck selection")
	}
	strategy, err := matchdomain.NormalizeBotStrategy(string(strategy))
	if err != nil {
		return nil, err
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
	sort.Slice(categories, func(left, right int) bool {
		leftRank := stableRank(matchID, "bot-category:"+string(strategy)+":"+categories[left])
		rightRank := stableRank(matchID, "bot-category:"+string(strategy)+":"+categories[right])
		if leftRank == rightRank {
			return categories[left] < categories[right]
		}
		return leftRank < rightRank
	})
	for category := range byCategory {
		bucket := byCategory[category]
		sort.Slice(bucket, func(left, right int) bool {
			if strategy == matchdomain.BotSmart {
				leftDifficulty := botDifficultyRank(bucket[left].Difficulty)
				rightDifficulty := botDifficultyRank(bucket[right].Difficulty)
				if leftDifficulty != rightDifficulty {
					return leftDifficulty > rightDifficulty
				}
			}
			leftRank := stableRank(matchID, "bot-card:"+string(strategy)+":"+bucket[left].ID)
			rightRank := stableRank(matchID, "bot-card:"+string(strategy)+":"+bucket[right].ID)
			if leftRank == rightRank {
				return bucket[left].ID < bucket[right].ID
			}
			return leftRank < rightRank
		})
		byCategory[category] = bucket
	}

	selected := make([]question.Question, 0, count)
	for _, category := range categories {
		if len(selected) >= count {
			break
		}
		if bucket := byCategory[category]; len(bucket) > 0 {
			selected = append(selected, bucket[0])
			byCategory[category] = bucket[1:]
		}
	}
	for len(selected) < count {
		added := false
		for _, category := range categories {
			bucket := byCategory[category]
			if len(bucket) == 0 {
				continue
			}
			selected = append(selected, bucket[0])
			byCategory[category] = bucket[1:]
			added = true
			if len(selected) >= count {
				break
			}
		}
		if !added {
			return nil, fmt.Errorf("question bank could not build a bot deck")
		}
	}
	return selected, nil
}

func botDifficultyRank(value question.Difficulty) int {
	switch value {
	case question.DifficultyHard:
		return 3
	case question.DifficultyMedium:
		return 2
	default:
		return 1
	}
}

func stableRank(userID int64, value string) uint64 {
	digest := sha256.Sum256([]byte(fmt.Sprintf("quizbattle:%d:%s", userID, value)))
	return binary.BigEndian.Uint64(digest[:8])
}
