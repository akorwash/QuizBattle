package repository

import (
	"context"
	"fmt"

	"github.com/akorwash/QuizBattle/datastore"
	"github.com/akorwash/QuizBattle/datastore/entites"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

//MongoUserRepository repo to query the users collection at mongo database
type MongoUserRepository struct {
	mongoContext *mongo.Database
}

//NewMongoUserRepository ctor for MongoUserRepository
func NewMongoUserRepository(dbConfig datastore.DBConfiguration) (*MongoUserRepository, error) {
	dbcontext, err := datastore.GetContext(dbConfig)
	if err != nil {
		println("Error while get database context: %v\n", err)
		return nil, err
	}
	repo := MongoUserRepository{}
	repo.mongoContext = dbcontext
	return &repo, nil
}

//GetUserByName query the database and find user by their username
func (repos *MongoUserRepository) GetUserByName(_name string) (*entites.User, error) {
	filter := bson.M{"username": _name}
	iter := repos.mongoContext.Collection("users")
	cursor, err := iter.Find(context.Background(), filter)
	if err != nil {
		println("Error while getting all todos, Reason: %v\n", err)
		return nil, err
	}

	var _user entites.User
	for cursor.Next(context.Background()) {
		cursor.Decode(&_user)
		break
	}
	if _user.Username == "" {
		return nil, fmt.Errorf("User not found")
	}
	return &_user, nil
}

//GetUserByMobile query the database and find user by their mobile number
func (repos *MongoUserRepository) GetUserByMobile(_mobile string) (*entites.User, error) {
	filter := bson.M{"mobilenumber": _mobile}
	iter := repos.mongoContext.Collection("users")
	cursor, err := iter.Find(context.Background(), filter)
	if err != nil {
		println("Error while getting all todos, Reason: %v\n", err)
		return nil, err
	}

	var _user entites.User
	for cursor.Next(context.Background()) {
		cursor.Decode(&_user)
		break
	}
	if _user.Username == "" {
		return nil, fmt.Errorf("User not found")
	}
	return &_user, nil
}

//GetUserByEmail query the database and find user by their email
func (repos *MongoUserRepository) GetUserByEmail(_email string) (*entites.User, error) {
	filter := bson.M{"email": _email}
	iter := repos.mongoContext.Collection("users")
	cursor, err := iter.Find(context.Background(), filter)
	if err != nil {
		println("Error while getting all todos, Reason: %v\n", err)
		return nil, err
	}

	var _user entites.User
	for cursor.Next(context.Background()) {
		cursor.Decode(&_user)
		break
	}
	if _user.Username == "" {
		return nil, fmt.Errorf("User not found")
	}
	return &_user, nil
}

//GetUserByID query the database and find user by their email
func (repos *MongoUserRepository) GetUserByID(_id int64) (*entites.User, error) {
	filter := bson.M{"id": bson.M{"$eq": _id}}
	iter := repos.mongoContext.Collection("users")
	cursor, err := iter.Find(context.Background(), filter)
	if err != nil {
		println("Error while getting all todos, Reason: %v\n", err)
		return nil, err
	}

	var _user entites.User
	for cursor.Next(context.Background()) {
		cursor.Decode(&_user)
		break
	}
	if _user.Username == "" {
		return nil, fmt.Errorf("User not found")
	}
	return &_user, nil
}

//AddUser to do
func (repos *MongoUserRepository) AddUser(user entites.User) error {
	iter := repos.mongoContext.Collection("users")

	userCount, err := iter.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		println("Error while count users recored: %v\n", err)
		return err

	}
	user.ID = userCount + 1

	iter.InsertOne(context.Background(), user)
	return nil
}

//UpdateUser to do
func (repos *MongoUserRepository) UpdateUser(user entites.User) error {
	filter := bson.M{"id": bson.M{"$eq": user.ID}}
	update := bson.M{
		"$set": bson.M{
			"fullname":     user.Fullname,
			"monthofbirth": user.MonthOfBirth,
			"yearofbirth":  user.YearOfBirth,
			"dayofbirth":   user.DayOfBirth,
			"mobilenumber": user.MobileNumber,
		},
	}
	iter := repos.mongoContext.Collection("users")

	_, err := iter.UpdateOne(
		context.Background(),
		filter,
		update,
	)
	return err
}
