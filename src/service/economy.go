package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/akorwash/QuizBattle/domain/economy"
	"github.com/akorwash/QuizBattle/domain/question"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
)

type EconomyRepository interface {
	EnsureStarter(ctx context.Context, userID int64, questions []question.Question) error
	GetWallet(ctx context.Context, userID int64) (*economy.Wallet, error)
	ListCards(ctx context.Context, userID int64) ([]economy.Card, error)
	GetCardsByIDs(ctx context.Context, ids []int64) (map[int64]economy.Card, error)
	CreateListing(ctx context.Context, sellerID, cardID, price int64, commandID string) (*economy.Listing, error)
	ExpireListings(ctx context.Context, limit int64) (int64, error)
	ListActiveListings(ctx context.Context, limit int64) ([]economy.Listing, error)
	BuyListing(ctx context.Context, buyerID, listingID int64, commandID string) (*economy.Listing, error)
	CancelListing(ctx context.Context, sellerID, listingID int64, commandID string) (*economy.Listing, error)
	CreateTrade(ctx context.Context, senderID, receiverID int64, offeredCardIDs, requestedCardIDs []int64, offeredCoins, requestedCoins int64, commandID string) (*economy.TradeOffer, error)
	ExpireTrades(ctx context.Context, limit int64) (int64, error)
	ListTrades(ctx context.Context, userID int64) ([]economy.TradeOffer, error)
	AcceptTrade(ctx context.Context, receiverID, tradeID int64, commandID string) (*economy.TradeOffer, error)
	CloseTrade(ctx context.Context, actorID, tradeID int64, action, commandID string) (*economy.TradeOffer, error)
}

const lazyEconomySweepLimit int64 = 25

type EconomyService struct {
	repository   EconomyRepository
	questionBank *QuestionBankService
}

func NewEconomyService(repository EconomyRepository, questionBank *QuestionBankService) *EconomyService {
	return &EconomyService{repository: repository, questionBank: questionBank}
}

func (service *EconomyService) Collection(ctx context.Context, userID int64) (*resources.Collection, error) {
	if userID <= 0 {
		return nil, ErrForbidden
	}
	wallet, err := service.ensureStarter(ctx, userID)
	if err != nil {
		return nil, err
	}
	cards, err := service.repository.ListCards(ctx, userID)
	if err != nil {
		return nil, err
	}
	return service.mapCollection(ctx, wallet, cards)
}

func (service *EconomyService) CreateListing(ctx context.Context, userID int64, model resources.CreateListingModel) (*resources.MarketListing, error) {
	if _, err := service.ensureStarter(ctx, userID); err != nil {
		return nil, err
	}
	listing, err := service.repository.CreateListing(ctx, userID, model.CardID, model.Price, model.CommandID)
	if err != nil {
		return nil, err
	}
	return service.mapListing(ctx, *listing)
}

func (service *EconomyService) Market(ctx context.Context) ([]resources.MarketListing, error) {
	if _, err := service.repository.ExpireListings(ctx, lazyEconomySweepLimit); err != nil {
		return nil, err
	}
	listings, err := service.repository.ListActiveListings(ctx, 50)
	if err != nil {
		return nil, err
	}
	return service.mapListings(ctx, listings)
}

func (service *EconomyService) BuyListing(ctx context.Context, userID, listingID int64, commandID string) (*resources.MarketListing, error) {
	if _, err := service.ensureStarter(ctx, userID); err != nil {
		return nil, err
	}
	if _, err := service.repository.ExpireListings(ctx, lazyEconomySweepLimit); err != nil {
		return nil, err
	}
	listing, err := service.repository.BuyListing(ctx, userID, listingID, commandID)
	if err != nil {
		return nil, err
	}
	return service.mapListing(ctx, *listing)
}

func (service *EconomyService) CancelListing(ctx context.Context, userID, listingID int64, commandID string) (*resources.MarketListing, error) {
	listing, err := service.repository.CancelListing(ctx, userID, listingID, commandID)
	if err != nil {
		return nil, err
	}
	return service.mapListing(ctx, *listing)
}

func (service *EconomyService) CreateTrade(ctx context.Context, senderID int64, model resources.CreateTradeModel) (*resources.TradeOffer, error) {
	if _, err := service.ensureStarter(ctx, senderID); err != nil {
		return nil, err
	}
	offered, err := parseStringIDs(model.OfferedCardIDs)
	if err != nil {
		return nil, ErrInvalidInput
	}
	requested, err := parseStringIDs(model.RequestedCardIDs)
	if err != nil {
		return nil, ErrInvalidInput
	}
	offer, err := service.repository.CreateTrade(ctx, senderID, model.ReceiverID, offered, requested, model.OfferedCoins, model.RequestedCoins, model.CommandID)
	if err != nil {
		return nil, err
	}
	result := tradeResource(*offer)
	return &result, nil
}

func (service *EconomyService) Trades(ctx context.Context, userID int64) ([]resources.TradeOffer, error) {
	if _, err := service.repository.ExpireTrades(ctx, lazyEconomySweepLimit); err != nil {
		return nil, err
	}
	offers, err := service.repository.ListTrades(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]resources.TradeOffer, 0, len(offers))
	for _, offer := range offers {
		result = append(result, tradeResource(offer))
	}
	return result, nil
}

func (service *EconomyService) AcceptTrade(ctx context.Context, userID, tradeID int64, commandID string) (*resources.TradeOffer, error) {
	if _, err := service.ensureStarter(ctx, userID); err != nil {
		return nil, err
	}
	if _, err := service.repository.ExpireTrades(ctx, lazyEconomySweepLimit); err != nil {
		return nil, err
	}
	offer, err := service.repository.AcceptTrade(ctx, userID, tradeID, commandID)
	if err != nil {
		return nil, err
	}
	result := tradeResource(*offer)
	return &result, nil
}

