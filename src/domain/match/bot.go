package match

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	BotActorID                  int64 = -1
	BotDecisionVersion                = 1
	BotSeedSize                       = 32
	maximumAutomaticTransitions       = 512
)

type PlayerKind string

const (
	PlayerHuman PlayerKind = "human"
	PlayerBot   PlayerKind = "bot"
)

type BotStrategy string

const (
	BotRandom BotStrategy = "random"
	BotSmart  BotStrategy = "smart"
)

var (
	ErrInvalidBotStrategy = errors.New("invalid bot strategy")
	ErrInvalidBotSeed     = errors.New("bot decision seed must contain exactly 32 bytes")
)

type BotConfig struct {
	Strategy        BotStrategy `bson:"strategy" json:"strategy"`
	DecisionVersion int         `bson:"decisionVersion" json:"-"`
	Seed            []byte      `bson:"seed" json:"-"`
}

type BotDecision struct {
	Option    int
	DueAt     time.Time
	CommandID string
}

// EffectiveKind treats old match documents, which predate player kinds, as
// human participants.
func (player *Player) EffectiveKind() PlayerKind {
	if player != nil && player.Kind == PlayerBot {
		return PlayerBot
	}
	return PlayerHuman
}

func (player *Player) IsBot() bool {
	return player != nil && player.EffectiveKind() == PlayerBot
}

func NormalizeBotStrategy(value string) (BotStrategy, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(BotRandom):
		return BotRandom, nil
	case string(BotSmart):
		return BotSmart, nil
	default:
		return "", ErrInvalidBotStrategy
	}
}

// NewBotDuel creates a match-only system participant. The bot is not a User
// and its negative actor ID is scoped to this aggregate.
func NewBotDuel(id, gameID, ownerID int64, strategy BotStrategy, seed []byte, now time.Time) (*Aggregate, error) {
	strategy, err := NormalizeBotStrategy(string(strategy))
	if err != nil {
		return nil, err
	}
	if len(seed) != BotSeedSize {
		return nil, ErrInvalidBotSeed
	}
	if id <= 0 || gameID <= 0 || ownerID <= 0 || now.IsZero() {
		return nil, ErrInvalidMatch
	}
	return &Aggregate{
		ID:      id,
		GameID:  gameID,
		OwnerID: ownerID,
		Mode:    ModeBot,
		Players: []Player{
			{UserID: ownerID, Kind: PlayerHuman},
			{
				UserID: BotActorID,
				Kind:   PlayerBot,
				Bot: &BotConfig{
					Strategy:        strategy,
					DecisionVersion: BotDecisionVersion,
					Seed:            append([]byte(nil), seed...),
				},
			},
		},
		Status:      StatusCollectingDecks,
		CurrentTurn: -1,
		Version:     1,
		CreatedAt:   now.UTC(),
	}, nil
}

// PlanBotDecision derives one stable, server-private decision. It never uses
// global randomness, so retries and reconnect catch-up cannot change history.
func PlanBotDecision(aggregate *Aggregate, turn *Turn, bot *Player) (BotDecision, error) {
	if aggregate == nil || turn == nil || bot == nil || !bot.IsBot() || bot.UserID >= 0 ||
		bot.Bot == nil || len(bot.Bot.Seed) != BotSeedSize ||
		bot.Bot.DecisionVersion != BotDecisionVersion {
		return BotDecision{}, ErrInvalidMatch
	}
	storedBot := aggregate.player(bot.UserID)
	if aggregate.effectiveMode() != ModeBot || storedBot == nil || !storedBot.IsBot() {
		return BotDecision{}, ErrInvalidMatch
	}
	bot = storedBot
	strategy, err := NormalizeBotStrategy(string(bot.Bot.Strategy))
	if err != nil {
		return BotDecision{}, err
	}
	if turn.Status != TurnActive || turn.StartedAt.IsZero() || !turn.Deadline.After(turn.StartedAt) ||
		!containsID(aggregate.eligibleFor(turn), bot.UserID) {
		return BotDecision{}, ErrInvalidTurn
	}
	question := aggregate.questionFor(turn)
	if err := validateQuestion(question); err != nil {
		return BotDecision{}, ErrInvalidTurn
	}

	optionRoll := botDecisionRoll(bot.Bot, aggregate.ID, bot.UserID, turn.ID, "option")
	delayRoll := botDecisionRoll(bot.Bot, aggregate.ID, bot.UserID, turn.ID, "delay")
	option := int(optionRoll % 4)
	minimumDelay := 2 * time.Second
	delayMilliseconds := delayRoll % 17_000
	if strategy == BotSmart {
		accuracy := smartAccuracy(question.Difficulty)
		correctRoll := botDecisionRoll(bot.Bot, aggregate.ID, bot.UserID, turn.ID, "accuracy")
		if int(correctRoll%100) < accuracy {
			option = question.CorrectOption
		} else {
			wrong := int(botDecisionRoll(bot.Bot, aggregate.ID, bot.UserID, turn.ID, "wrong") % 3)
			option = wrong
			if option >= question.CorrectOption {
				option++
			}
		}
		delayMilliseconds = delayRoll % 12_000
	}
	delay := minimumDelay + time.Duration(delayMilliseconds)*time.Millisecond
	dueAt := turn.StartedAt.Add(delay)
	latest := turn.Deadline.Add(-time.Millisecond)
	if dueAt.After(latest) {
		dueAt = latest
	}
	if dueAt.Before(turn.StartedAt) || !dueAt.Before(turn.Deadline) {
		return BotDecision{}, ErrInvalidTurn
	}
	commandDigest := botDecisionDigest(bot.Bot, aggregate.ID, bot.UserID, turn.ID, "command")
	return BotDecision{
		Option:    option,
		DueAt:     dueAt.UTC(),
		CommandID: fmt.Sprintf("bot:%d:%d:v%d:%x", aggregate.ID, bot.UserID, bot.Bot.DecisionVersion, commandDigest[:8]),
	}, nil
}

