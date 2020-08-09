package engine

import (
	"math/rand"

	. "github.com/ahmetb/go-linq"
)

//Question to do
type Question struct {
	id      int
	header  string
	answers []Answer
}

//QuestionList to do
type QuestionList []Question

//QuestionSet to do
var QuestionSet QuestionList

//CalculateQuestions ctor for User Account
func (card *Card) CalculateQuestions() *Card {
	min := 0
	max := len(QuestionSet) - 1

	questionIndex := rand.Intn(max-min+1) + min
	card.questions = QuestionSet[questionIndex]
	return card
}

//GetQuestionByID ctor for User Account
func (card *Card) GetQuestionByID(_id int) *Question {
	var _questions []Question

	From(QuestionSet).Where(func(c interface{}) bool {
		return c.(Question).id == _id
	}).Select(func(c interface{}) interface{} {
		return c.(Question)
	}).ToSlice(&_questions)

	if len(_questions) == 1 {
		return &_questions[0]
	} else {
		return nil
	}
}

//GetAnswers ctor for User Account
func (question *Question) GetAnswers() *[]Answer {
	return &question.answers
}

//GetHeader ctor for User Account
func (question *Question) GetHeader() *string {
	return &question.header
}

//GetID ctor for User Account
func (question *Question) GetID() *int {
	return &question.id
}

//NewQuestion ctor for Answer
func NewQuestion(_id int, _header string) *Question {
	question := Question{id: _id, header: _header}
	return &question
}
