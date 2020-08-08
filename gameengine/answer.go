package engine

//Answer to do
type Answer struct {
	id        int
	text      string
	isCorrect bool
}

//GetText ctor for User Account
func (answer *Answer) GetText() string {
	return answer.text
}

//GetIsCorrect ctor for User Account
func (answer *Answer) GetIsCorrect() bool {
	return answer.isCorrect
}

//GetID ctor for User Account
func (answer *Answer) GetID() int {
	return answer.id
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
