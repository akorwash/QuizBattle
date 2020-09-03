package repository

import (
	"context"
	"fmt"

	"github.com/akorwash/QuizBattle/datastore"
	"github.com/akorwash/QuizBattle/datastore/entites"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

//MongoGameRepository repo to query the question collection at mongo database
type MongoGameRepository struct {
	mongoContext *mongo.Database
}

//NewMongoGameRepository ctor for MongoQuestionRepository
func NewMongoGameRepository(dbConfig datastore.DBConfiguration) (*MongoGameRepository, error) {
	dbcontext, err := datastore.GetContext(dbConfig)
	if err != nil {
		println("Error while get database context: %v\n", err)
		return nil, err
	}

	repo := MongoGameRepository{}
	repo.mongoContext = dbcontext
	return &repo, nil
}

//Count get total count of games
func (repos MongoGameRepository) Count() (int64, error) {
	iter := repos.mongoContext.Collection("Game")

	count, err := iter.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		println("Error while count users recored: %v\n", err)
		return 0, err

	}

	return count, err
}

//CountActiveGame get total count of games that still active
func (repos MongoGameRepository) CountActiveGame(usreID uint64) (int64, error) {
	iter := repos.mongoContext.Collection("Game")

	count, err := iter.CountDocuments(context.Background(), bson.M{"userid": bson.M{"$eq": usreID}, "isactive": bson.M{"$eq": true}})
	if err != nil {
		println("Error while count users recored: %v\n", err)
		return 0, err

	}

	return count, err
}

//Add add new game
func (repos MongoGameRepository) Add(game entites.Game) error {
	iter := repos.mongoContext.Collection("Game")

	iter.InsertOne(context.Background(), game)
	return nil
}

//JoinedGame new user to join for game
func (repos MongoGameRepository) JoinedGame(gameID int64, usreID []uint64) error {
	iter := repos.mongoContext.Collection("Game")

	filter := bson.M{"id": bson.M{"$eq": gameID}}
	update := bson.M{
		"$set": bson.M{
			"joinedusers": usreID,
		},
	}

	_, err := iter.UpdateOne(
		context.Background(),
		filter,
		update,
	)
	return err
}

//CloseGame this will end the game
func (repos MongoGameRepository) CloseGame(gameID int64) error {
	iter := repos.mongoContext.Collection("Game")

	filter := bson.M{"id": bson.M{"$eq": gameID}}
	update := bson.M{
		"$set": bson.M{
			"isactive": false,
		},
	}

	_, err := iter.UpdateOne(
		context.Background(),
		filter,
		update,
	)
	return err
}

//GetGameByID query the database and find user by their email
func (repos *MongoGameRepository) GetGameByID(_id int64) (*entites.Game, error) {
	filter := bson.M{"id": bson.M{"$eq": _id}}
	iter := repos.mongoContext.Collection("Game")
	cursor, err := iter.Find(context.Background(), filter)
	if err != nil {
		println("Error while getting all todos, Reason: %v\n", err)
		return nil, err
	}

	var _entity entites.Game
	for cursor.Next(context.Background()) {
		cursor.Decode(&_entity)
		break
	}

	if _entity.ID == 0 {
		return nil, fmt.Errorf("game not found")
	}
	return &_entity, nil
}

//GetPublicBattle query the database and find public battles
func (repos *MongoGameRepository) GetPublicBattle() ([]entites.Game, error) {
	filter := bson.M{"ispublic": bson.M{"$eq": true}}
	iter := repos.mongoContext.Collection("Game")
	cursor, err := iter.Find(context.Background(), filter)
	if err != nil {
		println("Error while getting all todos, Reason: %v\n", err)
		return nil, err
	}

	var entities []entites.Game
	for cursor.Next(context.Background()) {
		var _entity entites.Game
		cursor.Decode(&_entity)
		if _entity.IsActive {
			entities = append(entities, _entity)
		}
	}
	return entities, nil
}

//GetMyBattle query the database and find public battles
func (repos *MongoGameRepository) GetMyBattle(userID uint64) ([]entites.Game, error) {
	filter := bson.M{"joinedusers": bson.M{"$in": []uint64{userID}}}
	iter := repos.mongoContext.Collection("Game")
	cursor, err := iter.Find(context.Background(), filter)
	if err != nil {
		println("Error while getting all todos, Reason: %v\n", err)
		return nil, err
	}

	var entities []entites.Game
	for cursor.Next(context.Background()) {
		var _entity entites.Game
		cursor.Decode(&_entity)
		entities = append(entities, _entity)
	}
	return entities, nil
}
