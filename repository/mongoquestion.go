package repository

import (
	"context"

	"github.com/akorwash/QuizBattle/datastore"
	"github.com/akorwash/QuizBattle/datastore/entites"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

//MongoQuestionRepository repo to query the question collection at mongo database
type MongoQuestionRepository struct {
	mongoContext *mongo.Database
}

//GetQuestionByID query the database and find question by id
func (repos *MongoQuestionRepository) GetQuestionByID(_id int) (*entites.Question, error) {

	filter := bson.M{"id": _id}
	iter := repos.mongoContext.Collection("Question")
	cursor, err := iter.Find(context.Background(), filter)
	if err != nil {
		println("Error while getting all todos, Reason: %v\n", err)
		return nil, err
	}

	var _question entites.Question
	for cursor.Next(context.Background()) {
		cursor.Decode(&_question)
		break
	}
	if _question.Header == "" {
		return nil, nil
	}
	return &_question, nil
}

//NewMongoQuestionRepository ctor for MongoQuestionRepository
func NewMongoQuestionRepository(dbConfig datastore.DBConfiguration) (*MongoQuestionRepository, error) {
	dbcontext, err := datastore.GetContext(dbConfig)
	if err != nil {
		println("Error while get database context: %v\n", err)
		return nil, err
	}

	repo := MongoQuestionRepository{}
	repo.mongoContext = dbcontext
	return &repo, nil
}
