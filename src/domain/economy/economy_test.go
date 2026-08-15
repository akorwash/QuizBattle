package economy

import (
	"errors"
	"testing"
	"time"
)

func TestMarketplaceSettlement(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	card := &Card{ID: 7, OwnerID: 11, QuestionID: "science-001", Status: CardAvailable, Version: 1}
	listing, err := NewListing(99, 11, card, 100, now)
	if err != nil {
		t.Fatal(err)
	}
	if card.Status != CardMarketEscrow || card.LockRef != "listing:99" {
		t.Fatalf("card was not escrowed: %+v", card)
	}
	buyer := &Wallet{UserID: 22, Balance: 600, Version: 1}
	seller := &Wallet{UserID: 11, Balance: 600, Version: 1}
	fee, err := SettlePurchase(listing, card, buyer, seller, 22, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if fee != 5 || buyer.Balance != 500 || seller.Balance != 695 || card.OwnerID != 22 || card.Status != CardAvailable || listing.Status != ListingSold {
		t.Fatalf("bad settlement fee=%d buyer=%+v seller=%+v card=%+v listing=%+v", fee, buyer, seller, card, listing)
	}
}

func TestMarketplaceRejectsAbuse(t *testing.T) {
	now := time.Now().UTC()
	card := &Card{ID: 7, OwnerID: 11, Status: CardAvailable}
	if _, err := NewListing(9, 22, card, 100, now); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("wrong owner: %v", err)
	}
	listing, _ := NewListing(9, 11, card, 100, now)
	if _, err := SettlePurchase(listing, card, &Wallet{UserID: 11, Balance: 1000}, &Wallet{UserID: 11}, 11, now); !errors.Is(err, ErrSelfPurchase) {
		t.Fatalf("self purchase: %v", err)
	}
	if _, err := SettlePurchase(listing, card, &Wallet{UserID: 22, Balance: 5}, &Wallet{UserID: 11}, 22, now); !errors.Is(err, ErrInsufficientCoins) {
		t.Fatalf("insufficient balance: %v", err)
	}
}

func TestFeeAndTradeValidation(t *testing.T) {
	if MarketFee(10) != 1 || MarketFee(100) != 5 || MarketFee(101) != 5 {
		t.Fatal("unexpected market fee rounding")
	}
	valid := &TradeOffer{ID: 1, SenderID: 11, ReceiverID: 22, OfferedCardIDs: []int64{1}, RequestedCardIDs: []int64{2}}
	if err := ValidateTrade(valid); err != nil {
		t.Fatal(err)
	}
	valid.OfferedCardIDs = []int64{1, 1}
	if err := ValidateTrade(valid); !errors.Is(err, ErrInvalidTrade) {
		t.Fatalf("duplicate cards accepted: %v", err)
	}
}