// AdvanceBots applies bot decisions and timeout/reveal boundaries in timestamp
// order. Calling Tick(until) first would incorrectly discard a bot answer that
// was due before a deadline while the player was disconnected.
func (aggregate *Aggregate) AdvanceBots(until time.Time) (bool, error) {
	if aggregate == nil || until.IsZero() {
		return false, ErrInvalidMatch
	}
	if aggregate.effectiveMode() != ModeBot {
		return aggregate.Tick(until.UTC()), nil
	}
	changed := false
	until = until.UTC()
	for transition := 0; transition < maximumAutomaticTransitions; transition++ {
		if !aggregate.playing() || aggregate.CurrentTurn < 0 || aggregate.CurrentTurn >= len(aggregate.Turns) {
			return changed, nil
		}
		if aggregate.Status == StatusTieBreak && aggregate.TieBreak.AwaitingQuestion {
			return changed, nil
		}
		turn := &aggregate.Turns[aggregate.CurrentTurn]
		switch turn.Status {
		case TurnActive:
			decision, botID, exists, err := aggregate.nextBotDecision(turn)
			if err != nil {
				return changed, err
			}
			if exists && !decision.DueAt.After(until) {
				applied, submitErr := aggregate.SubmitBotAnswer(botID, turn.ID, decision.Option, decision.CommandID, decision.DueAt)
				if submitErr != nil {
					return changed, submitErr
				}
				if !applied {
					return changed, ErrInvalidState
				}
				changed = true
				continue
			}
			if until.Before(turn.Deadline) {
				return changed, nil
			}
			if !aggregate.Tick(turn.Deadline) {
				return changed, ErrInvalidState
			}
			changed = true
		case TurnResolved:
			if until.Before(turn.RevealUntil) {
				return changed, nil
			}
			if !aggregate.Tick(turn.RevealUntil) {
				return changed, ErrInvalidState
			}
			changed = true
		default:
			return changed, ErrInvalidState
		}
	}
	return changed, ErrInvalidState
}

func (aggregate *Aggregate) nextBotDecision(turn *Turn) (BotDecision, int64, bool, error) {
	var selected BotDecision
	var selectedID int64
	found := false
	for index := range aggregate.Players {
		player := &aggregate.Players[index]
		if !player.IsBot() || !containsID(aggregate.eligibleFor(turn), player.UserID) {
			continue
		}
		if _, answered := answerFor(turn.Answers, player.UserID); answered {
			continue
		}
		decision, err := PlanBotDecision(aggregate, turn, player)
		if err != nil {
			return BotDecision{}, 0, false, err
		}
		if !found || decision.DueAt.Before(selected.DueAt) ||
			(decision.DueAt.Equal(selected.DueAt) && player.UserID < selectedID) {
			selected = decision
			selectedID = player.UserID
			found = true
		}
	}
	return selected, selectedID, found, nil
}

func (aggregate *Aggregate) botPlayer() *Player {
	for index := range aggregate.Players {
		if aggregate.Players[index].IsBot() {
			return &aggregate.Players[index]
		}
	}
	return nil
}

func smartAccuracy(difficulty string) int {
	switch strings.ToLower(strings.TrimSpace(difficulty)) {
	case "easy":
		return 85
	case "medium":
		return 70
	case "hard":
		return 55
	default:
		return 65
	}
}

func botDecisionRoll(config *BotConfig, matchID, botID int64, turnID, label string) uint64 {
	digest := botDecisionDigest(config, matchID, botID, turnID, label)
	return binary.BigEndian.Uint64(digest[:8])
}

func botDecisionDigest(config *BotConfig, matchID, botID int64, turnID, label string) [sha256.Size]byte {
	mac := hmac.New(sha256.New, config.Seed)
	_, _ = fmt.Fprintf(mac, "quizbattle-bot|v%d|%d|%d|%s|%s", config.DecisionVersion, matchID, botID, turnID, label)
	var result [sha256.Size]byte
	copy(result[:], mac.Sum(nil))
	return result
}
