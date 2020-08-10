package repository

import (
	"context"

	"github.com/akorwash/QuizBattle/datastore"
	"github.com/akorwash/QuizBattle/datastore/entites"
	"go.mongodb.org/mongo-driver/bson"
)

//QuestionRepository to do
type QuestionRepository struct{}

//GetQuestionByID to do
func (repos *QuestionRepository) GetQuestionByID(_id int) (*entites.Question, error) {
	dbcontext, cancelContext, err := datastore.GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		defer cancelContext()
		return nil, err
	}

	filter := bson.M{"id": _id}
	iter := dbcontext.Collection("Question")
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
	defer cancelContext()
	if _question.Header == "" {
		return nil, nil
	}
	return &_question, nil
}
