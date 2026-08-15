package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/domain/economy"
	matchdomain "github.com/akorwash/QuizBattle/domain/match"
	"github.com/akorwash/QuizBattle/domain/question"
	"github.com/akorwash/QuizBattle/repository"
)

func TestMatchPrepareIsOwnerOnly(t *testing.T) {
	fixture := newMatchMultiplayerFixture(matchMultiplayerGame("duel", 2, 2))

	if _, err := fixture.service.Prepare(context.Background(), 2, fixture.gameID, "guest-prepare-001"); !errors.Is(err, matchdomain.ErrNotOwner) {
		t.Fatalf("non-owner prepare returned %v", err)
	}
	if fixture.matches.createCalls != 0 {
		t.Fatal("non-owner prepare created a match")
	}

	prepared, err := fixture.service.Prepare(context.Background(), 1, fixture.gameID, "owner-prepare-001")
	if err != nil {
		t.Fatal(err)
	}
	if prepared.OwnerID != 1 || prepared.Mode != matchdomain.ModeDuel || len(prepared.Players) != 2 || fixture.matches.createCalls != 1 {
		t.Fatalf("unexpected prepared match: snapshot=%+v creates=%d", prepared, fixture.matches.createCalls)
	}
}

func TestMatchPrepareFixedModesRequireFullRoster(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		capacity int
	}{
		{"duel", "duel", 2},
		{"two versus two", "team_2v2", 4},
		{"four versus four", "team_4v4", 8},
	}

	for _, test := range tests {
		t.Run(test.name+" incomplete", func(t *testing.T) {
			fixture := newMatchMultiplayerFixture(matchMultiplayerGame(test.mode, test.capacity, test.capacity-1))
			if _, err := fixture.service.Prepare(context.Background(), 1, fixture.gameID, "prepare-incomplete-001"); !errors.Is(err, ErrArenaNotReady) {
				t.Fatalf("incomplete fixed roster returned %v", err)
			}
			if fixture.matches.createCalls != 0 {
				t.Fatal("incomplete fixed roster created a match")
			}
		})

		t.Run(test.name+" full", func(t *testing.T) {
			fixture := newMatchMultiplayerFixture(matchMultiplayerGame(test.mode, test.capacity, test.capacity))
			prepared, err := fixture.service.Prepare(context.Background(), 1, fixture.gameID, "prepare-full-0001")
			if err != nil {
				t.Fatal(err)
			}
			if len(prepared.Players) != test.capacity || string(prepared.Mode) != test.mode || fixture.matches.createCalls != 1 {
				t.Fatalf("full fixed roster was not frozen: snapshot=%+v creates=%d", prepared, fixture.matches.createCalls)
			}
		})
	}
}

