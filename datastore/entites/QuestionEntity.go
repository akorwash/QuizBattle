package entites

//QuestionsFileName to do
var QuestionsFileName = "questions.json"

//QuestionEntity to do
type QuestionEntity struct {
	ID      int
	Header  string
	Answers []AnswerEntity
}
