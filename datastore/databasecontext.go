package datastore

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/akorwash/QuizBattle/datastore/entites"
)

//DBContext to do
type DBContext struct {
}

//MyDBContext to do
var MyDBContext DBContext

var mongoContext *mongo.Database

//BaseDirectory to do
var BaseDirectory string

//InitializingDB to do
func (_context *DBContext) InitializingDB() *DBContext {
	seedInit := SeedInitializer{}
	seedInit.Seed()
	return _context
}

//AddUser to do
func (_context *DBContext) AddUser(user entites.User) error {
	dbcontext, cancelContext, err := GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		defer cancelContext()
		return err
	}
	iter := dbcontext.Collection("users")
	//create the bot account
	iter.InsertOne(context.Background(), user)
	defer cancelContext()
	return nil
}
