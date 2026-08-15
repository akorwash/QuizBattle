package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/akorwash/QuizBattle/domain/economy"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (repository *MongoEconomyRepository) CreateListing(ctx context.Context, sellerID, cardID, price int64, commandID string) (*economy.Listing, error) {
	if sellerID <= 0 || cardID <= 0 || !validIdempotencyKey(commandID) {
		return nil, economy.ErrInvalidEconomyState
	}
	listingID, err := NewID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var output *economy.Listing
	err = repository.withTransaction(ctx, func(tx context.Context) error {
		var existing economy.Listing
		if err := repository.listings.FindOne(tx, bson.M{"commandId": commandID}).Decode(&existing); err == nil {
			if existing.SellerID != sellerID || existing.CardID != cardID || existing.Price != price {
				return ErrConflict
			}
			output = &existing
			return nil
		} else if !errors.Is(err, mongo.ErrNoDocuments) {
			return fmt.Errorf("find listing command: %w", err)
		}

		var card economy.Card
		if err := repository.cards.FindOne(tx, bson.M{"id": cardID}).Decode(&card); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return ErrNotFound
			}
			return fmt.Errorf("find listing card: %w", err)
		}
		oldVersion := card.Version
		listing, err := economy.NewListing(listingID, sellerID, &card, price, now)
		if err != nil {
			return err
		}
		listing.CommandID = commandID
		if _, err := repository.listings.InsertOne(tx, listing); err != nil {
			return fmt.Errorf("insert listing: %w", err)
		}
		result, err := repository.cards.ReplaceOne(tx, bson.M{"id": card.ID, "version": oldVersion}, card)
		if err != nil {
			return fmt.Errorf("escrow listed card: %w", err)
		}
		if err := matched(result); err != nil {
			return err
		}
		output = listing
		return nil
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}

// ExpireListings releases a bounded number of elapsed listing escrows. Each
// listing/card pair is settled in its own transaction so a concurrent purchase
// or cancellation can win through normal CAS without rolling back unrelated
// expiry work.
func (repository *MongoEconomyRepository) ExpireListings(ctx context.Context, limit int64) (int64, error) {
	limit = boundedSweepLimit(limit)
	now := time.Now().UTC()
	cursor, err := repository.listings.Find(
		ctx,
		bson.M{"status": economy.ListingActive, "expiresAt": bson.M{"$lte": now}},
		options.Find().SetSort(bson.D{{Key: "expiresAt", Value: 1}, {Key: "id", Value: 1}}).SetLimit(limit).SetProjection(bson.M{"id": 1}),
	)
	if err != nil {
		return 0, fmt.Errorf("find expired listings: %w", err)
	}
	defer cursor.Close(ctx)
	var candidates []struct {
		ID int64 `bson:"id"`
	}
	if err := cursor.All(ctx, &candidates); err != nil {
		return 0, fmt.Errorf("decode expired listings: %w", err)
	}

	var expired int64
	for _, candidate := range candidates {
		changed, err := repository.expireListing(ctx, candidate.ID, now)
		if err != nil {
			return expired, err
		}
		if changed {
			expired++
		}
	}
	return expired, nil
}

