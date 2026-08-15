package economy

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type CardStatus string

const (
	CardAvailable    CardStatus = "available"
	CardMatchLocked  CardStatus = "match_locked"
	CardMarketEscrow CardStatus = "market_escrow"
	CardTradeEscrow  CardStatus = "trade_escrow"
)

type ListingStatus string

const (
	ListingActive    ListingStatus = "active"
	ListingSold      ListingStatus = "sold"
	ListingCancelled ListingStatus = "cancelled"
	ListingExpired   ListingStatus = "expired"
)

type TradeStatus string

const (
	TradePending   TradeStatus = "pending"
	TradeAccepted  TradeStatus = "accepted"
	TradeRejected  TradeStatus = "rejected"
	TradeCancelled TradeStatus = "cancelled"
	TradeExpired   TradeStatus = "expired"
)

const (
	StarterBalance int64 = 600
	StarterCards         = 10
	MinimumPrice   int64 = 10
	MaximumPrice   int64 = 100_000
	MarketFeeBasis int64 = 500 // 5% in basis points.
)

var (
	ErrInvalidEconomyState = errors.New("invalid economy state")
	ErrNotOwner            = errors.New("card is not owned by the user")
	ErrCardUnavailable     = errors.New("card is not available")
	ErrInsufficientCoins   = errors.New("insufficient coins")
	ErrInvalidPrice        = errors.New("price must be between 10 and 100000 coins")
	ErrSelfPurchase        = errors.New("seller cannot buy their own listing")
	ErrInvalidListing      = errors.New("listing is not active")
	ErrInvalidTrade        = errors.New("trade offer is invalid")
)

type Wallet struct {
	UserID    int64     `bson:"userId" json:"userId,string"`
	Balance   int64     `bson:"balance" json:"balance"`
	Locked    int64     `bson:"locked" json:"locked"`
	Version   int64     `bson:"version" json:"version"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

type Card struct {
	ID         int64      `bson:"id" json:"id,string"`
	OwnerID    int64      `bson:"ownerId" json:"ownerId,string"`
	QuestionID string     `bson:"questionId" json:"questionId"`
	Edition    int        `bson:"edition" json:"edition"`
	Rarity     string     `bson:"rarity" json:"rarity"`
	Power      int        `bson:"power" json:"power"`
	Plays      int        `bson:"plays" json:"plays"`
	Wins       int        `bson:"wins" json:"wins"`
	Status     CardStatus `bson:"status" json:"status"`
	LockRef    string     `bson:"lockRef,omitempty" json:"lockRef,omitempty"`
	Version    int64      `bson:"version" json:"version"`
	CreatedAt  time.Time  `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time  `bson:"updatedAt" json:"updatedAt"`
}

type Listing struct {
	ID                  int64         `bson:"id" json:"id,string"`
	CardID              int64         `bson:"cardId" json:"cardId,string"`
	SellerID            int64         `bson:"sellerId" json:"sellerId,string"`
	BuyerID             int64         `bson:"buyerId,omitempty" json:"buyerId,omitempty,string"`
	Price               int64         `bson:"price" json:"price"`
	Fee                 int64         `bson:"fee,omitempty" json:"fee,omitempty"`
	Status              ListingStatus `bson:"status" json:"status"`
	CreatedAt           time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt           time.Time     `bson:"updatedAt" json:"updatedAt"`
	ExpiresAt           time.Time     `bson:"expiresAt" json:"expiresAt"`
	Version             int64         `bson:"version" json:"version"`
	CommandID           string        `bson:"commandId" json:"-"`
	SettlementCommandID string        `bson:"settlementCommandId,omitempty" json:"-"`
	CancelCommandID     string        `bson:"cancelCommandId,omitempty" json:"-"`
}

type TradeOffer struct {
	ID                  int64       `bson:"id" json:"id,string"`
	SenderID            int64       `bson:"senderId" json:"senderId,string"`
	ReceiverID          int64       `bson:"receiverId" json:"receiverId,string"`
	OfferedCardIDs      []int64     `bson:"offeredCardIds" json:"offeredCardIds"`
	RequestedCardIDs    []int64     `bson:"requestedCardIds" json:"requestedCardIds"`
	OfferedCoins        int64       `bson:"offeredCoins" json:"offeredCoins"`
	RequestedCoins      int64       `bson:"requestedCoins" json:"requestedCoins"`
	Status              TradeStatus `bson:"status" json:"status"`
	CreatedAt           time.Time   `bson:"createdAt" json:"createdAt"`
	UpdatedAt           time.Time   `bson:"updatedAt" json:"updatedAt"`
	ExpiresAt           time.Time   `bson:"expiresAt" json:"expiresAt"`
	Version             int64       `bson:"version" json:"version"`
	CommandID           string      `bson:"commandId" json:"-"`
	SettlementCommandID string      `bson:"settlementCommandId,omitempty" json:"-"`
}

