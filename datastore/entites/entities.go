package entites

import (
	"log"

	"golang.org/x/crypto/bcrypt"
)

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
	ID             int64
	Username       string
	Fullname       string
	YearOfBirth    int
	MonthOfBirth   int
	DayOfBirth     int
	HashedPassword string
	Email          string
	MobileNumber   string
}

//ValidatePassword get name of Bot
func (userAccount *User) ValidatePassword(_pass string) bool {
	return comparePasswords(userAccount.HashedPassword, []byte(_pass))
}

//compare password with hashed one
func comparePasswords(hashedPwd string, plainPwd []byte) bool {
	// Since we'll be getting the hashed password from the DB it
	// will be a string so we'll need to convert it to a byte slice
	byteHash := []byte(hashedPwd)
	err := bcrypt.CompareHashAndPassword(byteHash, plainPwd)
	if err != nil {
		log.Println(err)
		return false
	}

	return true
}

//HashAndSalt hash string and salt it
func HashAndSalt(pwd []byte) string {

	// Use GenerateFromPassword to hash & salt pwd.
	// MinCost is just an integer constant provided by the bcrypt
	// package along with DefaultCost & MaxCost.
	// The cost can be any value you want provided it isn't lower
	// than the MinCost (4)
	hash, err := bcrypt.GenerateFromPassword(pwd, bcrypt.MinCost)
	if err != nil {
		log.Println(err)
	}
	// GenerateFromPassword returns a byte slice so we need to
	// convert the bytes to a string and return it
	return string(hash)
}

//Bot is class represent Player User account with login data
type Bot struct {
	ID      int
	BotName string
	Level   int
}

//Game class represnt game history
type Game struct {
	ID          int
	IsPublic    bool
	UserID      int
	TimeLine    []string
	JoinedUsers []int
}
