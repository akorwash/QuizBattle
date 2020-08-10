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

//ValidatePassword get name of Bot
func (userAccount *User) ValidatePassword(_pass string) bool {
	return (userAccount.Password == _pass)
}

//Bot is class represent Player User account with login data
type Bot struct {
	ID      int
	BotName string
	Level   int
}
