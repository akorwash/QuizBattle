package repositorytests

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func TestMain(m *testing.M) {
	Database()
	os.Exit(m.Run())
}

func Database() {
	// Database conection string
	clientOptions := options.Client().ApplyURI("mongodb://" + os.Getenv("MongoUsername") + ":" + os.Getenv("MongoPassword") + "@ds029979.mlab.com:29979/heroku_9gr1xz3v?retryWrites=false")
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
	}

	fmt.Println("Success to connect to database")
}
