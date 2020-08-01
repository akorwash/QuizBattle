package engine

import (
	"math/rand"
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

//GetAnswers ctor for User Account
func (question *Question) GetAnswers() *[]Answer {
	return &question.answers
}

//GetHeader ctor for User Account
func (question *Question) GetHeader() *string {
	return &question.header
}

//NewQuestion ctor for Answer
func NewQuestion(_id int, _header string) *Question {
	question := Question{id: _id, header: _header}
	return &question
}
