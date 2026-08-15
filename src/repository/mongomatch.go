package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/akorwash/QuizBattle/domain/economy"
	matchdomain "github.com/akorwash/QuizBattle/domain/match"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

const matchCollection = "Matches"

type MongoMatchRepository struct {
	database *mongo.Database
	matches  *mongo.Collection
	games    *mongo.Collection
	cards    *mongo.Collection
}

func NewMongoMatchRepository(database *mongo.Database) *MongoMatchRepository {
	return &MongoMatchRepository{
		database: database, matches: database.Collection(matchCollection),
		games: database.Collection("Game"), cards: database.Collection(cardCollection),
	}
}

func (repository *MongoMatchRepository) CreateForGame(ctx context.Context, aggregate *matchdomain.Aggregate) error {
	if aggregate == nil || aggregate.ID <= 0 || aggregate.GameID <= 0 || len(aggregate.Players) < 2 || len(aggregate.Players) > maximumBattleMembers {
		return matchdomain.ErrInvalidMatch
	}
	playerIDs := make([]int64, 0, len(aggregate.Players))
	for _, player := range aggregate.Players {
		playerIDs = append(playerIDs, player.UserID)
	}
	return repository.withTransaction(ctx, func(tx context.Context) error {
		result, err := repository.games.UpdateOne(
			tx,
			bson.M{
				"id": aggregate.GameID, "isactive": true, "joinedusers": bson.M{"$size": len(playerIDs), "$all": playerIDs},
				"$or":  bson.A{bson.M{"state": "lobby"}, bson.M{"state": ""}, bson.M{"state": bson.M{"$exists": false}}},
				"$and": bson.A{bson.M{"$or": bson.A{bson.M{"matchid": 0}, bson.M{"matchid": bson.M{"$exists": false}}}}},
			},
			bson.M{"$set": bson.M{"state": string(matchdomain.StatusCollectingDecks), "matchid": aggregate.ID}},
		)
		if err != nil {
			return fmt.Errorf("attach match to game: %w", err)
		}
		if result.MatchedCount != 1 {
			return ErrConflict
		}
		if _, err := repository.matches.InsertOne(tx, aggregate); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return ErrConflict
			}
			return fmt.Errorf("create match: %w", err)
		}
		return nil
	})
}

func (repository *MongoMatchRepository) GetByGameID(ctx context.Context, gameID int64) (*matchdomain.Aggregate, error) {
	var aggregate matchdomain.Aggregate
	if err := repository.matches.FindOne(ctx, bson.M{"gameId": gameID}).Decode(&aggregate); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get match: %w", err)
	}
	return &aggregate, nil
}

func (repository *MongoMatchRepository) Update(ctx context.Context, aggregate *matchdomain.Aggregate, expectedVersion int64) error {
	if aggregate == nil || expectedVersion <= 0 || aggregate.Version <= expectedVersion {
		return ErrConflict
	}
	return repository.withTransaction(ctx, func(tx context.Context) error {
		result, err := repository.matches.ReplaceOne(tx, bson.M{"id": aggregate.ID, "version": expectedVersion}, aggregate)
		if err != nil {
			return fmt.Errorf("update match: %w", err)
		}
		if result.MatchedCount != 1 {
			return ErrConflict
		}
		return repository.syncGameState(tx, aggregate)
	})
}

func (repository *MongoMatchRepository) CommitDeck(
	ctx context.Context,
	aggregate *matchdomain.Aggregate,
	expectedVersion, userID int64,
	newCardIDs, previousCardIDs []int64,
) error {
	if aggregate == nil || expectedVersion <= 0 || userID <= 0 || len(newCardIDs) != matchdomain.DeckSize {
		return economy.ErrInvalidEconomyState
	}
	lockRef := fmt.Sprintf("match:%d", aggregate.ID)
	now := time.Now().UTC()
	return repository.withTransaction(ctx, func(tx context.Context) error {
		if len(previousCardIDs) > 0 {
			result, err := repository.cards.UpdateMany(
				tx,
				bson.M{"id": bson.M{"$in": previousCardIDs}, "ownerId": userID, "status": economy.CardMatchLocked, "lockRef": lockRef},
				bson.M{"$set": bson.M{"status": economy.CardAvailable, "lockRef": "", "updatedAt": now}, "$inc": bson.M{"version": 1}},
			)
			if err != nil {
				return fmt.Errorf("release previous deck: %w", err)
			}
			if result.MatchedCount != int64(len(previousCardIDs)) {
				return economy.ErrInvalidEconomyState
			}
		}
		result, err := repository.cards.UpdateMany(
			tx,
			bson.M{"id": bson.M{"$in": newCardIDs}, "ownerId": userID, "status": economy.CardAvailable, "$or": bson.A{bson.M{"lockRef": ""}, bson.M{"lockRef": bson.M{"$exists": false}}}},
			bson.M{"$set": bson.M{"status": economy.CardMatchLocked, "lockRef": lockRef, "updatedAt": now}, "$inc": bson.M{"version": 1}},
		)
		if err != nil {
			return fmt.Errorf("lock committed deck: %w", err)
		}
		if result.ModifiedCount != int64(len(newCardIDs)) {
			return economy.ErrCardUnavailable
		}
		matchResult, err := repository.matches.ReplaceOne(tx, bson.M{"id": aggregate.ID, "version": expectedVersion}, aggregate)
		if err != nil {
			return fmt.Errorf("commit deck to match: %w", err)
		}
		if matchResult.MatchedCount != 1 {
			return ErrConflict
		}
		return nil
	})
}

func (repository *MongoMatchRepository) syncGameState(ctx context.Context, aggregate *matchdomain.Aggregate) error {
	state := string(aggregate.Status)
	update := bson.M{"state": state}
	result, err := repository.games.UpdateOne(ctx, bson.M{"id": aggregate.GameID, "matchid": aggregate.ID}, bson.M{"$set": update})
	if err != nil {
		return fmt.Errorf("sync game match state: %w", err)
	}
	if result.MatchedCount != 1 {
		return ErrConflict
	}
	return nil
}

func (repository *MongoMatchRepository) withTransaction(ctx context.Context, operation func(context.Context) error) error {
	session, err := repository.database.Client().StartSession()
	if err != nil {
		return fmt.Errorf("start match transaction: %w", err)
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(
		ctx,
		func(tx context.Context) (any, error) { return nil, operation(tx) },
		options.Transaction().SetReadConcern(readconcern.Snapshot()).SetWriteConcern(writeconcern.Majority()),
	)
	if err != nil {
		return fmt.Errorf("match transaction: %w", err)
	}
	return nil
}
