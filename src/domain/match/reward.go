package match

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const (
	RewardPolicyV1       = "rewards-v1"
	RewardNonceSize      = 32
	BotDailyRewardLimit  = 3
	PVPDailyRewardLimit  = 10
	BotRandomWinnerCoins = 60
	BotSmartWinnerCoins  = 100
)

type RewardSource string

const (
	RewardSourcePVP RewardSource = "pvp"
	RewardSourceBot RewardSource = "bot"
)

type RewardOutcome string

const (
	RewardOutcomeChampion   RewardOutcome = "champion"
	RewardOutcomeTeamWinner RewardOutcome = "team_winner"
	RewardOutcomeLoss       RewardOutcome = "loss"
	RewardOutcomeDraw       RewardOutcome = "draw"
	RewardOutcomeForfeit    RewardOutcome = "forfeit"
)

type RewardStatus string

const (
	RewardStatusGranted    RewardStatus = "granted"
	RewardStatusCapped     RewardStatus = "capped"
	RewardStatusIneligible RewardStatus = "ineligible"
)

const (
	RewardReasonBotDailyCap = "bot_daily_cap"
	RewardReasonPVPDailyCap = "pvp_daily_cap"
)

var ErrInvalidRewardPolicy = errors.New("invalid match reward policy")

// RewardCard is a safe, immutable summary of a card minted by match
// settlement. It intentionally contains no answer or explanation data.
type RewardCard struct {
	ID         int64  `bson:"id" json:"id,string"`
	QuestionID string `bson:"questionId" json:"questionId"`
	Category   string `bson:"category" json:"category"`
	Difficulty string `bson:"difficulty" json:"difficulty"`
	Rarity     string `bson:"rarity" json:"rarity"`
	Power      int    `bson:"power" json:"power"`
	Edition    int    `bson:"edition" json:"edition"`
}

// RewardReceipt is the durable, viewer-specific outcome of one settlement.
// Receipts, wallets, cards, ledger rows, quotas and the terminal settlement
// marker are committed together in one MongoDB transaction.
type RewardReceipt struct {
	UserID        int64         `bson:"userId" json:"userId,string"`
	PolicyVersion string        `bson:"policyVersion" json:"policyVersion"`
	Source        RewardSource  `bson:"source" json:"source"`
	BotStrategy   BotStrategy   `bson:"botStrategy,omitempty" json:"botStrategy,omitempty"`
	Outcome       RewardOutcome `bson:"outcome" json:"outcome"`
	Status        RewardStatus  `bson:"status" json:"status"`
	CoinsGranted  int64         `bson:"coinsGranted" json:"coinsGranted"`
	Card          *RewardCard   `bson:"card,omitempty" json:"card,omitempty"`
	Reason        string        `bson:"reason,omitempty" json:"reason,omitempty"`
	SettledAt     time.Time     `bson:"settledAt" json:"settledAt"`
}

// RewardCandidate contains the server-authoritative reward intent before
// quota reservation and card minting. It is never accepted from a client.
type RewardCandidate struct {
	UserID      int64
	Source      RewardSource
	BotStrategy BotStrategy
	Outcome     RewardOutcome
	Coins       int64
	GrantsCard  bool
}

// InitializeRewardPolicy stamps a newly-created match with the current policy
// and private entropy. It is idempotent so creation services may safely retry
// before persistence. Legacy matches deliberately keep an empty version.
func (aggregate *Aggregate) InitializeRewardPolicy() error {
	if aggregate == nil || aggregate.ID <= 0 {
		return ErrInvalidRewardPolicy
	}
	if aggregate.RewardPolicyVersion != "" && aggregate.RewardPolicyVersion != RewardPolicyV1 {
		return ErrInvalidRewardPolicy
	}
	if aggregate.RewardPolicyVersion == RewardPolicyV1 && len(aggregate.RewardNonce) == RewardNonceSize {
		return nil
	}
	if len(aggregate.RewardNonce) != 0 {
		return ErrInvalidRewardPolicy
	}
	nonce := make([]byte, RewardNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generate reward nonce: %w", err)
	}
	aggregate.RewardPolicyVersion = RewardPolicyV1
	aggregate.RewardNonce = nonce
	return nil
}

func (aggregate *Aggregate) RewardFor(userID int64) *RewardReceipt {
	if aggregate == nil || userID <= 0 {
		return nil
	}
	for index := range aggregate.RewardReceipts {
		if aggregate.RewardReceipts[index].UserID == userID {
			result := aggregate.RewardReceipts[index]
			if result.Card != nil {
				card := *result.Card
				result.Card = &card
			}
			return &result
		}
	}
	return nil
}

