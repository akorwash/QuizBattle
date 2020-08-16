package repository

import (
	"github.com/akorwash/QuizBattle/datastore"
	"go.mongodb.org/mongo-driver/mongo"
)

//MongoGameRepository repo to query the question collection at mongo database
type MongoGameRepository struct {
	mongoContext *mongo.Database
}

//NewMongoGameRepository ctor for MongoQuestionRepository
func NewMongoGameRepository(dbConfig datastore.DBConfiguration) (*MongoGameRepository, error) {
	dbcontext, err := datastore.GetContext(dbConfig)
	if err != nil {
		println("Error while get database context: %v\n", err)
		return nil, err
	}

	repo := MongoGameRepository{}
	repo.mongoContext = dbcontext
	return &repo, nil
}
