package repositorytests

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/akorwash/QuizBattle/datastore"
	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/handler"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/service"
	"github.com/akorwash/QuizBattle/service/createaccount"
	"github.com/akorwash/QuizBattle/service/login"
	"github.com/akorwash/QuizBattle/service/updateaccount"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var qeustionRepo repository.IQuestionRepository
var userRepo repository.IUserRepository
var questionSvc service.IQuestionServices
var updateAccSvc service.IUpdateAccountServices
var createAccSvc service.ICreateAccountServices
var loginSvc *login.LoginService
var dbcontext *mongo.Database

func TestMain(m *testing.M) {
	err := initTest()
	if err == nil {
		Database()
		os.Exit(m.Run())
	}
}

func initTest() error {
	dbConfig := handler.GetTestDBConfig()
	_questionRepo, errQuesRerpo := repository.NewMongoQuestionRepository(dbConfig)
	if errQuesRerpo != nil {
		println("Error while get database context For Repo: %v\n", errQuesRerpo)
		return errQuesRerpo
	}
	qeustionRepo = _questionRepo
	_userRepo, errUserRepo := repository.NewMongoUserRepository(dbConfig)
	if errUserRepo != nil {
		println("Error while get database context For Repo: %v\n", errUserRepo)
		return errUserRepo
	}

	userRepo = _userRepo
	questionSvc = service.NewQuestionServices(qeustionRepo)
	createAccSvc = createaccount.NEW(userRepo)
	updateAccSvc = updateaccount.NEW(userRepo)
	loginSvc = login.New(userRepo)
	return nil
}

func Database() {
	dbConfig := handler.GetTestDBConfig()
	_dbcontext, err := datastore.GetContext(dbConfig)
	if err != nil {
		log.Fatal("Error while get database context: \n", err)
		return
	}
	dbcontext = _dbcontext
	fmt.Println("Success to connect to database")
}

func seedtestUser() (*entites.User, error) {
	iter := dbcontext.Collection("users")
	userCount, err := iter.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		println("Error while count users recored: %v\n", err)
		return nil, err

	}

	user := entites.User{ID: userCount + 1, Username: "testuser", HashedPassword: entites.HashAndSalt([]byte("TestPass#2010")), Email: "test@test.com", MobileNumber: "01585285285"}

	if err != nil {
		log.Fatal("Error while get database context: \n", err)
		return nil, err
	}

	//create the bot account
	iter.InsertOne(context.Background(), user)
	return &user, nil
}

func deletetestUser(user *entites.User) error {
	iter := dbcontext.Collection("users")
	//create the bot account
	iter.DeleteOne(context.Background(), *user)
	return nil
}

func seedtestQuestion() (*entites.Question, error) {
	question := entites.Question{ID: 5525525, Header: "Header"}

	iter := dbcontext.Collection("Question")
	//create the bot account
	iter.InsertOne(context.Background(), question)
	return &question, nil
}

func deletetestQuestion(question *entites.Question) error {
	iter := dbcontext.Collection("Question")
	//create the bot account
	iter.DeleteOne(context.Background(), *question)
	return nil
}
