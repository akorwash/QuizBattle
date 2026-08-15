package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/akorwash/QuizBattle/domain/economy"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (repository *MongoEconomyRepository) CreateTrade(
	ctx context.Context,
	senderID, receiverID int64,
	offeredCardIDs, requestedCardIDs []int64,
	offeredCoins, requestedCoins int64,
	commandID string,
) (*economy.TradeOffer, error) {
	tradeID, err := NewID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	offer := &economy.TradeOffer{
		ID: tradeID, SenderID: senderID, ReceiverID: receiverID,
		OfferedCardIDs: append([]int64(nil), offeredCardIDs...), RequestedCardIDs: append([]int64(nil), requestedCardIDs...),
		OfferedCoins: offeredCoins, RequestedCoins: requestedCoins, Status: economy.TradePending,
		CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(24 * time.Hour), Version: 1, CommandID: commandID,
	}
	if !validIdempotencyKey(commandID) || economy.ValidateTrade(offer) != nil {
		return nil, economy.ErrInvalidTrade
	}
	var output *economy.TradeOffer
	err = repository.withTransaction(ctx, func(tx context.Context) error {
		var existing economy.TradeOffer
		if err := repository.trades.FindOne(tx, bson.M{"commandId": commandID}).Decode(&existing); err == nil {
			if !sameTradeRequest(existing, *offer) {
				return ErrConflict
			}
			output = &existing
			return nil
		} else if !errors.Is(err, mongo.ErrNoDocuments) {
			return fmt.Errorf("find trade command: %w", err)
		}

		if err := repository.validateTradeCards(tx, senderID, receiverID, offeredCardIDs, requestedCardIDs, tradeID); err != nil {
			return err
		}
		if offeredCoins > 0 {
			var wallet economy.Wallet
			if err := repository.wallets.FindOne(tx, bson.M{"userId": senderID}).Decode(&wallet); err != nil {
				return fmt.Errorf("find trade sender wallet: %w", err)
			}
			if wallet.Balance-wallet.Locked < offeredCoins {
				return economy.ErrInsufficientCoins
			}
			result, err := repository.wallets.UpdateOne(
				tx,
				bson.M{"userId": senderID, "version": wallet.Version, "$expr": bson.M{"$gte": bson.A{bson.M{"$subtract": bson.A{"$balance", "$locked"}}, offeredCoins}}},
				bson.M{"$inc": bson.M{"locked": offeredCoins, "version": 1}, "$set": bson.M{"updatedAt": now}},
			)
			if err != nil {
				return fmt.Errorf("lock trade coins: %w", err)
			}
			if err := matched(result); err != nil {
				return economy.ErrInsufficientCoins
			}
		}
		if len(offeredCardIDs) > 0 {
			result, err := repository.cards.UpdateMany(
				tx,
				bson.M{"id": bson.M{"$in": offeredCardIDs}, "ownerId": senderID, "status": economy.CardAvailable},
				bson.M{"$set": bson.M{"status": economy.CardTradeEscrow, "lockRef": fmt.Sprintf("trade:%d", tradeID), "updatedAt": now}, "$inc": bson.M{"version": 1}},
			)
			if err != nil {
				return fmt.Errorf("escrow trade cards: %w", err)
			}
			if result.ModifiedCount != int64(len(offeredCardIDs)) {
				return economy.ErrCardUnavailable
			}
		}
		if _, err := repository.trades.InsertOne(tx, offer); err != nil {
			return fmt.Errorf("insert trade: %w", err)
		}
		output = offer
		return nil
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}

// ExpireTrades releases sender-side coin and card escrow for a bounded number
// of elapsed offers. Requested assets were never escrowed and need no mutation.
func (repository *MongoEconomyRepository) ExpireTrades(ctx context.Context, limit int64) (int64, error) {
	limit = boundedSweepLimit(limit)
	now := time.Now().UTC()
	cursor, err := repository.trades.Find(
		ctx,
		bson.M{"status": economy.TradePending, "expiresAt": bson.M{"$lte": now}},
		options.Find().SetSort(bson.D{{Key: "expiresAt", Value: 1}, {Key: "id", Value: 1}}).SetLimit(limit).SetProjection(bson.M{"id": 1}),
	)
	if err != nil {
		return 0, fmt.Errorf("find expired trades: %w", err)
	}
	defer cursor.Close(ctx)
	var candidates []struct {
		ID int64 `bson:"id"`
	}
	if err := cursor.All(ctx, &candidates); err != nil {
		return 0, fmt.Errorf("decode expired trades: %w", err)
	}

	var expired int64
	for _, candidate := range candidates {
		changed, err := repository.expireTrade(ctx, candidate.ID, now)
		if err != nil {
			return expired, err
		}
		if changed {
			expired++
		}
	}
	return expired, nil
}

func (repository *MongoEconomyRepository) expireTrade(ctx context.Context, tradeID int64, now time.Time) (bool, error) {
	changed := false
	err := repository.withTransaction(ctx, func(tx context.Context) error {
		changed = false
		var offer economy.TradeOffer
		if err := repository.trades.FindOne(tx, bson.M{"id": tradeID}).Decode(&offer); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil
			}
			return fmt.Errorf("find trade expiry: %w", err)
		}
		if offer.Status != economy.TradePending || now.Before(offer.ExpiresAt) {
			return nil
		}

		if offer.OfferedCoins > 0 {
			result, err := repository.wallets.UpdateOne(
				tx,
				bson.M{"userId": offer.SenderID, "locked": bson.M{"$gte": offer.OfferedCoins}},
				bson.M{"$inc": bson.M{"locked": -offer.OfferedCoins, "version": 1}, "$set": bson.M{"updatedAt": now}},
			)
			if err != nil {
				return fmt.Errorf("release expired trade coins: %w", err)
			}
			if err := matchedAndModified(result, 1); err != nil {
				return economy.ErrInvalidEconomyState
			}
		}

		if len(offer.OfferedCardIDs) > 0 {
			result, err := repository.cards.UpdateMany(
				tx,
				bson.M{"id": bson.M{"$in": offer.OfferedCardIDs}, "ownerId": offer.SenderID, "status": economy.CardTradeEscrow, "lockRef": fmt.Sprintf("trade:%d", offer.ID)},
				bson.M{"$set": bson.M{"status": economy.CardAvailable, "lockRef": "", "updatedAt": now}, "$inc": bson.M{"version": 1}},
			)
			if err != nil {
				return fmt.Errorf("release expired trade cards: %w", err)
			}
			if err := matchedAndModified(result, int64(len(offer.OfferedCardIDs))); err != nil {
				return economy.ErrInvalidEconomyState
			}
		}

		oldVersion := offer.Version
		offer.Status = economy.TradeExpired
		offer.SettlementCommandID = fmt.Sprintf("expire:%d", offer.ID)
		offer.UpdatedAt = now
		offer.Version++
		result, err := repository.trades.ReplaceOne(
			tx,
			bson.M{"id": offer.ID, "version": oldVersion, "status": economy.TradePending},
			offer,
		)
		if err != nil {
			return fmt.Errorf("mark trade expired: %w", err)
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

func (repository *MongoEconomyRepository) ListTrades(ctx context.Context, userID int64) ([]economy.TradeOffer, error) {
	cursor, err := repository.trades.Find(
		ctx,
		bson.M{"$or": bson.A{bson.M{"senderId": userID}, bson.M{"receiverId": userID}}},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(100),
	)
	if err != nil {
		return nil, fmt.Errorf("list trades: %w", err)
	}
	defer cursor.Close(ctx)
	result := make([]economy.TradeOffer, 0)
	if err := cursor.All(ctx, &result); err != nil {
		return nil, fmt.Errorf("decode trades: %w", err)
	}
	return result, nil
}

func (repository *MongoEconomyRepository) AcceptTrade(ctx context.Context, receiverID, tradeID int64, commandID string) (*economy.TradeOffer, error) {
	if receiverID <= 0 || tradeID <= 0 || !validIdempotencyKey(commandID) {
		return nil, economy.ErrInvalidTrade
	}
	ledgerIDs, err := newIDs(24)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var output *economy.TradeOffer
	err = repository.withTransaction(ctx, func(tx context.Context) error {
		var offer economy.TradeOffer
		if err := repository.trades.FindOne(tx, bson.M{"id": tradeID}).Decode(&offer); err != nil {
			return ErrNotFound
		}
		if offer.Status == economy.TradeAccepted && offer.ReceiverID == receiverID && offer.SettlementCommandID == commandID {
			output = &offer
			return nil
		}
		if offer.Status != economy.TradePending || offer.ReceiverID != receiverID || !now.Before(offer.ExpiresAt) {
			return economy.ErrInvalidTrade
		}
		oldOfferVersion := offer.Version
		var senderWallet, receiverWallet economy.Wallet
		if err := repository.wallets.FindOne(tx, bson.M{"userId": offer.SenderID}).Decode(&senderWallet); err != nil {
			return fmt.Errorf("find sender wallet: %w", err)
		}
		if err := repository.wallets.FindOne(tx, bson.M{"userId": receiverID}).Decode(&receiverWallet); err != nil {
			return fmt.Errorf("find receiver wallet: %w", err)
		}
		oldSenderVersion, oldReceiverVersion := senderWallet.Version, receiverWallet.Version
		if senderWallet.Locked < offer.OfferedCoins || senderWallet.Balance < offer.OfferedCoins || receiverWallet.Balance-receiverWallet.Locked < offer.RequestedCoins {
			return economy.ErrInsufficientCoins
		}

		offeredCards, err := repository.tradeCards(tx, offer.OfferedCardIDs)
		if err != nil {
			return err
		}
		requestedCards, err := repository.tradeCards(tx, offer.RequestedCardIDs)
		if err != nil {
			return err
		}
		lockRef := fmt.Sprintf("trade:%d", offer.ID)
		for _, card := range offeredCards {
			if card.OwnerID != offer.SenderID || card.Status != economy.CardTradeEscrow || card.LockRef != lockRef {
				return economy.ErrCardUnavailable
			}
		}
		for _, card := range requestedCards {
			if card.OwnerID != receiverID || card.Status != economy.CardAvailable || card.LockRef != "" {
				return economy.ErrCardUnavailable
			}
		}

		senderWallet.Locked -= offer.OfferedCoins
		senderWallet.Balance = senderWallet.Balance - offer.OfferedCoins + offer.RequestedCoins
		receiverWallet.Balance = receiverWallet.Balance - offer.RequestedCoins + offer.OfferedCoins
		senderWallet.Version++
		receiverWallet.Version++
		senderWallet.UpdatedAt, receiverWallet.UpdatedAt = now, now
		if result, err := repository.wallets.ReplaceOne(tx, bson.M{"userId": senderWallet.UserID, "version": oldSenderVersion}, senderWallet); err != nil {
			return err
		} else if err := matched(result); err != nil {
			return err
		}
		if result, err := repository.wallets.ReplaceOne(tx, bson.M{"userId": receiverWallet.UserID, "version": oldReceiverVersion}, receiverWallet); err != nil {
			return err
		} else if err := matched(result); err != nil {
			return err
		}

		entries := make([]any, 0, 24)
		ledgerIndex := 0
		if delta := -offer.OfferedCoins + offer.RequestedCoins; delta != 0 {
			entries = append(entries, economy.LedgerEntry{ID: ledgerIDs[ledgerIndex], UserID: offer.SenderID, CoinDelta: delta, BalanceAfter: senderWallet.Balance, Reason: "trade_settlement", ReferenceType: "trade", ReferenceID: fmt.Sprint(offer.ID), IdempotencyKey: commandID, EntryPart: "sender_coins", CreatedAt: now})
			ledgerIndex++
		}
		if delta := -offer.RequestedCoins + offer.OfferedCoins; delta != 0 {
			entries = append(entries, economy.LedgerEntry{ID: ledgerIDs[ledgerIndex], UserID: receiverID, CoinDelta: delta, BalanceAfter: receiverWallet.Balance, Reason: "trade_settlement", ReferenceType: "trade", ReferenceID: fmt.Sprint(offer.ID), IdempotencyKey: commandID, EntryPart: "receiver_coins", CreatedAt: now})
			ledgerIndex++
		}
		for _, card := range offeredCards {
			oldVersion := card.Version
			card.OwnerID, card.Status, card.LockRef, card.UpdatedAt, card.Version = receiverID, economy.CardAvailable, "", now, card.Version+1
			if result, err := repository.cards.ReplaceOne(tx, bson.M{"id": card.ID, "version": oldVersion}, card); err != nil {
				return err
			} else if err := matched(result); err != nil {
				return err
			}
			entries = append(entries, economy.LedgerEntry{ID: ledgerIDs[ledgerIndex], UserID: receiverID, CardID: card.ID, PreviousOwner: offer.SenderID, NewOwner: receiverID, Reason: "trade_received", ReferenceType: "trade", ReferenceID: fmt.Sprint(offer.ID), IdempotencyKey: commandID, EntryPart: fmt.Sprintf("offered_card_%d", card.ID), CreatedAt: now})
			ledgerIndex++
		}
		for _, card := range requestedCards {
			oldVersion := card.Version
			card.OwnerID, card.Status, card.LockRef, card.UpdatedAt, card.Version = offer.SenderID, economy.CardAvailable, "", now, card.Version+1
			if result, err := repository.cards.ReplaceOne(tx, bson.M{"id": card.ID, "version": oldVersion}, card); err != nil {
				return err
			} else if err := matched(result); err != nil {
				return err
			}
			entries = append(entries, economy.LedgerEntry{ID: ledgerIDs[ledgerIndex], UserID: offer.SenderID, CardID: card.ID, PreviousOwner: receiverID, NewOwner: offer.SenderID, Reason: "trade_received", ReferenceType: "trade", ReferenceID: fmt.Sprint(offer.ID), IdempotencyKey: commandID, EntryPart: fmt.Sprintf("requested_card_%d", card.ID), CreatedAt: now})
			ledgerIndex++
		}
		if len(entries) > 0 {
			if _, err := repository.ledger.InsertMany(tx, entries); err != nil {
				return fmt.Errorf("record trade ledger: %w", err)
			}
		}
		offer.Status = economy.TradeAccepted
		offer.SettlementCommandID = commandID
		offer.UpdatedAt = now
		offer.Version++
		if result, err := repository.trades.ReplaceOne(tx, bson.M{"id": offer.ID, "version": oldOfferVersion}, offer); err != nil {
			return err
		} else if err := matched(result); err != nil {
			return err
		}
		output = &offer
		return nil
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}

func (repository *MongoEconomyRepository) CloseTrade(ctx context.Context, actorID, tradeID int64, action, commandID string) (*economy.TradeOffer, error) {
	if actorID <= 0 || tradeID <= 0 || !validIdempotencyKey(commandID) || (action != "reject" && action != "cancel") {
		return nil, economy.ErrInvalidTrade
	}
	now := time.Now().UTC()
	var output *economy.TradeOffer
	err := repository.withTransaction(ctx, func(tx context.Context) error {
		var offer economy.TradeOffer
		if err := repository.trades.FindOne(tx, bson.M{"id": tradeID}).Decode(&offer); err != nil {
			return ErrNotFound
		}
		targetStatus := economy.TradeRejected
		if action == "cancel" {
			targetStatus = economy.TradeCancelled
		}
		if offer.Status == targetStatus && offer.SettlementCommandID == commandID {
			output = &offer
			return nil
		}
		if offer.Status != economy.TradePending || (action == "reject" && actorID != offer.ReceiverID) || (action == "cancel" && actorID != offer.SenderID) {
			return economy.ErrInvalidTrade
		}
		oldVersion := offer.Version
		if offer.OfferedCoins > 0 {
			result, err := repository.wallets.UpdateOne(tx, bson.M{"userId": offer.SenderID, "locked": bson.M{"$gte": offer.OfferedCoins}}, bson.M{"$inc": bson.M{"locked": -offer.OfferedCoins, "version": 1}, "$set": bson.M{"updatedAt": now}})
			if err != nil {
				return err
			}
			if err := matched(result); err != nil {
				return economy.ErrInvalidEconomyState
			}
		}
		if len(offer.OfferedCardIDs) > 0 {
			result, err := repository.cards.UpdateMany(
				tx,
				bson.M{"id": bson.M{"$in": offer.OfferedCardIDs}, "ownerId": offer.SenderID, "status": economy.CardTradeEscrow, "lockRef": fmt.Sprintf("trade:%d", offer.ID)},
				bson.M{"$set": bson.M{"status": economy.CardAvailable, "lockRef": "", "updatedAt": now}, "$inc": bson.M{"version": 1}},
			)
			if err != nil {
				return err
			}
			if result.ModifiedCount != int64(len(offer.OfferedCardIDs)) {
				return economy.ErrInvalidEconomyState
			}
		}
		offer.Status = targetStatus
		offer.SettlementCommandID = commandID
		offer.UpdatedAt = now
		offer.Version++
		if result, err := repository.trades.ReplaceOne(tx, bson.M{"id": offer.ID, "version": oldVersion}, offer); err != nil {
			return err
		} else if err := matched(result); err != nil {
			return err
		}
		output = &offer
		return nil
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}

func (repository *MongoEconomyRepository) validateTradeCards(ctx context.Context, senderID, receiverID int64, offered, requested []int64, tradeID int64) error {
	if len(offered) > 0 {
		cards, err := repository.tradeCards(ctx, offered)
		if err != nil {
			return err
		}
		for _, card := range cards {
			if card.OwnerID != senderID || card.Status != economy.CardAvailable || card.LockRef != "" {
				return economy.ErrCardUnavailable
			}
		}
	}
	if len(requested) > 0 {
		cards, err := repository.tradeCards(ctx, requested)
		if err != nil {
			return err
		}
		for _, card := range cards {
			if card.OwnerID != receiverID || card.Status != economy.CardAvailable || card.LockRef != "" {
				return economy.ErrCardUnavailable
			}
		}
	}
	_ = tradeID
	return nil
}

func (repository *MongoEconomyRepository) tradeCards(ctx context.Context, ids []int64) ([]economy.Card, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	cursor, err := repository.cards.Find(ctx, bson.M{"id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var cards []economy.Card
	if err := cursor.All(ctx, &cards); err != nil {
		return nil, err
	}
	if len(cards) != len(ids) {
		return nil, economy.ErrCardUnavailable
	}
	return cards, nil
}

func sameTradeRequest(existing, requested economy.TradeOffer) bool {
	return existing.SenderID == requested.SenderID &&
		existing.ReceiverID == requested.ReceiverID &&
		existing.OfferedCoins == requested.OfferedCoins &&
		existing.RequestedCoins == requested.RequestedCoins &&
		slices.Equal(existing.OfferedCardIDs, requested.OfferedCardIDs) &&
		slices.Equal(existing.RequestedCardIDs, requested.RequestedCardIDs)
}
