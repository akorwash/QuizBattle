package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/akorwash/QuizBattle/domain/economy"
	matchdomain "github.com/akorwash/QuizBattle/domain/match"
	"github.com/akorwash/QuizBattle/domain/question"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

const (
	walletCollection  = "Wallets"
	cardCollection    = "Cards"
	ledgerCollection  = "EconomyLedger"
	listingCollection = "MarketListings"
	tradeCollection   = "TradeOffers"
)

type MongoEconomyRepository struct {
	database *mongo.Database
	wallets  *mongo.Collection
	cards    *mongo.Collection
	ledger   *mongo.Collection
	listings *mongo.Collection
	trades   *mongo.Collection
}

func NewMongoEconomyRepository(database *mongo.Database) *MongoEconomyRepository {
	return &MongoEconomyRepository{
		database: database,
		wallets:  database.Collection(walletCollection), cards: database.Collection(cardCollection),
		ledger: database.Collection(ledgerCollection), listings: database.Collection(listingCollection),
		trades: database.Collection(tradeCollection),
	}
}

func (repository *MongoEconomyRepository) EnsureStarter(ctx context.Context, userID int64, questions []question.Question) error {
	if userID <= 0 || len(questions) != economy.StarterCards {
		return economy.ErrInvalidEconomyState
	}
	cardIDs := make([]int64, len(questions))
	for index := range cardIDs {
		id, err := NewID()
		if err != nil {
			return err
		}
		cardIDs[index] = id
	}
	ledgerID, err := NewID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return repository.withTransaction(ctx, func(tx context.Context) error {
		var wallet economy.Wallet
		findError := repository.wallets.FindOne(tx, bson.M{"userId": userID}).Decode(&wallet)
		if findError != nil && !errors.Is(findError, mongo.ErrNoDocuments) {
			return fmt.Errorf("find starter wallet: %w", findError)
		}
		walletCreated := errors.Is(findError, mongo.ErrNoDocuments)
		if walletCreated {
			wallet = economy.Wallet{UserID: userID, Balance: economy.StarterBalance, Version: 1, CreatedAt: now, UpdatedAt: now}
			if _, err := repository.wallets.InsertOne(tx, wallet); err != nil {
				return fmt.Errorf("create starter wallet: %w", err)
			}
			entry := economy.LedgerEntry{
				ID: ledgerID, UserID: userID, CoinDelta: economy.StarterBalance,
				BalanceAfter: economy.StarterBalance, Reason: "starter_grant",
				ReferenceType: "account", ReferenceID: fmt.Sprint(userID),
				IdempotencyKey: fmt.Sprintf("starter:%d", userID), EntryPart: "coins", CreatedAt: now,
			}
			if _, err := repository.ledger.InsertOne(tx, entry); err != nil {
				return fmt.Errorf("record starter grant: %w", err)
			}
		}

		for index, item := range questions {
			card := economy.Card{
				ID: cardIDs[index], OwnerID: userID, QuestionID: item.ID, Edition: 1,
				Rarity: economy.RarityForDifficulty(string(item.Difficulty)), Power: 1,
				Status: economy.CardAvailable, Version: 1, CreatedAt: now, UpdatedAt: now,
			}
			_, err := repository.cards.UpdateOne(
				tx,
				bson.M{"ownerId": userID, "questionId": item.ID, "edition": 1},
				bson.M{"$setOnInsert": card},
				options.UpdateOne().SetUpsert(true),
			)
			if err != nil {
				return fmt.Errorf("create starter card %s: %w", item.ID, err)
			}
		}
		return nil
	})
}

func (repository *MongoEconomyRepository) GetWallet(ctx context.Context, userID int64) (*economy.Wallet, error) {
	var wallet economy.Wallet
	if err := repository.wallets.FindOne(ctx, bson.M{"userId": userID}).Decode(&wallet); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get wallet: %w", err)
	}
	return &wallet, nil
}

func (repository *MongoEconomyRepository) ListCards(ctx context.Context, userID int64) ([]economy.Card, error) {
	cursor, err := repository.cards.Find(
		ctx,
		bson.M{"ownerId": userID},
		options.Find().SetSort(bson.D{{Key: "rarity", Value: -1}, {Key: "createdAt", Value: 1}}).SetLimit(500),
	)
	if err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	defer cursor.Close(ctx)
	result := make([]economy.Card, 0)
	if err := cursor.All(ctx, &result); err != nil {
		return nil, fmt.Errorf("decode cards: %w", err)
	}
	return result, nil
}

