package match

import (
	"bytes"
	"testing"
	"time"
)

func TestRewardCandidatesPVPWinnerMintsCardAndKeepsCoinRules(t *testing.T) {
	aggregate := &Aggregate{
		ID: 91, Mode: ModeDuel, Status: StatusCompleted,
		Players:  []Player{{UserID: 11}, {UserID: 22}},
		WinnerID: 11, WinnerIDs: []int64{11},
		RewardPolicyVersion: RewardPolicyV1,
		RewardNonce:         bytes.Repeat([]byte{1}, RewardNonceSize),
	}
	candidates, err := aggregate.RewardCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidate count = %d", len(candidates))
	}
	if got := candidates[0]; got.UserID != 11 || got.Source != RewardSourcePVP || got.Outcome != RewardOutcomeChampion || got.Coins != 120 || !got.GrantsCard {
		t.Fatalf("winner candidate = %+v", got)
	}
	if got := candidates[1]; got.UserID != 22 || got.Outcome != RewardOutcomeLoss || got.Coins != 45 || got.GrantsCard {
		t.Fatalf("loser candidate = %+v", got)
	}
}

func TestRewardCandidatesBotUsesStrategyAndExcludesSystemActor(t *testing.T) {
	for _, test := range []struct {
		name     string
		strategy BotStrategy
		coins    int64
	}{
		{name: "random", strategy: BotRandom, coins: BotRandomWinnerCoins},
		{name: "smart", strategy: BotSmart, coins: BotSmartWinnerCoins},
	} {
		t.Run(test.name, func(t *testing.T) {
			aggregate := rewardBotAggregate(test.strategy, 41)
			candidates, err := aggregate.RewardCandidates()
			if err != nil {
				t.Fatal(err)
			}
			if len(candidates) != 1 {
				t.Fatalf("bot was included in rewards: %+v", candidates)
			}
			got := candidates[0]
			if got.UserID != 41 || got.Source != RewardSourceBot || got.BotStrategy != test.strategy || got.Outcome != RewardOutcomeChampion || got.Coins != test.coins || !got.GrantsCard {
				t.Fatalf("candidate = %+v", got)
			}
		})
	}
}

func TestRewardCandidatesBotLossPaysNothing(t *testing.T) {
	aggregate := rewardBotAggregate(BotSmart, BotActorID)
	candidates, err := aggregate.RewardCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].Outcome != RewardOutcomeLoss || candidates[0].Coins != 0 || candidates[0].GrantsCard {
		t.Fatalf("loss candidate = %+v", candidates)
	}
}

func TestRewardCandidatesLegacyRemainsCoinOnly(t *testing.T) {
	aggregate := &Aggregate{
		ID: 92, Mode: ModeDuel, Status: StatusCompleted,
		Players: []Player{{UserID: 51}, {UserID: 52}}, WinnerID: 51,
	}
	candidates, err := aggregate.RewardCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if candidates[0].Coins != 120 || candidates[0].GrantsCard || candidates[1].Coins != 45 || candidates[1].GrantsCard {
		t.Fatalf("legacy rewards changed: %+v", candidates)
	}
}

func TestRewardCandidatesRequiresPrivateNonceForV1(t *testing.T) {
	aggregate := &Aggregate{
		ID: 93, Mode: ModeDuel, Status: StatusCompleted,
		Players: []Player{{UserID: 61}, {UserID: 62}}, WinnerID: 61,
		RewardPolicyVersion: RewardPolicyV1,
	}
	if _, err := aggregate.RewardCandidates(); err == nil {
		t.Fatal("accepted rewards-v1 without a private nonce")
	}
}

func TestInitializeRewardPolicyIsIdempotent(t *testing.T) {
	aggregate := &Aggregate{ID: 94}
	if err := aggregate.InitializeRewardPolicy(); err != nil {
		t.Fatal(err)
	}
	first := append([]byte(nil), aggregate.RewardNonce...)
	if aggregate.RewardPolicyVersion != RewardPolicyV1 || len(first) != RewardNonceSize {
		t.Fatalf("policy was not initialized: %+v", aggregate)
	}
	if err := aggregate.InitializeRewardPolicy(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, aggregate.RewardNonce) {
		t.Fatal("idempotent initialization replaced reward entropy")
	}
}

func TestRewardRarityDistributionsHaveExactBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		candidate RewardCandidate
		common    int
		rare      int
		epic      int
	}{
		{name: "pvp", candidate: RewardCandidate{Source: RewardSourcePVP}, common: 60, rare: 32, epic: 8},
		{name: "smart", candidate: RewardCandidate{Source: RewardSourceBot, BotStrategy: BotSmart}, common: 70, rare: 25, epic: 5},
		{name: "random", candidate: RewardCandidate{Source: RewardSourceBot, BotStrategy: BotRandom}, common: 85, rare: 14, epic: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			commonBoundary, rareBoundary, err := rewardRarityBoundaries(test.candidate)
			if err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			for roll := 0; roll < 100; roll++ {
				counts[rarityFromRoll(roll, commonBoundary, rareBoundary)]++
			}
			if counts["common"] != test.common || counts["rare"] != test.rare || counts["epic"] != test.epic {
				t.Fatalf("distribution = %+v", counts)
			}
		})
	}
}

func TestRewardQuestionRankIsStableAndPrivate(t *testing.T) {
	nonce := bytes.Repeat([]byte{7}, RewardNonceSize)
	first, err := RewardQuestionRank(nonce, 101, 71, "science-001")
	if err != nil {
		t.Fatal(err)
	}
	replay, err := RewardQuestionRank(nonce, 101, 71, "science-001")
	if err != nil || replay != first {
		t.Fatalf("unstable rank first=%d replay=%d err=%v", first, replay, err)
	}
	other, err := RewardQuestionRank(bytes.Repeat([]byte{8}, RewardNonceSize), 101, 71, "science-001")
	if err != nil {
		t.Fatal(err)
	}
	if other == first {
		t.Fatal("different private entropy produced the same test rank")
	}
}

func TestRewardForReturnsIndependentCardCopy(t *testing.T) {
	aggregate := &Aggregate{RewardReceipts: []RewardReceipt{{
		UserID: 81, PolicyVersion: RewardPolicyV1, Status: RewardStatusGranted,
		Card: &RewardCard{ID: 500, QuestionID: "science-001"}, SettledAt: time.Now().UTC(),
	}}}
	receipt := aggregate.RewardFor(81)
	if receipt == nil || receipt.Card == nil {
		t.Fatal("reward receipt missing")
	}
	receipt.Card.QuestionID = "changed"
	if aggregate.RewardReceipts[0].Card.QuestionID != "science-001" {
		t.Fatal("RewardFor leaked the persisted card pointer")
	}
}

func TestSafeSnapshotUsesPersistedRewardReceipt(t *testing.T) {
	aggregate := &Aggregate{
		ID: 96, GameID: 196, OwnerID: 91, Mode: ModeBot, Status: StatusCompleted,
		Players: []Player{
			{UserID: 91, Kind: PlayerHuman},
			{UserID: BotActorID, Kind: PlayerBot, Bot: &BotConfig{Strategy: BotRandom}},
		},
		WinnerID: 91, WinnerIDs: []int64{91}, RewardPolicyVersion: RewardPolicyV1,
		RewardNonce: bytes.Repeat([]byte{3}, RewardNonceSize), RewardsSettled: true,
		RewardReceipts: []RewardReceipt{{
			UserID: 91, PolicyVersion: RewardPolicyV1, Source: RewardSourceBot,
			BotStrategy: BotRandom, Outcome: RewardOutcomeChampion,
			Status: RewardStatusCapped, CoinsGranted: 0, Reason: RewardReasonBotDailyCap,
			SettledAt: time.Now().UTC(),
		}},
	}
	snapshot, err := aggregate.SafeSnapshot(91)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Reward == nil || snapshot.Reward.Status != RewardStatusCapped || snapshot.Reward.Reason != RewardReasonBotDailyCap || snapshot.RewardCoins != 0 {
		t.Fatalf("snapshot did not expose persisted receipt: %+v", snapshot.Reward)
	}
}

func rewardBotAggregate(strategy BotStrategy, winnerID int64) *Aggregate {
	return &Aggregate{
		ID: 95, Mode: ModeBot, Status: StatusCompleted,
		Players: []Player{
			{UserID: 41, Kind: PlayerHuman},
			{UserID: BotActorID, Kind: PlayerBot, Bot: &BotConfig{Strategy: strategy}},
		},
		WinnerID: winnerID, WinnerIDs: []int64{winnerID},
		RewardPolicyVersion: RewardPolicyV1,
		RewardNonce:         bytes.Repeat([]byte{2}, RewardNonceSize),
	}
}
