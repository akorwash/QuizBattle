package datastore

import (
	"go.mongodb.org/mongo-driver/mongo"
)

//DBContext to do
type DBContext struct {
}

//MyDBContext to do
var MyDBContext DBContext

var mongoContext *mongo.Database

//BaseDirectory to do
var BaseDirectory string

//InitializingDB here we will intaite the database, also seed the database
func (_context *DBContext) InitializingDB() *DBContext {
	seedInit := SeedInitializer{}
	seedInit.Seed()
	return _context
}
