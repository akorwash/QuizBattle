package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/akorwash/QuizBattle/datastore/entites"
	matchdomain "github.com/akorwash/QuizBattle/domain/match"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoGameRepository struct {
	collection *mongo.Collection
}

const maximumBattleMembers = 8

func NewMongoGameRepository(database *mongo.Database) *MongoGameRepository {
	// MongoDB collection names are case-sensitive. This is the name used by the
	// original application and therefore preserves existing battle history.
	return &MongoGameRepository{collection: database.Collection("Game")}
}

func (repository *MongoGameRepository) CountActiveGame(userID int64) (int64, error) {
	ctx, cancel := operationContext()
	defer cancel()
	// isactive is a lobby lifecycle flag, not the match status. Terminal games
	// intentionally remain active so both players can reopen their saved result,
	// but they must no longer consume the owner's concurrent-battle quota.
	count, err := repository.collection.CountDocuments(ctx, bson.M{
		"userid":   userID,
		"isactive": true,
		"state": bson.M{"$nin": bson.A{
			string(matchdomain.StatusCompleted),
			string(matchdomain.StatusForfeited),
		}},
	})
	if err != nil {
		return 0, fmt.Errorf("count active games: %w", err)
	}
	return count, nil
}

func (repository *MongoGameRepository) Add(game *entites.Game) error {
	if game == nil {
		return fmt.Errorf("add game: nil entity")
	}
	if err := validateStoredGameBotConfiguration(game); err != nil {
		return fmt.Errorf("add game: %w", err)
	}
	id, err := newID()
	if err != nil {
		return err
	}
	game.ID = id
	if game.CreatedAt.IsZero() {
		game.CreatedAt = time.Now().UTC()
	}
	ctx, cancel := operationContext()
	defer cancel()
	if _, err := repository.collection.InsertOne(ctx, game); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrConflict
		}
		return fmt.Errorf("add game: %w", err)
	}
	return nil
}

