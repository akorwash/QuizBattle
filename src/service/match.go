package service

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/akorwash/QuizBattle/domain/economy"
	matchdomain "github.com/akorwash/QuizBattle/domain/match"
	"github.com/akorwash/QuizBattle/domain/question"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
)

const tieBreakQuestionPoolSize = 50

type MatchRepository interface {
	CreateForGame(ctx context.Context, aggregate *matchdomain.Aggregate) error
	GetByGameID(ctx context.Context, gameID int64) (*matchdomain.Aggregate, error)
	Update(ctx context.Context, aggregate *matchdomain.Aggregate, expectedVersion int64) error
	CommitDeck(ctx context.Context, aggregate *matchdomain.Aggregate, expectedVersion, userID int64, newCardIDs, previousCardIDs []int64) error
}

type MatchEconomyRepository interface {
	GetCardsByIDs(ctx context.Context, ids []int64) (map[int64]economy.Card, error)
	SettleMatchRewards(ctx context.Context, matchID int64) error
}

type MatchService struct {
	matches     MatchRepository
	economy     MatchEconomyRepository
	collections *EconomyService
	questions   *QuestionBankService
	games       repository.IGameRepository
	events      GameEventPublisher
	locks       [128]sync.Mutex
}

func NewMatchService(
	matches MatchRepository,
	economy MatchEconomyRepository,
	collections *EconomyService,
	questions *QuestionBankService,
	games repository.IGameRepository,
	events GameEventPublisher,
) *MatchService {
	return &MatchService{matches: matches, economy: economy, collections: collections, questions: questions, games: games, events: events}
}