func TestMatchPrepareOpenAcceptsTwoThroughEightPlayers(t *testing.T) {
	for playerCount := 1; playerCount <= 9; playerCount++ {
		playerCount := playerCount
		t.Run(fmt.Sprintf("players_%d", playerCount), func(t *testing.T) {
			fixture := newMatchMultiplayerFixture(matchMultiplayerGame("open", 8, playerCount))
			prepared, err := fixture.service.Prepare(context.Background(), 1, fixture.gameID, "prepare-open-0001")
			valid := playerCount >= 2 && playerCount <= 8
			if !valid {
				if !errors.Is(err, ErrArenaNotReady) || fixture.matches.createCalls != 0 {
					t.Fatalf("invalid open roster: snapshot=%+v err=%v creates=%d", prepared, err, fixture.matches.createCalls)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if prepared.Mode != matchdomain.ModeOpen || len(prepared.Players) != playerCount || fixture.matches.createCalls != 1 {
				t.Fatalf("valid open roster was not frozen: snapshot=%+v creates=%d", prepared, fixture.matches.createCalls)
			}
		})
	}
}

func TestMatchPrepareIsIdempotent(t *testing.T) {
	fixture := newMatchMultiplayerFixture(matchMultiplayerGame("team_2v2", 4, 4))
	first, err := fixture.service.Prepare(context.Background(), 1, fixture.gameID, "prepare-idempotent-001")
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.service.Prepare(context.Background(), 1, fixture.gameID, "prepare-idempotent-001")
	if err != nil {
		t.Fatal(err)
	}

	if first.ID != second.ID || first.Version != second.Version || fixture.matches.createCalls != 1 {
		t.Fatalf("prepare retry was not idempotent: first=%+v second=%+v creates=%d", first, second, fixture.matches.createCalls)
	}
	if len(fixture.events.events) != 1 || fixture.events.events[0].Type != "match_created" {
		t.Fatalf("prepare retry published duplicate events: %+v", fixture.events.events)
	}
}

func TestMatchCommitDeckBeforePrepareIsRejected(t *testing.T) {
	fixture := newMatchMultiplayerFixture(matchMultiplayerGame("open", 8, 2))
	_, err := fixture.service.CommitDeck(
		context.Background(), 1, fixture.gameID, matchMultiplayerDeckIDs(1), "deck-before-prepare-001",
	)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("commit before prepare returned %v", err)
	}
	if fixture.matches.commitCalls != 0 || fixture.matches.aggregate != nil {
		t.Fatalf("commit before prepare mutated match storage: commits=%d aggregate=%+v", fixture.matches.commitCalls, fixture.matches.aggregate)
	}
}

func TestMatchOwnerStartWaitsForEveryFiveCardDeck(t *testing.T) {
	fixture := newMatchMultiplayerFixture(matchMultiplayerGame("open", 8, 3))
	if _, err := fixture.service.Prepare(context.Background(), 1, fixture.gameID, "prepare-open-start-001"); err != nil {
		t.Fatal(err)
	}
	fixture.commitDeck(t, 1)
	fixture.commitDeck(t, 2)

	if _, err := fixture.service.Start(context.Background(), 2, fixture.gameID, "guest-start-0001"); !errors.Is(err, matchdomain.ErrNotOwner) {
		t.Fatalf("non-owner start returned %v", err)
	}
	if _, err := fixture.service.Start(context.Background(), 1, fixture.gameID, "owner-start-blocked-001"); !errors.Is(err, matchdomain.ErrDecksNotReady) {
		t.Fatalf("owner start with one missing deck returned %v", err)
	}
	if fixture.matches.updateCalls != 0 {
		t.Fatal("blocked start persisted a match update")
	}

	beforeReady, err := fixture.service.Snapshot(context.Background(), 1, fixture.gameID)
	if err != nil {
		t.Fatal(err)
	}
	if beforeReady.CanStart || len(beforeReady.StartBlockers) == 0 {
		t.Fatalf("snapshot did not expose the missing deck blocker: %+v", beforeReady)
	}

	fixture.commitDeck(t, 3)
	ready, err := fixture.service.Snapshot(context.Background(), 1, fixture.gameID)
	if err != nil {
		t.Fatal(err)
	}
	if !ready.CanStart || len(ready.StartBlockers) != 0 {
		t.Fatalf("all committed decks did not make the owner ready to start: %+v", ready)
	}

	started, err := fixture.service.Start(context.Background(), 1, fixture.gameID, "owner-start-ready-001")
	if err != nil {
		t.Fatal(err)
	}
	if started.Status != matchdomain.StatusActive || started.TotalTurns != matchdomain.DeckSize*3 || !started.TieBreak.Enabled || started.TieBreak.RemainingQuestions != tieBreakQuestionPoolSize {
		t.Fatalf("unexpected started multiplayer snapshot: %+v", started)
	}
	if fixture.matches.updateCalls != 1 {
		t.Fatalf("successful start update count=%d", fixture.matches.updateCalls)
	}
}

func TestMatchSnapshotsExposeTeamsAndReadinessWithoutOtherDecks(t *testing.T) {
	fixture := newMatchMultiplayerFixture(matchMultiplayerGame("team_2v2", 4, 4))
	if _, err := fixture.service.Prepare(context.Background(), 1, fixture.gameID, "prepare-team-snapshot-001"); err != nil {
		t.Fatal(err)
	}
	fixture.commitDeck(t, 1)
	fixture.commitDeck(t, 2)

	snapshot, err := fixture.service.Snapshot(context.Background(), 1, fixture.gameID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Mode != matchdomain.ModeTeam2v2 || len(snapshot.Players) != 4 {
		t.Fatalf("unexpected team snapshot: %+v", snapshot)
	}
	wantTeams := []int{1, 2, 1, 2}
	wantReady := []bool{true, true, false, false}
	for index, player := range snapshot.Players {
		if player.UserID != int64(index+1) || player.Team != wantTeams[index] || player.DeckReady != wantReady[index] {
			t.Fatalf("player %d: got %+v want team=%d ready=%v", index, player, wantTeams[index], wantReady[index])
		}
		wantDeckIDs := 0
		if player.UserID == 1 {
			wantDeckIDs = matchdomain.DeckSize
		}
		if len(player.DeckCardIDs) != wantDeckIDs {
			t.Fatalf("viewer deck privacy failed for user %d: %+v", player.UserID, player.DeckCardIDs)
		}
	}
	if len(snapshot.TeamScores) != 2 || snapshot.TeamScores[0].Team != 1 || snapshot.TeamScores[0].Score != 0 ||
		snapshot.TeamScores[1].Team != 2 || snapshot.TeamScores[1].Score != 0 {
		t.Fatalf("unexpected initial team scores: %+v", snapshot.TeamScores)
	}
	if snapshot.CanStart || len(snapshot.StartBlockers) != 2 {
		t.Fatalf("readiness blockers did not match the two missing decks: %+v", snapshot.StartBlockers)
	}
}

func TestMatchSnapshotReplenishesExhaustedTieBreakPool(t *testing.T) {
	now := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)
	aggregate, err := matchdomain.NewArena(7001, 7002, 1, matchdomain.ModeDuel, []int64{1, 2}, now)
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Status = matchdomain.StatusTieBreak
	aggregate.CurrentTurn = -1
	aggregate.TieBreak = matchdomain.TieBreakState{
		Enabled: true, Active: true, Phase: matchdomain.TieBreakChampion,
		ContenderIDs: []int64{1, 2}, AwaitingQuestion: true,
	}

	questionRepository := newMatchMultiplayerQuestionRepository()
	for index := 0; index < tieBreakQuestionPoolSize; index++ {
		questionRepository.add(matchMultiplayerQuestion(fmt.Sprintf("tie-replenish-%03d", index)))
	}
	matches := &matchMultiplayerMatchRepository{aggregate: aggregate}
	events := &eventRecorder{}
	matchService := NewMatchService(
		matches, nil, nil, NewQuestionBankService(questionRepository), nil, events,
	)

	snapshot, err := matchService.Snapshot(context.Background(), 1, aggregate.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if matches.updateCalls != 1 || len(matches.expectedVersions) != 1 || matches.expectedVersions[0] != 1 {
		t.Fatalf("replenishment persistence was not versioned once: updates=%d versions=%v", matches.updateCalls, matches.expectedVersions)
	}
	if questionRepository.listCalls != 1 || aggregate.TieBreak.AwaitingQuestion || len(aggregate.TieBreak.QuestionPool) != tieBreakQuestionPoolSize {
		t.Fatalf("tie-break pool was not replenished: listCalls=%d state=%+v", questionRepository.listCalls, aggregate.TieBreak)
	}
	if snapshot.Status != matchdomain.StatusTieBreak || snapshot.CurrentTurn == nil || snapshot.CurrentTurn.Kind != matchdomain.TurnTieBreak ||
		snapshot.TieBreak.Round != 1 || snapshot.TieBreak.AwaitingQuestion || snapshot.TieBreak.RemainingQuestions != tieBreakQuestionPoolSize-1 {
		t.Fatalf("unexpected replenished tie-break snapshot: %+v", snapshot)
	}
	if len(events.events) != 1 || events.events[0].Type != "tiebreak_started" {
		t.Fatalf("tie-break replenishment event missing: %+v", events.events)
	}
}

type matchMultiplayerFixture struct {
	service   *MatchService
	matches   *matchMultiplayerMatchRepository
	economy   *matchMultiplayerEconomyRepository
	questions *matchMultiplayerQuestionRepository
	events    *eventRecorder
	gameID    int64
}

func newMatchMultiplayerFixture(game entites.Game) *matchMultiplayerFixture {
	if game.ID <= 0 {
		game.ID = 5001
	}
	questions := newMatchMultiplayerQuestionRepository()
	economyRepository := &matchMultiplayerEconomyRepository{cards: make(map[int64]economy.Card)}
	for _, userID := range game.JoinedUsers {
		for cardIndex, cardID := range matchMultiplayerDeckIDs(userID) {
			questionID := fmt.Sprintf("deck-%d-%02d", userID, cardIndex)
			questions.add(matchMultiplayerQuestion(questionID))
			economyRepository.cards[cardID] = economy.Card{
				ID: cardID, OwnerID: userID, QuestionID: questionID,
				Status: economy.CardAvailable, Rarity: "common", Power: 1, Version: 1,
			}
		}
	}
	for index := 0; index < tieBreakQuestionPoolSize; index++ {
		questions.add(matchMultiplayerQuestion(fmt.Sprintf("tie-question-%03d", index)))
	}

	questionService := NewQuestionBankService(questions)
	matches := &matchMultiplayerMatchRepository{}
	events := &eventRecorder{}
	gameRepository := &fakeGameRepository{games: map[int64]entites.Game{game.ID: game}}
	collectionService := NewEconomyService(economyRepository, questionService)
	return &matchMultiplayerFixture{
		service: NewMatchService(matches, economyRepository, collectionService, questionService, gameRepository, events),
		matches: matches, economy: economyRepository, questions: questions, events: events, gameID: game.ID,
	}
}

func (fixture *matchMultiplayerFixture) commitDeck(t *testing.T, userID int64) {
	t.Helper()
	if _, err := fixture.service.CommitDeck(
		context.Background(), userID, fixture.gameID, matchMultiplayerDeckIDs(userID), fmt.Sprintf("deck-user-%d-0001", userID),
	); err != nil {
		t.Fatalf("commit user %d deck: %v", userID, err)
	}
}

func matchMultiplayerGame(mode string, maximumPlayers, playerCount int) entites.Game {
	joinedUsers := make([]int64, playerCount)
	for index := range joinedUsers {
		joinedUsers[index] = int64(index + 1)
	}
	return entites.Game{
		ID: 5001, UserID: 1, IsPublic: true, IsActive: true,
		Mode: mode, MaxPlayers: maximumPlayers, JoinedUsers: joinedUsers, State: "lobby",
	}
}

func matchMultiplayerDeckIDs(userID int64) []int64 {
	result := make([]int64, matchdomain.DeckSize)
	for index := range result {
		result[index] = userID*1000 + int64(index+1)
	}
	return result
}

type matchMultiplayerMatchRepository struct {
	aggregate        *matchdomain.Aggregate
	createCalls      int
	updateCalls      int
	commitCalls      int
	expectedVersions []int64
}

func (store *matchMultiplayerMatchRepository) CreateForGame(_ context.Context, aggregate *matchdomain.Aggregate) error {
	if store.aggregate != nil {
		return repository.ErrConflict
	}
	store.aggregate = aggregate
	store.createCalls++
	return nil
}

func (store *matchMultiplayerMatchRepository) GetByGameID(_ context.Context, gameID int64) (*matchdomain.Aggregate, error) {
	if store.aggregate == nil || store.aggregate.GameID != gameID {
		return nil, repository.ErrNotFound
	}
	return store.aggregate, nil
}

func (store *matchMultiplayerMatchRepository) Update(_ context.Context, aggregate *matchdomain.Aggregate, expectedVersion int64) error {
	if aggregate == nil || aggregate.Version <= expectedVersion {
		return repository.ErrConflict
	}
	store.aggregate = aggregate
	store.updateCalls++
	store.expectedVersions = append(store.expectedVersions, expectedVersion)
	return nil
}

func (store *matchMultiplayerMatchRepository) CommitDeck(
	_ context.Context,
	aggregate *matchdomain.Aggregate,
	expectedVersion, _ int64,
	_, _ []int64,
) error {
	if aggregate == nil || aggregate.Version <= expectedVersion {
		return repository.ErrConflict
	}
	store.aggregate = aggregate
	store.commitCalls++
	store.expectedVersions = append(store.expectedVersions, expectedVersion)
	return nil
}

type matchMultiplayerEconomyRepository struct {
	economyBoundaryRepository
	cards       map[int64]economy.Card
	settlements int
}

func (store *matchMultiplayerEconomyRepository) GetWallet(_ context.Context, userID int64) (*economy.Wallet, error) {
	return &economy.Wallet{UserID: userID, Balance: economy.StarterBalance, Version: 1}, nil
}

func (store *matchMultiplayerEconomyRepository) ListCards(_ context.Context, userID int64) ([]economy.Card, error) {
	result := make([]economy.Card, 0, matchdomain.DeckSize)
	for _, card := range store.cards {
		if card.OwnerID == userID {
			result = append(result, card)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result, nil
}

func (store *matchMultiplayerEconomyRepository) GetCardsByIDs(_ context.Context, ids []int64) (map[int64]economy.Card, error) {
	result := make(map[int64]economy.Card, len(ids))
	for _, id := range ids {
		if card, exists := store.cards[id]; exists {
			result[id] = card
		}
	}
	return result, nil
}

func (store *matchMultiplayerEconomyRepository) SettleMatchRewards(context.Context, int64) error {
	store.settlements++
	return nil
}

type matchMultiplayerQuestionRepository struct {
	items     []question.Question
	byID      map[string]question.Question
	listCalls int
}

func newMatchMultiplayerQuestionRepository() *matchMultiplayerQuestionRepository {
	return &matchMultiplayerQuestionRepository{byID: make(map[string]question.Question)}
}

func (store *matchMultiplayerQuestionRepository) add(item question.Question) {
	store.items = append(store.items, item)
	store.byID[item.ID] = item
}

func (store *matchMultiplayerQuestionRepository) GetByID(_ context.Context, id string) (*question.Question, error) {
	item, exists := store.byID[id]
	if !exists {
		return nil, repository.ErrNotFound
	}
	copy := item
	return &copy, nil
}

func (store *matchMultiplayerQuestionRepository) GetByIDs(_ context.Context, ids []string) (map[string]question.Question, error) {
	result := make(map[string]question.Question, len(ids))
	for _, id := range ids {
		if item, exists := store.byID[id]; exists {
			result[id] = item
		}
	}
	return result, nil
}

func (store *matchMultiplayerQuestionRepository) ListActive(context.Context) ([]question.Question, error) {
	store.listCalls++
	return append([]question.Question(nil), store.items...), nil
}

func (store *matchMultiplayerQuestionRepository) CountActive(context.Context) (int64, error) {
	return int64(len(store.items)), nil
}

func matchMultiplayerQuestion(id string) question.Question {
	return question.Question{
		ID: id, Category: "science", Difficulty: question.DifficultyEasy,
		Prompt:             "ما الإجابة الصحيحة في هذا السؤال الاختباري؟",
		Options:            []string{"الإجابة الأولى", "الإجابة الثانية", "الإجابة الثالثة", "الإجابة الرابعة"},
		CorrectOptionIndex: 0, Explanation: "شرح الإجابة الصحيحة", Status: question.StatusActive,
		ContentHash: "test-content-hash",
	}
}