func (repository *MongoGameRepository) JoinGame(gameID, userID int64) error {
	if gameID <= 0 || userID <= 0 {
		return ErrConflict
	}
	ctx, cancel := operationContext()
	defer cancel()
	filter := bson.M{
		"id":          gameID,
		"isactive":    true,
		"ispublic":    true,
		"mode":        bson.M{"$ne": string(matchdomain.ModeBot)},
		"bot":         bson.M{"$exists": false},
		"joinedusers": bson.M{"$ne": userID},
		"$or": bson.A{
			bson.M{"state": "lobby"},
			bson.M{"state": ""},
			bson.M{"state": bson.M{"$exists": false}},
		},
		"$expr": bson.M{"$lt": bson.A{
			bson.M{"$size": bson.M{"$ifNull": bson.A{"$joinedusers", bson.A{}}}},
			bson.M{"$min": bson.A{bson.M{"$ifNull": bson.A{"$maxplayers", 2}}, maximumBattleMembers}},
		}},
	}
	result, err := repository.collection.UpdateOne(ctx, filter, bson.M{"$addToSet": bson.M{"joinedusers": userID}})
	if err != nil {
		return fmt.Errorf("join game: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrConflict
	}
	return nil
}

func validateStoredGameBotConfiguration(game *entites.Game) error {
	mode, err := matchdomain.NormalizeMode(game.Mode)
	if err != nil {
		return matchdomain.ErrInvalidMatch
	}
	if mode != matchdomain.ModeBot {
		if game.Bot != nil {
			return matchdomain.ErrInvalidMatch
		}
		return nil
	}
	if game.UserID <= 0 || game.IsPublic || game.MaxPlayers != matchdomain.MaxPlayers(matchdomain.ModeBot) ||
		game.Bot == nil || game.Bot.ActorID != matchdomain.BotActorID || strings.TrimSpace(game.Bot.Name) == "" ||
		len(game.JoinedUsers) != 1 || game.JoinedUsers[0] != game.UserID {
		return matchdomain.ErrInvalidMatch
	}
	strategy, err := matchdomain.NormalizeBotStrategy(game.Bot.Strategy)
	if err != nil || string(strategy) != game.Bot.Strategy {
		return matchdomain.ErrInvalidMatch
	}
	return nil
}

func (repository *MongoGameRepository) LeaveGame(gameID, userID int64) error {
	return repository.updateMembers(
		bson.M{
			"id": gameID, "isactive": true, "joinedusers": userID,
			"$or": exitAllowedGameStates(),
		},
		bson.M{"$pull": bson.M{"joinedusers": userID}},
	)
}

func (repository *MongoGameRepository) updateMembers(filter, update bson.M) error {
	ctx, cancel := operationContext()
	defer cancel()
	result, err := repository.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("update game membership: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (repository *MongoGameRepository) CloseGame(gameID int64) error {
	ctx, cancel := operationContext()
	defer cancel()
	result, err := repository.collection.UpdateOne(
		ctx,
		bson.M{"id": gameID, "isactive": true, "$or": exitAllowedGameStates()},
		bson.M{"$set": bson.M{"isactive": false}},
	)
	if err != nil {
		return fmt.Errorf("close game: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func exitAllowedGameStates() bson.A {
	return bson.A{
		bson.M{"state": "lobby"},
		bson.M{"state": ""},
		bson.M{"state": bson.M{"$exists": false}},
		bson.M{"state": string(matchdomain.StatusCompleted)},
		bson.M{"state": string(matchdomain.StatusForfeited)},
	}
}

func (repository *MongoGameRepository) GetGameByID(id int64) (*entites.Game, error) {
	ctx, cancel := operationContext()
	defer cancel()
	var game entites.Game
	if err := repository.collection.FindOne(ctx, bson.M{"id": id}).Decode(&game); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find game: %w", err)
	}
	return &game, nil
}

func (repository *MongoGameRepository) GetPublicBattle() ([]entites.Game, error) {
	return repository.findMany(bson.M{
		"ispublic": true,
		"isactive": true,
		"mode":     bson.M{"$ne": string(matchdomain.ModeBot)},
		"bot":      bson.M{"$exists": false},
		"$or": bson.A{
			bson.M{"state": "lobby"},
			bson.M{"state": ""},
			bson.M{"state": bson.M{"$exists": false}},
		},
		"$expr": bson.M{"$and": bson.A{
			bson.M{"$lte": bson.A{
				bson.M{"$size": bson.M{"$ifNull": bson.A{"$joinedusers", bson.A{}}}},
				bson.M{"$ifNull": bson.A{"$maxplayers", 2}},
			}},
			bson.M{"$lte": bson.A{bson.M{"$ifNull": bson.A{"$maxplayers", 2}}, maximumBattleMembers}},
		}},
	})
}

func (repository *MongoGameRepository) GetMyBattle(userID int64) ([]entites.Game, error) {
	return repository.findMany(bson.M{
		"joinedusers": userID,
		"isactive":    true,
		"$expr": bson.M{"$and": bson.A{
			bson.M{"$lte": bson.A{
				bson.M{"$size": bson.M{"$ifNull": bson.A{"$joinedusers", bson.A{}}}},
				bson.M{"$ifNull": bson.A{"$maxplayers", 2}},
			}},
			bson.M{"$lte": bson.A{bson.M{"$ifNull": bson.A{"$maxplayers", 2}}, maximumBattleMembers}},
		}},
	})
}

func (repository *MongoGameRepository) findMany(filter bson.M) ([]entites.Game, error) {
	ctx, cancel := operationContext()
	defer cancel()
	cursor, err := repository.collection.Find(ctx, filter, options.Find().SetLimit(100).SetSort(bson.D{{Key: "createdat", Value: -1}, {Key: "id", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("find games: %w", err)
	}
	defer cursor.Close(ctx)
	games := make([]entites.Game, 0)
	if err := cursor.All(ctx, &games); err != nil {
		return nil, fmt.Errorf("decode games: %w", err)
	}
	return games, nil
}
