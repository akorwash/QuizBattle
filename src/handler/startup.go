package handler

import (
	"math/rand"
	"os"
	"time"

	"github.com/akorwash/QuizBattle/datastore"
)

//Startup to do
type Startup struct {
	Errors error
}

const (
	//NumberOFQuestion to do
	NumberOFQuestion = 100
	//MinNumOfBots to do
	MinNumOfBots = 25
	//MaxNubOfBots to do
	MaxNubOfBots = 63
	//MinBotLevel to do
	MinBotLevel = 1
	//MaxBotLevel to do
	MaxBotLevel = 25
	//MinNumOfCards to do
	MinNumOfCards = 100
	//MaxNubOfCards to do
	MaxNubOfCards = 250
)

//StartUp responsible for intiate the database, randmoize with UNIX Nano
func StartUp(dbConfig datastore.DBConfiguration) *Startup {
	//Intialize the database, to prepare the context and run the seed mthods
	datastore.MyDBContext.InitializingDB(dbConfig)
	//somtimes we need to generate randome numbers, we use this to seed the randome numbers
	rand.Seed(time.Now().UnixNano())
	return &Startup{}
}

//GetDBConfig get config for production database
func GetDBConfig() datastore.DBConfiguration {
	config := datastore.DBConfiguration{}
	config.DBName = os.Getenv("MongoDBName")
	config.HostID = os.Getenv("MongoHostID")
	config.PORT = os.Getenv("MongoPORT")
	config.Password = os.Getenv("MongoPassword")
	config.Username = os.Getenv("MongoUsername")
	return config
}

//GetTestDBConfig get config for test database
func GetTestDBConfig() datastore.DBConfiguration {
	config := datastore.DBConfiguration{}
	config.DBName = os.Getenv("TestMongoDBName")
	config.HostID = os.Getenv("TestMongoHostID")
	config.PORT = os.Getenv("TestMongoPORT")
	config.Password = os.Getenv("TestMongoPassword")
	config.Username = os.Getenv("TestMongoUsername")
	return config
}

//GetRedisConfig get config for redis
func GetRedisConfig() datastore.RedisConfiguration {
	config := datastore.RedisConfiguration{}
	config.EndPoint = os.Getenv("RedisEndPoint")
	config.Password = os.Getenv("RedisPassword")
	return config
}
