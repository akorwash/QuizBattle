package service

import (
	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
)

//QuestionServices busniess of how to create account
type QuestionServices struct {
	questionRepo repository.IQuestionRepository
}

//NewQuestionServices busniess of how to create account
func NewQuestionServices(_questionRepo repository.IQuestionRepository) *QuestionServices {
	return &QuestionServices{questionRepo: _questionRepo}
}

//GetQuestionByID call GetQuestionByID
func (svc QuestionServices) GetQuestionByID(_id int) (*entites.Question, error) {
	return svc.questionRepo.GetQuestionByID(_id)
}
