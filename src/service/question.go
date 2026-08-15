package service

import (
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
)

// QuestionServices busniess of how to create account
type QuestionServices struct {
	questionRepo repository.IQuestionRepository
}

// NewQuestionServices busniess of how to create account
func NewQuestionServices(_questionRepo repository.IQuestionRepository) *QuestionServices {
	return &QuestionServices{questionRepo: _questionRepo}
}

// GetQuestionByID call GetQuestionByID
func (svc QuestionServices) GetQuestionByID(_id int) (*resources.Question, error) {
	question, err := svc.questionRepo.GetQuestionByID(_id)
	if err != nil || question == nil {
		return nil, err
	}
	result := &resources.Question{ID: question.ID, Header: question.Header, Answers: make([]resources.Answer, 0, len(question.Answers))}
	for _, answer := range question.Answers {
		result.Answers = append(result.Answers, resources.Answer{ID: answer.ID, Text: answer.Text})
	}
	return result, nil
}
