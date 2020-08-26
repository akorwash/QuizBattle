package datastore

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

//DBConfiguration config of database
type DBConfiguration struct {
	DBName   string
	Username string
	Password string
	HostID   string
	PORT     string
}

//GetContext each time need to connect the database must get active context
func GetContext(dbConfig DBConfiguration) (*mongo.Database, error) {
	// Database Config
	clientOptions := options.Client().ApplyURI("mongodb://" + dbConfig.Username + ":" + dbConfig.Password + "@" + dbConfig.HostID + ".mlab.com:" + dbConfig.PORT + "/" + dbConfig.DBName + "?retryWrites=false")
	client, err := mongo.NewClient(clientOptions)
	//Set up a context required by mongo.Connect
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	err = client.Connect(ctx)
	//Cancel context to avoid memory leak
	defer cancel()

	// Ping our db connection
	err = client.Ping(context.Background(), readpref.Primary())
	if err != nil {
		log.Fatal("Couldn't connect to the database", err)
		return nil, err
	}

	// Connect to the database
	return client.Database(dbConfig.DBName), nil
}