func (repository *MongoEconomyRepository) GetCardsByIDs(ctx context.Context, ids []int64) (map[int64]economy.Card, error) {
	result := make(map[int64]economy.Card, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	cursor, err := repository.cards.Find(ctx, bson.M{"id": bson.M{"$in": ids}})
	if err != nil {
		return nil, fmt.Errorf("find cards: %w", err)
	}
	defer cursor.Close(ctx)
	var cards []economy.Card
	if err := cursor.All(ctx, &cards); err != nil {
		return nil, fmt.Errorf("decode cards: %w", err)
	}
	for _, card := range cards {
		result[card.ID] = card
	}
	return result, nil
}

func (repository *MongoEconomyRepository) LockCardsForMatch(ctx context.Context, userID int64, cardIDs []int64, matchID int64) error {
	if userID <= 0 || matchID <= 0 || len(cardIDs) != 5 || duplicateInt64(cardIDs) {
		return economy.ErrInvalidEconomyState
	}
	lockRef := fmt.Sprintf("match:%d", matchID)
	return repository.withTransaction(ctx, func(tx context.Context) error {
		result, err := repository.cards.UpdateMany(
			tx,
			bson.M{"id": bson.M{"$in": cardIDs}, "ownerId": userID, "status": economy.CardAvailable, "lockRef": bson.M{"$in": []any{"", nil}}},
			bson.M{"$set": bson.M{"status": economy.CardMatchLocked, "lockRef": lockRef, "updatedAt": time.Now().UTC()}, "$inc": bson.M{"version": 1}},
		)
		if err != nil {
			return fmt.Errorf("lock match cards: %w", err)
		}
		if result.ModifiedCount != int64(len(cardIDs)) {
			return economy.ErrCardUnavailable
		}
		return nil
	})
}

func (repository *MongoEconomyRepository) UnlockMatchCards(ctx context.Context, matchID int64) error {
	if matchID <= 0 {
		return economy.ErrInvalidEconomyState
	}
	_, err := repository.cards.UpdateMany(
		ctx,
		bson.M{"status": economy.CardMatchLocked, "lockRef": fmt.Sprintf("match:%d", matchID)},
		bson.M{"$set": bson.M{"status": economy.CardAvailable, "lockRef": "", "updatedAt": time.Now().UTC()}, "$inc": bson.M{"version": 1}},
	)
	if err != nil {
		return fmt.Errorf("unlock match cards: %w", err)
	}
	return nil
}

func (repository *MongoEconomyRepository) SettleMatchRewards(ctx context.Context, matchID int64) error {
	if matchID <= 0 {
		return economy.ErrInvalidEconomyState
	}
	now := time.Now().UTC()
	return repository.withTransaction(ctx, func(tx context.Context) error {
		matches := repository.database.Collection(matchCollection)
		var aggregate matchdomain.Aggregate
		if err := matches.FindOne(tx, bson.M{"id": matchID}).Decode(&aggregate); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return ErrNotFound
			}
			return fmt.Errorf("find match settlement: %w", err)
		}
		if aggregate.RewardsSettled {
			return nil
		}
		rewards := aggregate.Rewards()
		if (aggregate.Status != matchdomain.StatusCompleted && aggregate.Status != matchdomain.StatusForfeited) || len(rewards) != len(aggregate.Players) {
			return matchdomain.ErrInvalidState
		}
		expectedLockedCards := 0
		for _, player := range aggregate.Players {
			if len(player.Deck) != 0 && len(player.Deck) != matchdomain.DeckSize {
				return economy.ErrInvalidEconomyState
			}
			expectedLockedCards += len(player.Deck)
		}
		if aggregate.Status == matchdomain.StatusCompleted && expectedLockedCards != matchdomain.DeckSize*len(aggregate.Players) {
			return economy.ErrInvalidEconomyState
		}
		positiveRewards := 0
		for _, reward := range rewards {
			if reward < 0 {
				return economy.ErrInvalidEconomyState
			}
			if reward > 0 {
				positiveRewards++
			}
		}
		ledgerIDs, err := newIDs(positiveRewards)
		if err != nil {
			return err
		}
		entries := make([]any, 0, positiveRewards)
		index := 0
		for userID, reward := range rewards {
			if reward == 0 {
				continue
			}
			var wallet economy.Wallet
			if err := repository.wallets.FindOne(tx, bson.M{"userId": userID}).Decode(&wallet); err != nil {
				return fmt.Errorf("find reward wallet: %w", err)
			}
			oldVersion := wallet.Version
			wallet.Balance += reward
			wallet.Version++
			wallet.UpdatedAt = now
			result, err := repository.wallets.ReplaceOne(tx, bson.M{"userId": userID, "version": oldVersion}, wallet)
			if err != nil {
				return fmt.Errorf("credit match reward: %w", err)
			}
			if err := matched(result); err != nil {
				return err
			}
			entries = append(entries, economy.LedgerEntry{
				ID: ledgerIDs[index], UserID: userID, CoinDelta: reward, BalanceAfter: wallet.Balance,
				Reason: "match_reward", ReferenceType: "match", ReferenceID: fmt.Sprint(matchID),
				IdempotencyKey: fmt.Sprintf("match:%d:reward", matchID), EntryPart: fmt.Sprintf("player:%d", userID), CreatedAt: now,
			})
			index++
		}
		if len(entries) > 0 {
			if _, err := repository.ledger.InsertMany(tx, entries); err != nil {
				return fmt.Errorf("record match rewards: %w", err)
			}
		}
		lockRef := fmt.Sprintf("match:%d", matchID)
		cardUpdate := bson.M{
			"$set": bson.M{"status": economy.CardAvailable, "lockRef": "", "updatedAt": now},
			"$inc": bson.M{"version": 1},
		}
		if aggregate.Status == matchdomain.StatusCompleted {
			cardUpdate["$inc"].(bson.M)["plays"] = 1
		}
		releasedCards, err := repository.cards.UpdateMany(
			tx,
			bson.M{"status": economy.CardMatchLocked, "lockRef": lockRef},
			cardUpdate,
		)
		if err != nil {
			return fmt.Errorf("release settled match cards: %w", err)
		}
		if err := matchedAndModified(releasedCards, int64(expectedLockedCards)); err != nil {
			return fmt.Errorf("release settled match cards: %w", economy.ErrInvalidEconomyState)
		}
		if aggregate.Status == matchdomain.StatusCompleted && !aggregate.IsTie {
			winnerIDs := append([]int64(nil), aggregate.WinnerIDs...)
			if len(winnerIDs) == 0 && aggregate.WinnerID > 0 {
				winnerIDs = append(winnerIDs, aggregate.WinnerID)
			}
			if len(winnerIDs) == 0 {
				return economy.ErrInvalidEconomyState
			}
			winnerSet := make(map[int64]struct{}, len(winnerIDs))
			for _, winnerID := range winnerIDs {
				if winnerID <= 0 {
					return economy.ErrInvalidEconomyState
				}
				winnerSet[winnerID] = struct{}{}
			}
			winnerCardIDs := make([]int64, 0, matchdomain.DeckSize*len(winnerSet))
			for _, player := range aggregate.Players {
				if _, winner := winnerSet[player.UserID]; !winner {
					continue
				}
				for _, card := range player.Deck {
					winnerCardIDs = append(winnerCardIDs, card.ID)
				}
			}
			if len(winnerCardIDs) != matchdomain.DeckSize*len(winnerSet) {
				return economy.ErrInvalidEconomyState
			}
			winningCards, err := repository.cards.UpdateMany(
				tx,
				bson.M{"id": bson.M{"$in": winnerCardIDs}, "ownerId": bson.M{"$in": winnerIDs}, "status": economy.CardAvailable, "lockRef": ""},
				bson.M{"$inc": bson.M{"wins": 1, "version": 1}, "$set": bson.M{"updatedAt": now}},
			)
			if err != nil {
				return fmt.Errorf("record winning card mastery: %w", err)
			}
			if err := matchedAndModified(winningCards, int64(len(winnerCardIDs))); err != nil {
				return fmt.Errorf("record winning card mastery: %w", economy.ErrInvalidEconomyState)
			}
		}
		result, err := matches.UpdateOne(tx, bson.M{"id": matchID, "rewardsSettled": false}, bson.M{"$set": bson.M{"rewardsSettled": true}})
		if err != nil {
			return fmt.Errorf("mark rewards settled: %w", err)
		}
		if result.MatchedCount != 1 {
			return ErrConflict
		}
		return nil
	})
}

func (repository *MongoEconomyRepository) withTransaction(ctx context.Context, operation func(context.Context) error) error {
	session, err := repository.database.Client().StartSession()
	if err != nil {
		return fmt.Errorf("start economy transaction: %w", err)
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(
		ctx,
		func(tx context.Context) (any, error) { return nil, operation(tx) },
		options.Transaction().SetReadConcern(readconcern.Snapshot()).SetWriteConcern(writeconcern.Majority()),
	)
	if err != nil {
		return fmt.Errorf("economy transaction: %w", err)
	}
	return nil
}

func duplicateInt64(values []int64) bool {
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

func matched(result *mongo.UpdateResult) error {
	if result == nil || result.MatchedCount != 1 {
		return ErrConflict
	}
	return nil
}

func matchedAndModified(result *mongo.UpdateResult, expected int64) error {
	if expected < 0 || result == nil || result.MatchedCount != expected || result.ModifiedCount != expected {
		return ErrConflict
	}
	return nil
}
