package match

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBotModeAndConstructorInvariants(t *testing.T) {
	now := time.Date(2026, time.August, 15, 18, 0, 0, 0, time.UTC)
	seed := bytes.Repeat([]byte{0x2a}, BotSeedSize)
	aggregate, err := NewBotDuel(101, 202, 11, BotSmart, seed, now)
	if err != nil {
		t.Fatal(err)
	}
	seed[0] = 0
	if aggregate.Mode != ModeBot || len(aggregate.Players) != 2 || aggregate.Players[0].EffectiveKind() != PlayerHuman ||
		!aggregate.Players[1].IsBot() || aggregate.Players[1].UserID != BotActorID || aggregate.Players[1].Bot == nil ||
		aggregate.Players[1].Bot.Seed[0] != 0x2a || !aggregate.validRoster() {
		t.Fatalf("unexpected bot duel: %+v", aggregate)
	}
	if normalized, normalizeErr := NormalizeMode(" BOT "); normalizeErr != nil || normalized != ModeBot {
		t.Fatalf("normalize bot mode: mode=%q err=%v", normalized, normalizeErr)
	}
	if MinPlayers(ModeBot) != 2 || MaxPlayers(ModeBot) != 2 || TeamSize(ModeBot) != 1 {
		t.Fatal("bot mode policy is not a two-participant duel")
	}
	if _, err := NewArena(102, 202, 11, ModeBot, []int64{11, 22}, now); !errors.Is(err, ErrInvalidMatch) {
		t.Fatalf("generic arena constructor created a fake-user bot: %v", err)
	}
	if _, err := NewBotDuel(101, 202, 11, "scripted", bytes.Repeat([]byte{1}, BotSeedSize), now); !errors.Is(err, ErrInvalidBotStrategy) {
		t.Fatalf("invalid strategy accepted: %v", err)
	}
	if _, err := NewBotDuel(101, 202, 11, BotRandom, []byte("short"), now); !errors.Is(err, ErrInvalidBotSeed) {
		t.Fatalf("invalid seed accepted: %v", err)
	}

	legacy := Player{UserID: 77}
	if legacy.IsBot() || legacy.EffectiveKind() != PlayerHuman {
		t.Fatal("legacy empty player kind was not treated as human")
	}
}

func TestBotDeckAndSnapshotAreSeparatedFromHumanCommands(t *testing.T) {
	now := time.Date(2026, time.August, 15, 18, 30, 0, 0, time.UTC)
	aggregate := mustBotMatch(t, BotSmart, now)
	if _, err := aggregate.CommitDeck(11, deck(11, 100), "human-deck-001", now); err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.CommitDeck(BotActorID, botDeck(), "spoof-bot-deck-001", now); !errors.Is(err, ErrNotPlayer) {
		t.Fatalf("human deck command acted as the bot: %v", err)
	}
	changed, err := aggregate.CommitBotDeck(botDeck(), "system-bot-deck-001", now)
	if err != nil || !changed {
		t.Fatalf("commit bot deck: changed=%v err=%v", changed, err)
	}
	if _, err := aggregate.SubmitAnswer(BotActorID, "not-started", 0, "spoof-bot-answer-1", now); !errors.Is(err, ErrNotPlayer) {
		t.Fatalf("human answer command acted as the bot: %v", err)
	}

	snapshot, err := aggregate.SafeSnapshot(11)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Players) != 2 || !snapshot.Players[1].IsBot || snapshot.Players[1].BotStrategy != BotSmart ||
		!snapshot.Players[1].DeckReady || len(snapshot.Players[1].DeckCardIDs) != 0 {
		t.Fatalf("unsafe or incomplete bot snapshot: %+v", snapshot.Players)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "KioqKioq") || strings.Contains(string(encoded), "decisionVersion") {
		t.Fatalf("bot decision material leaked: %s", encoded)
	}
	if _, err := aggregate.SafeSnapshot(BotActorID); !errors.Is(err, ErrNotPlayer) {
		t.Fatalf("bot was allowed to authenticate as a snapshot viewer: %v", err)
	}
}

