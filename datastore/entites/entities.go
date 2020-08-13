package entites

//Answer answer entity, eacg question will have 4 answers one of them will be is correct
type Answer struct {
	ID        int
	Text      string
	IsCorrect bool
}

//Card card entity, tradable object between players also main object when the battle start.
type Card struct {
	ID    int
	Power float32
	Owner string

	Likes int
	Hits  int

	Questions Question
}

//Question question entity
type Question struct {
	ID      int
	Header  string
	Answers []Answer
}

//User user entity contains personal information
type User struct {
	ID           int64
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