func (repository *MongoEconomyRepository) expireListing(ctx context.Context, listingID int64, now time.Time) (bool, error) {
	changed := false
	err := repository.withTransaction(ctx, func(tx context.Context) error {
		changed = false
		var listing economy.Listing
		if err := repository.listings.FindOne(tx, bson.M{"id": listingID}).Decode(&listing); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil
			}
			return fmt.Errorf("find listing expiry: %w", err)
		}
		if listing.Status != economy.ListingActive || now.Before(listing.ExpiresAt) {
			return nil
		}

		var card economy.Card
		if err := repository.cards.FindOne(tx, bson.M{"id": listing.CardID}).Decode(&card); err != nil {
			return fmt.Errorf("find expired listing card: %w", err)
		}
		lockRef := fmt.Sprintf("listing:%d", listing.ID)
		if card.OwnerID != listing.SellerID || card.Status != economy.CardMarketEscrow || card.LockRef != lockRef {
			return economy.ErrInvalidEconomyState
		}

		oldCardVersion, oldListingVersion := card.Version, listing.Version
		card.Status = economy.CardAvailable
		card.LockRef = ""
		card.UpdatedAt = now
		card.Version++
		listing.Status = economy.ListingExpired
		listing.UpdatedAt = now
		listing.Version++

		result, err := repository.cards.ReplaceOne(tx, bson.M{"id": card.ID, "version": oldCardVersion}, card)
		if err != nil {
			return fmt.Errorf("release expired listing card: %w", err)
		}
		if err := matched(result); err != nil {
			return err
		}
		result, err = repository.listings.ReplaceOne(tx, bson.M{"id": listing.ID, "version": oldListingVersion, "status": economy.ListingActive}, listing)
		if err != nil {
			return fmt.Errorf("mark listing expired: %w", err)
		}
		if err := matched(result); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

func (repository *MongoEconomyRepository) ListActiveListings(ctx context.Context, limit int64) ([]economy.Listing, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	now := time.Now().UTC()
	cursor, err := repository.listings.Find(
		ctx,
		bson.M{"status": economy.ListingActive, "expiresAt": bson.M{"$gt": now}},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(limit),
	)
	if err != nil {
		return nil, fmt.Errorf("list market listings: %w", err)
	}
	defer cursor.Close(ctx)
	result := make([]economy.Listing, 0)
	if err := cursor.All(ctx, &result); err != nil {
		return nil, fmt.Errorf("decode market listings: %w", err)
	}
	return result, nil
}

func (repository *MongoEconomyRepository) BuyListing(ctx context.Context, buyerID, listingID int64, commandID string) (*economy.Listing, error) {
	if buyerID <= 0 || listingID <= 0 || !validIdempotencyKey(commandID) {
		return nil, economy.ErrInvalidEconomyState
	}
	ledgerIDs, err := newIDs(4)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var output *economy.Listing
	err = repository.withTransaction(ctx, func(tx context.Context) error {
		var listing economy.Listing
		if err := repository.listings.FindOne(tx, bson.M{"id": listingID}).Decode(&listing); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return ErrNotFound
			}
			return fmt.Errorf("find listing: %w", err)
		}
		if listing.Status == economy.ListingSold && listing.BuyerID == buyerID && listing.SettlementCommandID == commandID {
			output = &listing
			return nil
		}
		oldListingVersion := listing.Version
		var card economy.Card
		if err := repository.cards.FindOne(tx, bson.M{"id": listing.CardID}).Decode(&card); err != nil {
			return fmt.Errorf("find listed card: %w", err)
		}
		oldCardVersion := card.Version
		var buyer, seller economy.Wallet
		if err := repository.wallets.FindOne(tx, bson.M{"userId": buyerID}).Decode(&buyer); err != nil {
			return fmt.Errorf("find buyer wallet: %w", err)
		}
		if err := repository.wallets.FindOne(tx, bson.M{"userId": listing.SellerID}).Decode(&seller); err != nil {
			return fmt.Errorf("find seller wallet: %w", err)
		}
		oldBuyerVersion, oldSellerVersion := buyer.Version, seller.Version
		fee, err := economy.SettlePurchase(&listing, &card, &buyer, &seller, buyerID, now)
		if err != nil {
			return err
		}
		listing.SettlementCommandID = commandID
		if result, err := repository.cards.ReplaceOne(tx, bson.M{"id": card.ID, "version": oldCardVersion}, card); err != nil {
			return err
		} else if err := matched(result); err != nil {
			return err
		}
		if result, err := repository.wallets.ReplaceOne(tx, bson.M{"userId": buyerID, "version": oldBuyerVersion}, buyer); err != nil {
			return err
		} else if err := matched(result); err != nil {
			return err
		}
		if result, err := repository.wallets.ReplaceOne(tx, bson.M{"userId": seller.UserID, "version": oldSellerVersion}, seller); err != nil {
			return err
		} else if err := matched(result); err != nil {
			return err
		}
		if result, err := repository.listings.ReplaceOne(tx, bson.M{"id": listing.ID, "version": oldListingVersion}, listing); err != nil {
			return err
		} else if err := matched(result); err != nil {
			return err
		}
		entries := []any{
			economy.LedgerEntry{ID: ledgerIDs[0], UserID: buyerID, CoinDelta: -listing.Price, BalanceAfter: buyer.Balance, Reason: "market_purchase", ReferenceType: "listing", ReferenceID: fmt.Sprint(listing.ID), IdempotencyKey: commandID, EntryPart: "buyer_coins", CreatedAt: now},
			economy.LedgerEntry{ID: ledgerIDs[1], UserID: seller.UserID, CoinDelta: listing.Price - fee, BalanceAfter: seller.Balance, Reason: "market_sale", ReferenceType: "listing", ReferenceID: fmt.Sprint(listing.ID), IdempotencyKey: commandID, EntryPart: "seller_coins", CreatedAt: now},
			economy.LedgerEntry{ID: ledgerIDs[2], UserID: buyerID, CardID: card.ID, PreviousOwner: seller.UserID, NewOwner: buyerID, Reason: "market_purchase", ReferenceType: "listing", ReferenceID: fmt.Sprint(listing.ID), IdempotencyKey: commandID, EntryPart: "buyer_card", CreatedAt: now},
			economy.LedgerEntry{ID: ledgerIDs[3], UserID: seller.UserID, CardID: card.ID, PreviousOwner: seller.UserID, NewOwner: buyerID, Reason: "market_sale", ReferenceType: "listing", ReferenceID: fmt.Sprint(listing.ID), IdempotencyKey: commandID, EntryPart: "seller_card", CreatedAt: now},
		}
		if _, err := repository.ledger.InsertMany(tx, entries); err != nil {
			return fmt.Errorf("record market ledger: %w", err)
		}
		output = &listing
		return nil
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}

func (repository *MongoEconomyRepository) CancelListing(ctx context.Context, sellerID, listingID int64, commandID string) (*economy.Listing, error) {
	if sellerID <= 0 || listingID <= 0 || !validIdempotencyKey(commandID) {
		return nil, economy.ErrInvalidEconomyState
	}
	now := time.Now().UTC()
	var output *economy.Listing
	err := repository.withTransaction(ctx, func(tx context.Context) error {
		var listing economy.Listing
		if err := repository.listings.FindOne(tx, bson.M{"id": listingID}).Decode(&listing); err != nil {
			return ErrNotFound
		}
		if listing.Status == economy.ListingCancelled && listing.CancelCommandID == commandID {
			output = &listing
			return nil
		}
		oldListingVersion := listing.Version
		var card economy.Card
		if err := repository.cards.FindOne(tx, bson.M{"id": listing.CardID}).Decode(&card); err != nil {
			return fmt.Errorf("find listing card: %w", err)
		}
		oldCardVersion := card.Version
		if err := economy.CancelListing(&listing, &card, sellerID, now); err != nil {
			return err
		}
		listing.CancelCommandID = commandID
		if result, err := repository.cards.ReplaceOne(tx, bson.M{"id": card.ID, "version": oldCardVersion}, card); err != nil {
			return err
		} else if err := matched(result); err != nil {
			return err
		}
		if result, err := repository.listings.ReplaceOne(tx, bson.M{"id": listing.ID, "version": oldListingVersion}, listing); err != nil {
			return err
		} else if err := matched(result); err != nil {
			return err
		}
		output = &listing
		return nil
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}

func newIDs(count int) ([]int64, error) {
	result := make([]int64, count)
	for index := range result {
		id, err := NewID()
		if err != nil {
			return nil, err
		}
		result[index] = id
	}
	return result, nil
}

func validIdempotencyKey(value string) bool {
	if len(value) < 8 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' || character == ':' {
			continue
		}
		return false
	}
	return true
}

func boundedSweepLimit(limit int64) int64 {
	if limit <= 0 || limit > 100 {
		return 25
	}
	return limit
}
