package entites

//AnswerFileName to do
var AnswerFileName = "answers.json"

//Answer to do
type Answer struct {
	ID        int
	Text      string
	IsCorrect bool
}

//CardFileName to do
var CardFileName = "cards.json"

//Card to do
type Card struct {
	ID    int
	Power float32
	Owner string

	Likes int
	Hits  int

	Questions Question
}

//QuestionsFileName to do
var QuestionsFileName = "questions.json"

//Question to do
type Question struct {
	ID      int
	Header  string
	Answers []Answer
}

//UsersFileName to do
var UsersFileName = "users.json"

//User to do
type User struct {
	Username     string
	Password     string
	Email        string
	MobileNumber string
}
