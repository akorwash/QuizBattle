package repository

import (
	"errors"
	"fmt"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoUserRepository struct {
	collection *mongo.Collection
}

func NewMongoUserRepository(database *mongo.Database) *MongoUserRepository {
	return &MongoUserRepository{collection: database.Collection("users")}
}

func (repository *MongoUserRepository) GetUserByName(name string) (*entites.User, error) {
	return repository.findOne(bson.M{"username": name})
}

func (repository *MongoUserRepository) GetUserByMobile(mobile string) (*entites.User, error) {
	return repository.findOne(bson.M{"mobilenumber": mobile})
}

func (repository *MongoUserRepository) GetUserByEmail(email string) (*entites.User, error) {
	return repository.findOne(bson.M{"email": email})
}

func (repository *MongoUserRepository) GetUserByID(id int64) (*entites.User, error) {
	return repository.findOne(bson.M{"id": id})
}

func (repository *MongoUserRepository) GetUsersByIDs(ids []int64) (map[int64]entites.User, error) {
	users := make(map[int64]entites.User, len(ids))
	if len(ids) == 0 {
		return users, nil
	}
	ctx, cancel := operationContext()
	defer cancel()
	cursor, err := repository.collection.Find(ctx, bson.M{"id": bson.M{"$in": ids}})
	if err != nil {
		return nil, fmt.Errorf("find users: %w", err)
	}
	defer cursor.Close(ctx)
	var found []entites.User
	if err := cursor.All(ctx, &found); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}
	for _, user := range found {
		users[user.ID] = user
	}
	return users, nil
}

func (repository *MongoUserRepository) findOne(filter bson.M) (*entites.User, error) {
	ctx, cancel := operationContext()
	defer cancel()
	var user entites.User
	if err := repository.collection.FindOne(ctx, filter).Decode(&user); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}
	return &user, nil
}

func (repository *MongoUserRepository) AddUser(user *entites.User) error {
	if user == nil {
		return fmt.Errorf("add user: nil entity")
	}
	id, err := newID()
	if err != nil {
		return err
	}
	user.ID = id
	ctx, cancel := operationContext()
	defer cancel()
	if _, err := repository.collection.InsertOne(ctx, user); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrConflict
		}
		return fmt.Errorf("add user: %w", err)
	}
	return nil
}

func (repository *MongoUserRepository) UpdateUser(user entites.User) error {
	ctx, cancel := operationContext()
	defer cancel()
	result, err := repository.collection.UpdateOne(
		ctx,
		bson.M{"id": user.ID},
		bson.M{"$set": bson.M{
			"fullname":     user.Fullname,
			"monthofbirth": user.MonthOfBirth,
			"yearofbirth":  user.YearOfBirth,
			"dayofbirth":   user.DayOfBirth,
		}},
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}
