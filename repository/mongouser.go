package repository

import (
	"context"
	"fmt"

	"github.com/akorwash/QuizBattle/datastore"
	"github.com/akorwash/QuizBattle/datastore/entites"
	"go.mongodb.org/mongo-driver/bson"
)

//MongoUserRepository repo to query the users collection at mongo database
type MongoUserRepository struct{}

//NewMongoUserRepository ctor for MongoUserRepository
func NewMongoUserRepository() *MongoUserRepository {
	repo := MongoUserRepository{}
	return &repo
}

//GetUserByName query the database and find user by their username
func (repos *MongoUserRepository) GetUserByName(_name string) (*entites.User, error) {
	dbcontext, err := datastore.GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		return nil, err
	}
	//
	filter := bson.M{"username": _name}
	iter := dbcontext.Collection("users")
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
	dbcontext, err := datastore.GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		return nil, err
	}

	filter := bson.M{"mobilenumber": _mobile}
	iter := dbcontext.Collection("users")
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
	dbcontext, err := datastore.GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		return nil, err
	}

	filter := bson.M{"email": _email}
	iter := dbcontext.Collection("users")
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
	dbcontext, err := datastore.GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		return nil, err
	}

	filter := bson.M{"id": bson.M{"$eq": _id}}
	iter := dbcontext.Collection("users")
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
	dbcontext, err := datastore.GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		return err
	}

	iter := dbcontext.Collection("users")
	//create the bot account
	iter.InsertOne(context.Background(), user)
	return nil
}

//UpdateUser to do
func (repos *MongoUserRepository) UpdateUser(user entites.User) error {
	dbcontext, err := datastore.GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		return err
	}
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
	iter := dbcontext.Collection("users")

	_, err = iter.UpdateOne(
		context.Background(),
		filter,
		update,
	)
	return err
}
