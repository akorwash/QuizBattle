package engine

//Answer to do
type Answer struct {
	id        int
	text      string
	isCorrect bool
}

//NewAnswer ctor for Answer
func NewAnswer(_id int, _text string, _isCorrect bool) *Answer {
	answer := Answer{id: _id, text: _text, isCorrect: _isCorrect}
	return &answer
}

//AddAnswers ctor for User Account
func (question *Question) AddAnswers(answer *Answer) *Question {
	question.answers = append(question.answers, *answer)
	return question
}