// ensureStarter treats the wallet as the durable initialization marker. The
// repository creates the wallet, grant ledger entry, and starter cards in one
// transaction, so a visible wallet means the complete starter economy committed.
func (service *EconomyService) ensureStarter(ctx context.Context, userID int64) (*economy.Wallet, error) {
	if userID <= 0 {
		return nil, ErrForbidden
	}
	wallet, err := service.repository.GetWallet(ctx, userID)
	if err == nil {
		return wallet, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	starters, err := service.questionBank.StarterQuestions(ctx, userID, economy.StarterCards)
	if err != nil {
		return nil, err
	}
	if err := service.repository.EnsureStarter(ctx, userID, starters); err != nil {
		return nil, err
	}
	return service.repository.GetWallet(ctx, userID)
}

func (service *EconomyService) CloseTrade(ctx context.Context, userID, tradeID int64, action, commandID string) (*resources.TradeOffer, error) {
	offer, err := service.repository.CloseTrade(ctx, userID, tradeID, action, commandID)
	if err != nil {
		return nil, err
	}
	result := tradeResource(*offer)
	return &result, nil
}

func (service *EconomyService) mapCollection(ctx context.Context, wallet *economy.Wallet, cards []economy.Card) (*resources.Collection, error) {
	questionIDs := make([]string, 0, len(cards))
	for _, card := range cards {
		questionIDs = append(questionIDs, card.QuestionID)
	}
	questions, err := service.questionBank.GetMany(ctx, questionIDs)
	if err != nil {
		return nil, err
	}
	result := &resources.Collection{
		Wallet: resources.Wallet{Balance: wallet.Balance, Locked: wallet.Locked, Available: wallet.Balance - wallet.Locked},
		Cards:  make([]resources.CollectibleCard, 0, len(cards)),
	}
	for _, card := range cards {
		item, exists := questions[card.QuestionID]
		if !exists {
			continue
		}
		result.Cards = append(result.Cards, cardResource(card, item))
	}
	return result, nil
}

func (service *EconomyService) mapListings(ctx context.Context, listings []economy.Listing) ([]resources.MarketListing, error) {
	cardIDs := make([]int64, 0, len(listings))
	for _, listing := range listings {
		cardIDs = append(cardIDs, listing.CardID)
	}
	cards, err := service.repository.GetCardsByIDs(ctx, cardIDs)
	if err != nil {
		return nil, err
	}
	questionIDs := make([]string, 0, len(cards))
	for _, card := range cards {
		questionIDs = append(questionIDs, card.QuestionID)
	}
	questions, err := service.questionBank.GetMany(ctx, questionIDs)
	if err != nil {
		return nil, err
	}
	result := make([]resources.MarketListing, 0, len(listings))
	for _, listing := range listings {
		card, exists := cards[listing.CardID]
		if !exists {
			continue
		}
		item, exists := questions[card.QuestionID]
		if !exists {
			continue
		}
		result = append(result, listingResource(listing, cardResource(card, item)))
	}
	return result, nil
}

func (service *EconomyService) mapListing(ctx context.Context, listing economy.Listing) (*resources.MarketListing, error) {
	listings, err := service.mapListings(ctx, []economy.Listing{listing})
	if err != nil {
		return nil, err
	}
	if len(listings) != 1 {
		return nil, repository.ErrNotFound
	}
	return &listings[0], nil
}

func cardResource(card economy.Card, item question.Question) resources.CollectibleCard {
	return resources.CollectibleCard{
		ID: card.ID, QuestionID: card.QuestionID, Prompt: item.Prompt,
		Category: item.Category, Difficulty: string(item.Difficulty), SourceTitle: item.Source.Title,
		Rarity: card.Rarity, Power: card.Power, Plays: card.Plays, Wins: card.Wins,
		Status: string(card.Status), LockRef: card.LockRef, CreatedAt: card.CreatedAt,
	}
}

func listingResource(listing economy.Listing, card resources.CollectibleCard) resources.MarketListing {
	return resources.MarketListing{
		ID: listing.ID, SellerID: listing.SellerID, BuyerID: listing.BuyerID,
		Price: listing.Price, Fee: listing.Fee, Status: string(listing.Status), Card: card,
		CreatedAt: listing.CreatedAt, ExpiresAt: listing.ExpiresAt,
	}
}

func tradeResource(offer economy.TradeOffer) resources.TradeOffer {
	return resources.TradeOffer{
		ID: offer.ID, SenderID: offer.SenderID, ReceiverID: offer.ReceiverID,
		OfferedCardIDs: stringifyIDs(offer.OfferedCardIDs), RequestedCardIDs: stringifyIDs(offer.RequestedCardIDs),
		OfferedCoins: offer.OfferedCoins, RequestedCoins: offer.RequestedCoins,
		Status: string(offer.Status), CreatedAt: offer.CreatedAt, ExpiresAt: offer.ExpiresAt,
	}
}

func parseStringIDs(values []string) ([]int64, error) {
	result := make([]int64, len(values))
	for index, value := range values {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid ID")
		}
		result[index] = id
	}
	return result, nil
}

func ParsePublicIDs(values []string) ([]int64, error) {
	return parseStringIDs(values)
}

func stringifyIDs(values []int64) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = strconv.FormatInt(value, 10)
	}
	return result
}
