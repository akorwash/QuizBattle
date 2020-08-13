package controllertests

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/akorwash/QuizBattle/api"
	"github.com/akorwash/QuizBattle/api/controller"
	"github.com/akorwash/QuizBattle/datastore"
	"github.com/akorwash/QuizBattle/datastore/entites"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var server = api.App{}
var userController = controller.UserController{}
var questionController = controller.QuestionController{}

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

func seedtestUser() (*entites.User, error) {
	dbcontext, err := datastore.GetContext()
	if err != nil {
		log.Fatal("Error while get database context: \n", err)
		return nil, err
	}

	iter := dbcontext.Collection("users")
	userCount, err := iter.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		println("Error while count users recored: %v\n", err)
		return nil, err
	}

	user := entites.User{ID: userCount + 1, Username: "testuser", Password: "TestPass#2010", Email: "test@test.com", MobileNumber: "01585285285"}

	//create the bot account
	iter.InsertOne(context.Background(), user)
	return &user, nil
}

func deletetestUser(user *entites.User) error {
	dbcontext, err := datastore.GetContext()
	if err != nil {
		log.Fatal("Error while get database context: \n", err)
		return err
	}

	iter := dbcontext.Collection("users")
	//create the bot account
	iter.DeleteOne(context.Background(), *user)
	return nil
}

func seedtestQuestions() ([]entites.Question, error) {
	question1 := entites.Question{ID: 10, Header: "Test 10"}
	question2 := entites.Question{ID: 20, Header: "Test 20"}
	question3 := entites.Question{ID: 30, Header: "Test 30"}
	question4 := entites.Question{ID: 40, Header: "Test 40"}
	questions := []entites.Question{question1, question2, question3, question4}
	dbcontext, err := datastore.GetContext()
	if err != nil {
		log.Fatal("Error while get database context: \n", err)
		return nil, err
	}

	iter := dbcontext.Collection("Question")
	//create the bot account
	for _, _q := range questions {
		iter.InsertOne(context.Background(), _q)
	}
	return questions, nil
}

func deleteSeedtestQuestions(questions []entites.Question) ([]entites.Question, error) {
	dbcontext, err := datastore.GetContext()
	if err != nil {
		log.Fatal("Error while get database context: \n", err)
		return nil, err
	}

	iter := dbcontext.Collection("Question")
	//create the bot account
	for _, _q := range questions {
		iter.DeleteOne(context.Background(), _q)
	}
	return questions, nil
}
