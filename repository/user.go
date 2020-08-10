package repository

import (
	"context"

	"github.com/akorwash/QuizBattle/datastore"
	"github.com/akorwash/QuizBattle/datastore/entites"
	"go.mongodb.org/mongo-driver/bson"
)

//UserRepository to do
type UserRepository struct{}

//GetUserByName to do
func (repos *UserRepository) GetUserByName(_name string) (*entites.User, error) {
	dbcontext, cancelContext, err := datastore.GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		defer cancelContext()
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
	defer cancelContext()
	if _user.Username == "" {
		return nil, nil
	}
	return &_user, nil
}

//GetUserByMobile to do
func (repos *UserRepository) GetUserByMobile(_mobile string) (*entites.User, error) {
	dbcontext, cancelContext, err := datastore.GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		defer cancelContext()
		return nil, err
	}

	filter := bson.M{"mobileNumber": _mobile}
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
	defer cancelContext()
	if _user.Username == "" {
		return nil, nil
	}
	return &_user, nil
}

//GetUserByEmail to do
func (repos *UserRepository) GetUserByEmail(_email string) (*entites.User, error) {
	dbcontext, cancelContext, err := datastore.GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		defer cancelContext()
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
	defer cancelContext()
	if _user.Username == "" {
		return nil, nil
	}
	return &_user, nil
}