// Prepare freezes the lobby roster and creates its match draft. Deck commits
// are deliberately unavailable before this boundary, so open lobbies can keep
// accepting members without silently excluding late joiners from the match.
func (service *MatchService) Prepare(ctx context.Context, userID, gameID int64, commandID string) (*matchdomain.Snapshot, error) {
	if err := validateMatchCommandID(commandID); err != nil {
		return nil, err
	}
	unlock := service.lock(gameID)
	defer unlock()
	game, err := service.games.GetGameByID(gameID)
	if err != nil {
		return nil, err
	}
	if game.UserID != userID {
		return nil, matchdomain.ErrNotOwner
	}
	if !game.IsActive {
		return nil, ErrGameClosed
	}
	if aggregate, matchErr := service.matches.GetByGameID(ctx, gameID); matchErr == nil {
		return safeSnapshot(aggregate, userID)
	} else if !errors.Is(matchErr, repository.ErrNotFound) {
		return nil, matchErr
	}
	if game.State != "" && game.State != "lobby" {
		return nil, matchdomain.ErrInvalidState
	}
	mode, minimumPlayers, maximumPlayers, _, err := gameModeDetails(game)
	if err != nil {
		return nil, err
	}
	playerCount := len(game.JoinedUsers)
	if playerCount < minimumPlayers || playerCount > maximumPlayers || (mode != matchdomain.ModeOpen && playerCount != maximumPlayers) {
		return nil, ErrArenaNotReady
	}
	if !containsUser(game.JoinedUsers, userID) {
		return nil, matchdomain.ErrNotPlayer
	}
	matchID, err := repository.NewID()
	if err != nil {
		return nil, err
	}
	aggregate, err := matchdomain.NewArena(matchID, gameID, game.UserID, mode, append([]int64(nil), game.JoinedUsers...), time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if err := service.matches.CreateForGame(ctx, aggregate); err != nil {
		return nil, err
	}
	service.publish("match_created", gameID, aggregate.Version)
	return safeSnapshot(aggregate, userID)
}

func (service *MatchService) CommitDeck(ctx context.Context, userID, gameID int64, cardIDs []int64, commandID string) (*matchdomain.Snapshot, error) {
	unlock := service.lock(gameID)
	defer unlock()
	if len(cardIDs) != matchdomain.DeckSize {
		return nil, matchdomain.ErrInvalidDeck
	}
	if _, err := service.collections.Collection(ctx, userID); err != nil {
		return nil, err
	}
	aggregate, err := service.getPrepared(ctx, userID, gameID)
	if err != nil {
		return nil, err
	}
	previous := playerCardIDs(aggregate, userID)
	cards, err := service.economy.GetCardsByIDs(ctx, cardIDs)
	if err != nil {
		return nil, err
	}
	if len(cards) != len(cardIDs) {
		return nil, economy.ErrCardUnavailable
	}
	questionIDs := make([]string, 0, len(cards))
	for _, cardID := range cardIDs {
		card, exists := cards[cardID]
		if !exists || card.OwnerID != userID || (card.Status != economy.CardAvailable && !(card.Status == economy.CardMatchLocked && card.LockRef == fmt.Sprintf("match:%d", aggregate.ID))) {
			return nil, economy.ErrCardUnavailable
		}
		questionIDs = append(questionIDs, card.QuestionID)
	}
	questions, err := service.questions.GetMany(ctx, questionIDs)
	if err != nil {
		return nil, err
	}
	snapshots := make([]matchdomain.CardSnapshot, 0, len(cardIDs))
	for _, cardID := range cardIDs {
		card := cards[cardID]
		item, exists := questions[card.QuestionID]
		if !exists {
			return nil, repository.ErrNotFound
		}
		snapshots = append(snapshots, matchCardSnapshot(card, item))
	}
	expectedVersion := aggregate.Version
	changed, err := aggregate.CommitDeck(userID, snapshots, commandID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if changed {
		if err := service.matches.CommitDeck(ctx, aggregate, expectedVersion, userID, cardIDs, previous); err != nil {
			return nil, err
		}
		service.publish("deck_committed", gameID, aggregate.Version)
	}
	return safeSnapshot(aggregate, userID)
}

func (service *MatchService) Start(ctx context.Context, userID, gameID int64, commandID string) (*matchdomain.Snapshot, error) {
	if err := validateMatchCommandID(commandID); err != nil {
		return nil, err
	}
	unlock := service.lock(gameID)
	defer unlock()
	aggregate, err := service.matches.GetByGameID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	expectedVersion := aggregate.Version
	var tieBreakQuestions []matchdomain.QuestionSnapshot
	if aggregate.Status == matchdomain.StatusCollectingDecks {
		if aggregate.OwnerID != userID {
			return nil, matchdomain.ErrNotOwner
		}
		if !allPlayersReady(aggregate) {
			return nil, matchdomain.ErrDecksNotReady
		}
		if service.questions == nil {
			return nil, fmt.Errorf("question bank service is required to start a match")
		}
		excluded := make([]string, 0, len(aggregate.Players)*matchdomain.DeckSize)
		for _, player := range aggregate.Players {
			for _, card := range player.Deck {
				excluded = append(excluded, card.Question.ID)
			}
		}
		items, questionErr := service.questions.TieBreakQuestions(ctx, aggregate.ID, excluded, tieBreakQuestionPoolSize)
		if questionErr != nil {
			return nil, questionErr
		}
		tieBreakQuestions = make([]matchdomain.QuestionSnapshot, 0, len(items))
		for _, item := range items {
			tieBreakQuestions = append(tieBreakQuestions, matchQuestionSnapshot(item))
		}
	}
	changed, err := aggregate.StartWithTieBreak(userID, tieBreakQuestions, commandID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if changed {
		if err := service.matches.Update(ctx, aggregate, expectedVersion); err != nil {
			return nil, err
		}
		service.publish("match_started", gameID, aggregate.Version)
	}
	return safeSnapshot(aggregate, userID)
}

func allPlayersReady(aggregate *matchdomain.Aggregate) bool {
	if aggregate == nil || len(aggregate.Players) < 2 {
		return false
	}
	for _, player := range aggregate.Players {
		if len(player.Deck) != matchdomain.DeckSize {
			return false
		}
	}
	return true
}

func (service *MatchService) Snapshot(ctx context.Context, userID, gameID int64) (*matchdomain.Snapshot, error) {
	unlock := service.lock(gameID)
	defer unlock()
	aggregate, err := service.matches.GetByGameID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	// Authorization must precede Tick because Tick persists state. An outsider
	// who guesses a game ID must never be able to trigger match writes.
	if !matchContains(aggregate, userID) {
		return nil, matchdomain.ErrNotPlayer
	}
	expectedVersion := aggregate.Version
	changed := aggregate.Tick(time.Now().UTC())
	if aggregate.Status == matchdomain.StatusTieBreak && aggregate.TieBreak.AwaitingQuestion {
		added, addErr := service.replenishTieBreak(ctx, aggregate)
		if addErr != nil {
			return nil, addErr
		}
		changed = changed || added
	}
	if changed {
		if err := service.matches.Update(ctx, aggregate, expectedVersion); err != nil {
			return nil, err
		}
		service.publish(matchTickEvent(aggregate), gameID, aggregate.Version)
	}
	if (aggregate.Status == matchdomain.StatusCompleted || aggregate.Status == matchdomain.StatusForfeited) && !aggregate.RewardsSettled {
		if err := service.economy.SettleMatchRewards(ctx, aggregate.ID); err != nil {
			return nil, err
		}
		aggregate.RewardsSettled = true
	}
	return safeSnapshot(aggregate, userID)
}

func (service *MatchService) replenishTieBreak(ctx context.Context, aggregate *matchdomain.Aggregate) (bool, error) {
	if aggregate == nil || aggregate.Status != matchdomain.StatusTieBreak || !aggregate.TieBreak.AwaitingQuestion {
		return false, nil
	}
	if service.questions == nil {
		return false, fmt.Errorf("question bank service is required to continue a tie-break")
	}
	excluded := make([]string, 0, len(aggregate.Players)*matchdomain.DeckSize+len(aggregate.TieBreak.QuestionPool))
	for _, player := range aggregate.Players {
		for _, card := range player.Deck {
			excluded = append(excluded, card.Question.ID)
		}
	}
	for _, item := range aggregate.TieBreak.QuestionPool {
		excluded = append(excluded, item.ID)
	}
	items, err := service.questions.TieBreakQuestions(ctx, aggregate.ID, excluded, tieBreakQuestionPoolSize)
	if err != nil {
		return false, err
	}
	questions := make([]matchdomain.QuestionSnapshot, 0, len(items))
	for _, item := range items {
		questions = append(questions, matchQuestionSnapshot(item))
	}
	return aggregate.AddTieBreakQuestions(questions, time.Now().UTC())
}

func (service *MatchService) Forfeit(ctx context.Context, userID, gameID int64, commandID string) (*matchdomain.Snapshot, error) {
	unlock := service.lock(gameID)
	defer unlock()
	aggregate, err := service.matches.GetByGameID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if !matchContains(aggregate, userID) {
		return nil, matchdomain.ErrNotPlayer
	}
	expectedVersion := aggregate.Version
	changed, err := aggregate.Forfeit(userID, commandID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if changed {
		if err := service.matches.Update(ctx, aggregate, expectedVersion); err != nil {
			return nil, err
		}
		service.publish("match_forfeited", gameID, aggregate.Version)
	}
	if !aggregate.RewardsSettled {
		if err := service.economy.SettleMatchRewards(ctx, aggregate.ID); err != nil {
			return nil, err
		}
		aggregate.RewardsSettled = true
	}
	return safeSnapshot(aggregate, userID)
}

func (service *MatchService) Answer(ctx context.Context, userID, gameID int64, turnID string, option int, commandID string) (*matchdomain.Snapshot, error) {
	unlock := service.lock(gameID)
	defer unlock()
	aggregate, err := service.matches.GetByGameID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	expectedVersion := aggregate.Version
	changed, err := aggregate.SubmitAnswer(userID, turnID, option, commandID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if changed {
		if err := service.matches.Update(ctx, aggregate, expectedVersion); err != nil {
			return nil, err
		}
		eventType := "answer_received"
		if aggregate.CurrentTurn >= 0 && aggregate.CurrentTurn < len(aggregate.Turns) && aggregate.Turns[aggregate.CurrentTurn].Status == matchdomain.TurnResolved {
			eventType = "turn_resolved"
		}
		service.publish(eventType, gameID, aggregate.Version)
	}
	return safeSnapshot(aggregate, userID)
}

func (service *MatchService) getPrepared(ctx context.Context, userID, gameID int64) (*matchdomain.Aggregate, error) {
	aggregate, err := service.matches.GetByGameID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if !matchContains(aggregate, userID) {
		return nil, matchdomain.ErrNotPlayer
	}
	return aggregate, nil
}

func (service *MatchService) publish(eventType string, gameID, version int64) {
	if service.events != nil {
		service.events.PublishGameEvent(resources.GameEvent{Type: eventType, GameID: gameID, MatchVersion: version})
	}
}

func (service *MatchService) lock(gameID int64) func() {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(fmt.Sprint(gameID)))
	mutex := &service.locks[hash.Sum32()%uint32(len(service.locks))]
	mutex.Lock()
	return mutex.Unlock
}

func matchCardSnapshot(card economy.Card, item question.Question) matchdomain.CardSnapshot {
	return matchdomain.CardSnapshot{
		ID: card.ID, OwnerID: card.OwnerID, Rarity: card.Rarity, Power: card.Power,
		Question: matchQuestionSnapshot(item),
	}
}

func matchQuestionSnapshot(item question.Question) matchdomain.QuestionSnapshot {
	return matchdomain.QuestionSnapshot{
		ID: item.ID, Prompt: item.Prompt, Options: append([]string(nil), item.Options...),
		CorrectOption: item.CorrectOptionIndex, Explanation: item.Explanation,
		Category: item.Category, Difficulty: string(item.Difficulty), ContentHash: item.ContentHash,
	}
}

func validateMatchCommandID(commandID string) error {
	commandID = strings.TrimSpace(commandID)
	if len(commandID) < 8 || len(commandID) > 128 {
		return matchdomain.ErrInvalidCommandID
	}
	for _, character := range commandID {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' || character == ':' {
			continue
		}
		return matchdomain.ErrInvalidCommandID
	}
	return nil
}

func playerCardIDs(aggregate *matchdomain.Aggregate, userID int64) []int64 {
	for _, player := range aggregate.Players {
		if player.UserID != userID {
			continue
		}
		result := make([]int64, 0, len(player.Deck))
		for _, card := range player.Deck {
			result = append(result, card.ID)
		}
		return result
	}
	return nil
}

func safeSnapshot(aggregate *matchdomain.Aggregate, userID int64) (*matchdomain.Snapshot, error) {
	snapshot, err := aggregate.SafeSnapshot(userID)
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func matchContains(aggregate *matchdomain.Aggregate, userID int64) bool {
	for _, player := range aggregate.Players {
		if player.UserID == userID {
			return true
		}
	}
	return false
}

func matchTickEvent(aggregate *matchdomain.Aggregate) string {
	if aggregate.Status == matchdomain.StatusCompleted {
		return "match_completed"
	}
	if aggregate.Status == matchdomain.StatusTieBreak {
		return "tiebreak_started"
	}
	if aggregate.CurrentTurn >= 0 && aggregate.CurrentTurn < len(aggregate.Turns) && aggregate.Turns[aggregate.CurrentTurn].Status == matchdomain.TurnResolved {
		return "turn_resolved"
	}
	return "turn_started"
}