func TestPlanBotDecisionIsDeterministicBoundedAndVaried(t *testing.T) {
	now := time.Date(2026, time.August, 15, 19, 0, 0, 0, time.UTC)
	aggregate := mustStartedBotMatch(t, BotRandom, now, false)
	turn := &aggregate.Turns[aggregate.CurrentTurn]
	bot := aggregate.botPlayer()
	first, err := PlanBotDecision(aggregate, turn, bot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PlanBotDecision(aggregate, turn, bot)
	if err != nil || first != second {
		t.Fatalf("decision changed across retry: first=%+v second=%+v err=%v", first, second, err)
	}
	if first.Option < 0 || first.Option > 3 || first.DueAt.Before(turn.StartedAt.Add(2*time.Second)) || !first.DueAt.Before(turn.Deadline) ||
		len(first.CommandID) < minimumCommandIDLength || len(first.CommandID) > maximumCommandIDLength {
		t.Fatalf("decision outside contract: %+v turn=%+v", first, turn)
	}

	counts := [4]int{}
	for index := 0; index < 400; index++ {
		candidate := cloneBotTurn(*turn, index, now)
		decision, decisionErr := PlanBotDecision(aggregate, &candidate, bot)
		if decisionErr != nil {
			t.Fatal(decisionErr)
		}
		counts[decision.Option]++
	}
	for option, count := range counts {
		if count < 60 {
			t.Fatalf("random bot did not vary option %d enough: counts=%v", option, counts)
		}
	}
}

func TestSmartBotAccuracyProfileStillProducesSafeMistakes(t *testing.T) {
	now := time.Date(2026, time.August, 15, 19, 30, 0, 0, time.UTC)
	aggregate := mustStartedBotMatch(t, BotSmart, now, false)
	base := aggregate.Turns[aggregate.CurrentTurn]
	bot := aggregate.botPlayer()
	correct := 0
	incorrect := 0
	for index := 0; index < 500; index++ {
		turn := cloneBotTurn(base, index, now)
		turn.Card.Question.Difficulty = "easy"
		decision, err := PlanBotDecision(aggregate, &turn, bot)
		if err != nil {
			t.Fatal(err)
		}
		if decision.Option == turn.Card.Question.CorrectOption {
			correct++
		} else {
			incorrect++
			if decision.Option < 0 || decision.Option > 3 {
				t.Fatalf("invalid wrong option: %+v", decision)
			}
		}
	}
	if correct < 390 || correct > 460 || incorrect == 0 {
		t.Fatalf("smart bot accuracy escaped its deterministic profile: correct=%d incorrect=%d", correct, incorrect)
	}
}

func TestAdvanceBotsAppliesDueAnswerBeforeLateTimeout(t *testing.T) {
	now := time.Date(2026, time.August, 15, 20, 0, 0, 0, time.UTC)
	aggregate := mustStartedBotMatch(t, BotSmart, now, false)
	turn := &aggregate.Turns[aggregate.CurrentTurn]
	decision, err := PlanBotDecision(aggregate, turn, aggregate.botPlayer())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.SubmitAnswer(11, turn.ID, turn.Card.Question.CorrectOption, "human-answer-001", turn.StartedAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	lateRequest := turn.Deadline.Add(time.Second)
	changed, err := aggregate.AdvanceBots(lateRequest)
	if err != nil || !changed {
		t.Fatalf("late catch-up: changed=%v err=%v", changed, err)
	}
	answer, found := answerFor(aggregate.Turns[0].Answers, BotActorID)
	if !found || !answer.SubmittedAt.Equal(decision.DueAt) || !answer.SubmittedAt.Before(aggregate.Turns[0].Deadline) {
		t.Fatalf("bot answer was not applied at its original due time: answer=%+v decision=%+v", answer, decision)
	}
	version := aggregate.Version
	changed, err = aggregate.AdvanceBots(lateRequest)
	if err != nil || changed || aggregate.Version != version {
		t.Fatalf("same catch-up was not idempotent: changed=%v version=%d/%d err=%v", changed, aggregate.Version, version, err)
	}
}

func TestBotTieBreakAutomationAndRewards(t *testing.T) {
	now := time.Date(2026, time.August, 15, 21, 0, 0, 0, time.UTC)
	aggregate := mustStartedBotMatch(t, BotRandom, now, true)
	mainEnd := now.Add(time.Duration(DeckSize*len(aggregate.Players)) * (TurnDuration + RevealDuration))
	if !aggregate.Tick(mainEnd) || aggregate.Status != StatusTieBreak {
		t.Fatalf("unanswered bot duel did not open tie-break: %+v", aggregate)
	}
	tieTurn := &aggregate.Turns[aggregate.CurrentTurn]
	decision, err := PlanBotDecision(aggregate, tieTurn, aggregate.botPlayer())
	if err != nil {
		t.Fatal(err)
	}
	changed, err := aggregate.AdvanceBots(decision.DueAt)
	if err != nil || !changed {
		t.Fatalf("tie-break bot answer: changed=%v err=%v", changed, err)
	}
	if _, found := answerFor(tieTurn.Answers, BotActorID); !found {
		t.Fatal("bot did not participate in tie-break")
	}

	aggregate.Status = StatusCompleted
	aggregate.IsTie = false
	aggregate.WinnerID = 11
	aggregate.WinnerIDs = []int64{11}
	rewards := aggregate.Rewards()
	if rewards[11] != 80 || rewards[BotActorID] != 0 {
		t.Fatalf("human bot-win rewards are wrong: %v", rewards)
	}
	aggregate.WinnerID = BotActorID
	aggregate.WinnerIDs = []int64{BotActorID}
	rewards = aggregate.Rewards()
	if rewards[11] != 0 || rewards[BotActorID] != 0 {
		t.Fatalf("bot win minted coins: %v", rewards)
	}
}

func mustBotMatch(t *testing.T, strategy BotStrategy, now time.Time) *Aggregate {
	t.Helper()
	aggregate, err := NewBotDuel(909, 808, 11, strategy, bytes.Repeat([]byte{0x2a}, BotSeedSize), now)
	if err != nil {
		t.Fatal(err)
	}
	return aggregate
}

func mustStartedBotMatch(t *testing.T, strategy BotStrategy, now time.Time, withTieBreak bool) *Aggregate {
	t.Helper()
	aggregate := mustBotMatch(t, strategy, now)
	if _, err := aggregate.CommitDeck(11, deck(11, 100), "human-deck-001", now); err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.CommitBotDeck(botDeck(), "system-bot-deck-001", now); err != nil {
		t.Fatal(err)
	}
	var err error
	if withTieBreak {
		_, err = aggregate.StartWithTieBreak(11, tieBreakQuestions(5), "bot-start-0001", now)
	} else {
		_, err = aggregate.Start(11, "bot-start-0001", now)
	}
	if err != nil {
		t.Fatal(err)
	}
	return aggregate
}

func botDeck() []CardSnapshot {
	return deck(BotActorID, 10_000)
}

func cloneBotTurn(source Turn, index int, startedAt time.Time) Turn {
	result := source
	result.ID = "bot-test-turn-" + commandID(index)
	result.StartedAt = startedAt
	result.Deadline = startedAt.Add(TurnDuration)
	result.Status = TurnActive
	result.Answers = nil
	result.EligibleUserIDs = []int64{11, BotActorID}
	return result
}
