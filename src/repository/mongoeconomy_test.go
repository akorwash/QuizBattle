package repository

import (
	"testing"

	"github.com/akorwash/QuizBattle/domain/economy"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestSameTradeRequestComparesCompleteCommandPayload(t *testing.T) {
	base := economy.TradeOffer{
		ID: 1, SenderID: 11, ReceiverID: 22,
		OfferedCardIDs: []int64{101, 102}, RequestedCardIDs: []int64{201, 202},
		OfferedCoins: 30, RequestedCoins: 40, CommandID: "trade-command-001",
	}
	equivalent := base
	equivalent.ID = 999
	if !sameTradeRequest(base, equivalent) {
		t.Fatal("database-generated fields changed idempotency equivalence")
	}

	tests := []struct {
		name   string
		mutate func(*economy.TradeOffer)
	}{
		{"sender", func(offer *economy.TradeOffer) { offer.SenderID++ }},
		{"receiver", func(offer *economy.TradeOffer) { offer.ReceiverID++ }},
		{"offered coins", func(offer *economy.TradeOffer) { offer.OfferedCoins++ }},
		{"requested coins", func(offer *economy.TradeOffer) { offer.RequestedCoins++ }},
		{"offered cards", func(offer *economy.TradeOffer) { offer.OfferedCardIDs = []int64{101, 103} }},
		{"requested cards", func(offer *economy.TradeOffer) { offer.RequestedCardIDs = []int64{201, 203} }},
		{"offered card order", func(offer *economy.TradeOffer) { offer.OfferedCardIDs = []int64{102, 101} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := base
			candidate.OfferedCardIDs = append([]int64(nil), base.OfferedCardIDs...)
			candidate.RequestedCardIDs = append([]int64(nil), base.RequestedCardIDs...)
			test.mutate(&candidate)
			if sameTradeRequest(base, candidate) {
				t.Fatal("mismatched payload was accepted as an idempotent replay")
			}
		})
	}
}

func TestMatchedAndModifiedRequiresExactCounts(t *testing.T) {
	if err := matchedAndModified(&mongo.UpdateResult{MatchedCount: 5, ModifiedCount: 5}, 5); err != nil {
		t.Fatal(err)
	}
	for _, result := range []*mongo.UpdateResult{
		nil,
		{MatchedCount: 4, ModifiedCount: 4},
		{MatchedCount: 5, ModifiedCount: 4},
	} {
		if err := matchedAndModified(result, 5); err == nil {
			t.Fatalf("accepted mismatched update result: %#v", result)
		}
	}
	if err := matchedAndModified(&mongo.UpdateResult{}, 0); err != nil {
		t.Fatalf("zero expected mutations should be valid: %v", err)
	}
}

func TestBoundedSweepLimit(t *testing.T) {
	for input, want := range map[int64]int64{-1: 25, 0: 25, 1: 1, 25: 25, 100: 100, 101: 25} {
		if got := boundedSweepLimit(input); got != want {
			t.Fatalf("limit %d: got %d want %d", input, got, want)
		}
	}
}