type LedgerEntry struct {
	ID             int64     `bson:"id" json:"id,string"`
	UserID         int64     `bson:"userId" json:"userId,string"`
	CoinDelta      int64     `bson:"coinDelta" json:"coinDelta"`
	BalanceAfter   int64     `bson:"balanceAfter" json:"balanceAfter"`
	CardID         int64     `bson:"cardId,omitempty" json:"cardId,omitempty,string"`
	PreviousOwner  int64     `bson:"previousOwnerId,omitempty" json:"previousOwnerId,omitempty,string"`
	NewOwner       int64     `bson:"newOwnerId,omitempty" json:"newOwnerId,omitempty,string"`
	Reason         string    `bson:"reason" json:"reason"`
	ReferenceType  string    `bson:"referenceType" json:"referenceType"`
	ReferenceID    string    `bson:"referenceId" json:"referenceId"`
	IdempotencyKey string    `bson:"idempotencyKey" json:"idempotencyKey"`
	EntryPart      string    `bson:"entryPart" json:"entryPart"`
	CreatedAt      time.Time `bson:"createdAt" json:"createdAt"`
}

func NewListing(id, sellerID int64, card *Card, price int64, now time.Time) (*Listing, error) {
	if id <= 0 || sellerID <= 0 || card == nil || now.IsZero() {
		return nil, ErrInvalidEconomyState
	}
	if err := ValidatePrice(price); err != nil {
		return nil, err
	}
	if card.OwnerID != sellerID {
		return nil, ErrNotOwner
	}
	if card.Status != CardAvailable || card.LockRef != "" {
		return nil, ErrCardUnavailable
	}
	listing := &Listing{
		ID: id, CardID: card.ID, SellerID: sellerID, Price: price,
		Status: ListingActive, CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
		ExpiresAt: now.UTC().Add(7 * 24 * time.Hour), Version: 1,
	}
	card.Status = CardMarketEscrow
	card.LockRef = fmt.Sprintf("listing:%d", id)
	card.UpdatedAt = now.UTC()
	card.Version++
	return listing, nil
}

// SettlePurchase applies the in-memory invariants used by the transactional
// repository. The caller must persist all four mutations and ledger entries in
// one database transaction.
func SettlePurchase(listing *Listing, card *Card, buyer, seller *Wallet, buyerID int64, now time.Time) (int64, error) {
	if listing == nil || card == nil || buyer == nil || seller == nil || buyerID <= 0 || now.IsZero() {
		return 0, ErrInvalidEconomyState
	}
	if listing.Status != ListingActive || !now.Before(listing.ExpiresAt) {
		return 0, ErrInvalidListing
	}
	if listing.SellerID == buyerID {
		return 0, ErrSelfPurchase
	}
	if buyer.UserID != buyerID || seller.UserID != listing.SellerID || card.OwnerID != listing.SellerID ||
		card.ID != listing.CardID || card.Status != CardMarketEscrow || card.LockRef != fmt.Sprintf("listing:%d", listing.ID) {
		return 0, ErrInvalidEconomyState
	}
	if buyer.Balance-buyer.Locked < listing.Price {
		return 0, ErrInsufficientCoins
	}
	fee := MarketFee(listing.Price)
	buyer.Balance -= listing.Price
	seller.Balance += listing.Price - fee
	buyer.Version++
	seller.Version++
	buyer.UpdatedAt = now.UTC()
	seller.UpdatedAt = now.UTC()
	card.OwnerID = buyerID
	card.Status = CardAvailable
	card.LockRef = ""
	card.UpdatedAt = now.UTC()
	card.Version++
	listing.BuyerID = buyerID
	listing.Fee = fee
	listing.Status = ListingSold
	listing.UpdatedAt = now.UTC()
	listing.Version++
	return fee, nil
}

func CancelListing(listing *Listing, card *Card, sellerID int64, now time.Time) error {
	if listing == nil || card == nil || listing.Status != ListingActive || listing.SellerID != sellerID ||
		card.OwnerID != sellerID || card.ID != listing.CardID || card.Status != CardMarketEscrow ||
		card.LockRef != fmt.Sprintf("listing:%d", listing.ID) {
		return ErrInvalidListing
	}
	listing.Status = ListingCancelled
	listing.UpdatedAt = now.UTC()
	listing.Version++
	card.Status = CardAvailable
	card.LockRef = ""
	card.UpdatedAt = now.UTC()
	card.Version++
	return nil
}

func ValidateTrade(offer *TradeOffer) error {
	if offer == nil || offer.ID <= 0 || offer.SenderID <= 0 || offer.ReceiverID <= 0 || offer.SenderID == offer.ReceiverID ||
		offer.OfferedCoins < 0 || offer.RequestedCoins < 0 || offer.OfferedCoins > MaximumPrice || offer.RequestedCoins > MaximumPrice ||
		(len(offer.OfferedCardIDs) == 0 && len(offer.RequestedCardIDs) == 0 && offer.OfferedCoins == 0 && offer.RequestedCoins == 0) {
		return ErrInvalidTrade
	}
	if len(offer.OfferedCardIDs) > 5 || len(offer.RequestedCardIDs) > 5 || hasDuplicate(offer.OfferedCardIDs) || hasDuplicate(offer.RequestedCardIDs) {
		return ErrInvalidTrade
	}
	return nil
}

func ValidatePrice(price int64) error {
	if price < MinimumPrice || price > MaximumPrice {
		return ErrInvalidPrice
	}
	return nil
}

func MarketFee(price int64) int64 {
	fee := price * MarketFeeBasis / 10_000
	if fee < 1 {
		return 1
	}
	return fee
}

func RarityForDifficulty(difficulty string) string {
	switch strings.ToLower(strings.TrimSpace(difficulty)) {
	case "hard":
		return "epic"
	case "medium":
		return "rare"
	default:
		return "common"
	}
}

func hasDuplicate(values []int64) bool {
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			return true
		}
		if _, exists := seen[value]; exists {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}