// RewardCandidates preserves legacy coin-only settlements while applying the
// versioned bot/card rules only to newly-stamped matches.
func (aggregate *Aggregate) RewardCandidates() ([]RewardCandidate, error) {
	if aggregate == nil || len(aggregate.Players) == 0 ||
		(aggregate.Status != StatusCompleted && aggregate.Status != StatusForfeited) {
		return nil, ErrInvalidRewardPolicy
	}
	if aggregate.RewardPolicyVersion != "" && aggregate.RewardPolicyVersion != RewardPolicyV1 {
		return nil, ErrInvalidRewardPolicy
	}
	if aggregate.RewardPolicyVersion == RewardPolicyV1 && len(aggregate.RewardNonce) != RewardNonceSize {
		return nil, ErrInvalidRewardPolicy
	}

	humans := make([]Player, 0, len(aggregate.Players))
	for _, player := range aggregate.Players {
		if !player.IsBot() {
			humans = append(humans, player)
		}
	}
	if len(humans) == 0 {
		return nil, ErrInvalidRewardPolicy
	}

	winners := aggregate.rewardWinnerSet()
	source := RewardSourcePVP
	strategy := BotStrategy("")
	if aggregate.effectiveMode() == ModeBot {
		source = RewardSourceBot
		bot := aggregate.botPlayer()
		if bot == nil || bot.Bot == nil {
			return nil, ErrInvalidRewardPolicy
		}
		var err error
		strategy, err = NormalizeBotStrategy(string(bot.Bot.Strategy))
		if err != nil {
			return nil, ErrInvalidRewardPolicy
		}
	}

	legacyCoins := aggregate.Rewards()
	result := make([]RewardCandidate, 0, len(humans))
	for _, player := range humans {
		outcome := aggregate.rewardOutcome(player.UserID, winners)
		candidate := RewardCandidate{UserID: player.UserID, Source: source, BotStrategy: strategy, Outcome: outcome}
		if aggregate.RewardPolicyVersion == "" {
			candidate.Coins = legacyCoins[player.UserID]
			result = append(result, candidate)
			continue
		}
		if source == RewardSourceBot {
			if outcome == RewardOutcomeChampion {
				candidate.GrantsCard = true
				switch strategy {
				case BotRandom:
					candidate.Coins = BotRandomWinnerCoins
				case BotSmart:
					candidate.Coins = BotSmartWinnerCoins
				default:
					return nil, ErrInvalidRewardPolicy
				}
			}
		} else {
			candidate.Coins = legacyCoins[player.UserID]
			candidate.GrantsCard = outcome == RewardOutcomeChampion || outcome == RewardOutcomeTeamWinner
		}
		result = append(result, candidate)
	}
	return result, nil
}

func (aggregate *Aggregate) rewardWinnerSet() map[int64]struct{} {
	result := make(map[int64]struct{}, len(aggregate.WinnerIDs)+1)
	if aggregate.Status != StatusCompleted || aggregate.IsTie {
		return result
	}
	for _, userID := range aggregate.WinnerIDs {
		result[userID] = struct{}{}
	}
	if len(result) == 0 && aggregate.WinnerID != 0 {
		result[aggregate.WinnerID] = struct{}{}
	}
	return result
}

func (aggregate *Aggregate) rewardOutcome(userID int64, winners map[int64]struct{}) RewardOutcome {
	if aggregate.Status == StatusForfeited {
		return RewardOutcomeForfeit
	}
	if aggregate.IsTie {
		return RewardOutcomeDraw
	}
	if userID == aggregate.WinnerID {
		return RewardOutcomeChampion
	}
	if _, won := winners[userID]; won {
		return RewardOutcomeTeamWinner
	}
	return RewardOutcomeLoss
}

// RewardRarity deterministically applies the source distribution using the
// private per-match nonce. The nonce must never be sent to a client.
func RewardRarity(nonce []byte, matchID int64, candidate RewardCandidate) (string, error) {
	roll, err := rewardRoll(nonce, matchID, candidate.UserID, "rarity")
	if err != nil {
		return "", err
	}
	value := int(roll % 100)
	commonBoundary, rareBoundary, err := rewardRarityBoundaries(candidate)
	if err != nil {
		return "", err
	}
	return rarityFromRoll(value, commonBoundary, rareBoundary), nil
}

// RewardQuestionRank provides a stable private ordering for unique-first card
// selection, independent of Mongo cursor order.
func RewardQuestionRank(nonce []byte, matchID, userID int64, questionID string) (uint64, error) {
	return rewardRoll(nonce, matchID, userID, "question:"+questionID)
}

func rewardRoll(nonce []byte, matchID, userID int64, label string) (uint64, error) {
	if len(nonce) != RewardNonceSize || matchID <= 0 || userID <= 0 || label == "" {
		return 0, ErrInvalidRewardPolicy
	}
	mac := hmac.New(sha256.New, nonce)
	_, _ = fmt.Fprintf(mac, "quizbattle-reward|%s|%d|%d|%s", RewardPolicyV1, matchID, userID, label)
	digest := mac.Sum(nil)
	return binary.BigEndian.Uint64(digest[:8]), nil
}

func rarityFromRoll(value, commonBoundary, rareBoundary int) string {
	if value < commonBoundary {
		return "common"
	}
	if value < rareBoundary {
		return "rare"
	}
	return "epic"
}

func rewardRarityBoundaries(candidate RewardCandidate) (int, int, error) {
	if candidate.Source == RewardSourcePVP {
		return 60, 92, nil
	}
	if candidate.Source != RewardSourceBot {
		return 0, 0, ErrInvalidRewardPolicy
	}
	switch candidate.BotStrategy {
	case BotRandom:
		return 85, 99, nil
	case BotSmart:
		return 70, 95, nil
	default:
		return 0, 0, ErrInvalidRewardPolicy
	}
}
